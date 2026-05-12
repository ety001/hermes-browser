// Hermes Browser — Content Script
// Injected into web pages to perform DOM operations.

// ─── Message Listener ─────────────────────────────────────────────────────

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    const { id, method, params } = message;

    switch (method) {
        case 'click':
            sendResponse(clickElement(params));
            break;
        case 'type':
            sendResponse(typeText(params));
            break;
        case 'hover':
            sendResponse(hoverElement(params));
            break;
        case 'scroll':
            sendResponse(scrollPage(params));
            break;
        case 'select_option':
            sendResponse(selectOption(params));
            break;
        case 'get_content':
            sendResponse(extractContent(params));
            break;
        case 'execute_js':
            executeJavaScript(params).then(sendResponse);
            return true; // async
        case 'wait_for':
            waitForElement(params).then(sendResponse);
            return true; // async
        case 'get_element_rect':
            sendResponse(getElementRect(params));
            break;
        default:
            sendResponse({
                status: 'error',
                code: 'UNKNOWN_METHOD',
                error: `Unknown method: ${method}`,
            });
    }
});

// ─── Click ────────────────────────────────────────────────────────────────

function clickElement(params) {
    const { selector, timeout = 10000 } = params;
    const el = findElement(selector, timeout);
    if (!el) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Element not found: ${selector}`,
        };
    }

    el.click();
    return {
        status: 'success',
        data: {
            clicked: true,
            tag: el.tagName.toLowerCase(),
            text: (el.textContent || '').trim().substring(0, 100),
        },
    };
}

// ─── Type Text ────────────────────────────────────────────────────────────

function isTextInputElement(el) {
    if (!el || !el.tagName) return false;
    const tag = el.tagName.toLowerCase();
    if (tag === 'textarea') return true;
    if (tag === 'input') {
        const type = (el.getAttribute('type') || 'text').toLowerCase();
        const validTypes = ['text', 'email', 'password', 'search', 'tel', 'url', 'number'];
        return validTypes.includes(type);
    }
    // contenteditable elements (div, span, etc.)
    if (el.isContentEditable) return true;
    return false;
}

function typeText(params) {
    const { selector, text, clear_first = true, press_enter = false } = params;
    const el = findElement(selector);
    if (!el) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Input field not found: ${selector}`,
        };
    }

    if (!isTextInputElement(el)) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Element '${el.tagName.toLowerCase()}' is not a text input field. Use a <textarea>, <input>, or contenteditable element.`,
        };
    }

    el.focus();

    if (clear_first) {
        el.value = '';
        // Dispatch input event to trigger React/Vue change detection
        el.dispatchEvent(new Event('input', { bubbles: true }));
    }

    // Type character by character
    for (const char of text) {
        el.value += char;
        el.dispatchEvent(new Event('input', { bubbles: true }));
        el.dispatchEvent(new KeyboardEvent('keydown', { key: char, bubbles: true }));
        el.dispatchEvent(new KeyboardEvent('keypress', { key: char, bubbles: true }));
        el.dispatchEvent(new KeyboardEvent('keyup', { key: char, bubbles: true }));
    }

    if (press_enter) {
        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        el.dispatchEvent(new KeyboardEvent('keypress', { key: 'Enter', bubbles: true }));
        el.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', bubbles: true }));
        const form = el.closest('form');
        if (form) {
            form.dispatchEvent(new Event('submit', { bubbles: true }));
        }
    }

    return {
        status: 'success',
        data: { typed: true, length: text.length },
    };
}

// ─── Hover ────────────────────────────────────────────────────────────────

function hoverElement(params) {
    const { selector } = params;
    const el = findElement(selector);
    if (!el) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Element not found: ${selector}`,
        };
    }

    el.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    el.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }));

    return {
        status: 'success',
        data: {
            hovered: true,
            tag: el.tagName.toLowerCase(),
            text: (el.textContent || '').trim().substring(0, 100),
        },
    };
}

// ─── Scroll ───────────────────────────────────────────────────────────────

function scrollPage(params) {
    const { direction = 'down', amount = 'one_page' } = params;

    let scrollAmount;
    if (amount === 'one_page') {
        scrollAmount = window.innerHeight;
    } else if (amount === 'half_page') {
        scrollAmount = window.innerHeight / 2;
    } else {
        // Try to parse as a CSS value like "500px"
        const match = amount.match(/^(\d+)(?:px)?$/);
        scrollAmount = match ? parseInt(match[1]) : window.innerHeight;
    }

    const delta = direction === 'up' ? -scrollAmount : scrollAmount;
    window.scrollBy({ top: delta, behavior: 'smooth' });

    return {
        status: 'success',
        data: {
            scrolled: true,
            scroll_y: window.scrollY,
            scroll_height: document.documentElement.scrollHeight,
        },
    };
}

// ─── Select Option ────────────────────────────────────────────────────────

function selectOption(params) {
    const { selector, value } = params;
    const el = findElement(selector);
    if (!el) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Select element not found: ${selector}`,
        };
    }

    if (el.tagName !== 'SELECT') {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Element is not a select: ${selector}`,
        };
    }

    el.value = value;
    el.dispatchEvent(new Event('change', { bubbles: true }));

    return {
        status: 'success',
        data: { selected: true, value: value },
    };
}

// ─── Get Element Rect (for screenshot cropping) ──────────────────────────

function getElementRect(params) {
    const { selector } = params;
    const el = findElement(selector);
    if (!el) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Element not found: ${selector}`,
        };
    }

    const rect = el.getBoundingClientRect();
    return {
        status: 'success',
        data: {
            x: window.scrollX + rect.x,
            y: window.scrollY + rect.y,
            width: rect.width,
            height: rect.height,
        },
    };
}

// ─── Extract Content ──────────────────────────────────────────────────────

function extractContent(params) {
    const { selector, type = 'text' } = params;
    const root = selector ? document.querySelector(selector) : document.body;

    if (!root) {
        return {
            status: 'error',
            code: 'ELEMENT_NOT_FOUND',
            error: `Element not found: ${selector}`,
        };
    }

    let data;
    switch (type) {
        case 'text':
            data = cleanText(root.innerText);
            break;
        case 'html':
            data = root.innerHTML;
            break;
        case 'markdown':
            data = htmlToMarkdown(root);
            break;
        default:
            data = cleanText(root.innerText);
    }

    return { status: 'success', data };
}

// ─── Execute JavaScript ───────────────────────────────────────────────────

async function executeJavaScript(params) {
    const { expression, return_value = true } = params;

    try {
        // Wrap in async IIFE to support top-level await
        const wrappedCode = return_value
            ? `(async () => { return (async () => { ${expression} })(); })()`
            : `(async () => { ${expression} })()`;

        const result = await eval(wrappedCode);
        return {
            status: 'success',
            data: {
                result: result !== undefined ? String(result) : 'undefined',
                type: typeof result,
            },
        };
    } catch (err) {
        return {
            status: 'error',
            code: 'JS_EXECUTION_ERROR',
            error: err.message,
        };
    }
}

// ─── Wait For Element ──────────────────────────────────────────────────────

function waitForElement(params) {
    const { selector, state = 'visible', timeout = 30000 } = params;

    return new Promise((resolve) => {
        const start = Date.now();

        const check = () => {
            const el = document.querySelector(selector);

            const found = checkElementState(el, state);
            if (found) {
                resolve({
                    status: 'success',
                    data: { found: true, elapsed: Date.now() - start },
                });
                return;
            }

            if (Date.now() - start >= timeout) {
                resolve({
                    status: 'error',
                    code: 'TIMEOUT',
                    error: `Timeout waiting for '${selector}' to be ${state}`,
                });
                return;
            }

            requestAnimationFrame(check);
        };

        check();
    });
}

function checkElementState(el, state) {
    switch (state) {
        case 'visible':
            return el !== null && el.offsetParent !== null;
        case 'hidden':
            return el === null || el.offsetParent === null;
        case 'attached':
            return el !== null;
        case 'detached':
            return el === null;
        default:
            return el !== null && el.offsetParent !== null;
    }
}

// ─── Utility Functions ────────────────────────────────────────────────────

function findElement(selector, timeout = 0) {
    if (timeout > 0) {
        // Simple synchronous check; the wait_for command should be used
        // for waiting. Here we just find what's available.
    }
    return document.querySelector(selector);
}

function cleanText(text) {
    return text
        .replace(/[ \t]+/g, ' ')       // Collapse inline whitespace
        .replace(/ *\n */g, '\n')       // Clean line-leading/trailing spaces
        .replace(/\n{3,}/g, '\n\n')     // Max two consecutive newlines
        .trim();
}

// ─── HTML to Markdown (lightweight) ───────────────────────────────────────

function htmlToMarkdown(root) {
    let md = '';
    traverseNodes(root, 0);
    return md.trim();

    function traverseNodes(node, depth) {
        if (node.nodeType === Node.TEXT_NODE) {
            const text = node.textContent.trim();
            if (text) md += text + ' ';
            return;
        }

        if (node.nodeType !== Node.ELEMENT_NODE) return;

        const tag = node.tagName.toLowerCase();
        const children = node.childNodes;

        switch (tag) {
            case 'h1': case 'h2': case 'h3':
            case 'h4': case 'h5': case 'h6': {
                const level = parseInt(tag[1]);
                md += '\n' + '#'.repeat(level) + ' ';
                traverseAll(children, depth + 1);
                md += '\n\n';
                break;
            }
            case 'p': {
                traverseAll(children, depth + 1);
                md += '\n\n';
                break;
            }
            case 'br': {
                md += '\n';
                break;
            }
            case 'hr': {
                md += '\n---\n\n';
                break;
            }
            case 'a': {
                const href = node.getAttribute('href') || '';
                const text = node.textContent.trim();
                if (text && href) {
                    md += `[${text}](${href}) `;
                } else if (text) {
                    md += text + ' ';
                }
                break;
            }
            case 'img': {
                const alt = node.getAttribute('alt') || '';
                const src = node.getAttribute('src') || '';
                if (src) md += `![${alt}](${src}) `;
                break;
            }
            case 'strong': case 'b': {
                md += '**' + node.textContent.trim() + '** ';
                break;
            }
            case 'em': case 'i': {
                md += '*' + node.textContent.trim() + '* ';
                break;
            }
            case 'code': {
                md += '`' + node.textContent.trim() + '` ';
                break;
            }
            case 'pre': {
                const code = node.textContent.trim();
                md += '\n```\n' + code + '\n```\n\n';
                break;
            }
            case 'ul': {
                for (const child of children) {
                    if (child.tagName === 'LI') {
                        md += '\n- ';
                        traverseAll(child.childNodes, depth + 1);
                        md += '\n';
                    }
                }
                md += '\n';
                break;
            }
            case 'ol': {
                let index = 1;
                for (const child of children) {
                    if (child.tagName === 'LI') {
                        md += `\n${index}. `;
                        traverseAll(child.childNodes, depth + 1);
                        md += '\n';
                        index++;
                    }
                }
                md += '\n';
                break;
            }
            case 'blockquote': {
                md += '\n> ';
                traverseAll(children, depth + 1);
                md += '\n\n';
                break;
            }
            case 'table': {
                md += '\n';
                const rows = node.querySelectorAll('tr');
                let headerDone = false;
                for (const row of rows) {
                    const cells = row.querySelectorAll('th, td');
                    md += '| ';
                    for (const cell of cells) {
                        md += cell.textContent.trim() + ' | ';
                    }
                    md += '\n';
                    if (!headerDone && row.querySelector('th')) {
                        md += '| ';
                        for (const cell of cells) {
                            md += '--- | ';
                        }
                        md += '\n';
                        headerDone = true;
                    }
                }
                md += '\n';
                break;
            }
            case 'div': case 'span': case 'section':
            case 'article': case 'main': case 'header':
            case 'footer': case 'nav': case 'aside': {
                traverseAll(children, depth + 1);
                md += '\n';
                break;
            }
            default:
                traverseAll(children, depth + 1);
        }
    }

    function traverseAll(children, depth) {
        for (const child of children) {
            traverseNodes(child, depth);
        }
    }
}
