# Deployment Guide

See [README.md](./README.md) for full setup instructions.

Key sections:
- [Deployment Topologies](./README.md#deployment-topologies) — same-machine vs cross-machine
- [Quick Start](./README.md#quick-start) — build or download, configure, start
- [systemd service](./README.md#3-run-as-a-systemd-user-service-recommended) — auto-start on boot
- [File placement on browser machine](./README.md#file-placement-on-browser-machine)
- [MCP Tools (15)](./README.md#mcp-tools-15) — parameter reference
- [Troubleshooting](./README.md#troubleshooting)

## Quick Reference

```bash
# 1. Build or download the binary
git clone https://github.com/ety001/hermes-browser.git
cd hermes-browser && make build

# 2. Place files on the browser machine
#    Binary → ~/Apps/hermes-browser/hermes-browser
#    Config → ~/Apps/hermes-browser/configs/deploy.yaml
#    Extension → load unpacked from extension/ in chrome://extensions/

# 3. Set up systemd auto-start
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/hermes-browser.service << 'EOF'
[Unit]
Description=Hermes Browser MCP Server
After=network-online.target

[Service]
Type=simple
ExecStart=%h/Apps/hermes-browser/hermes-browser -c %h/Apps/hermes-browser/configs/deploy.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

loginctl enable-linger $USER
systemctl --user daemon-reload
systemctl --user enable --now hermes-browser.service

# 4. On Hermes machine, add MCP server:
hermes mcp add hermes-browser \
  --url "http://<BROWSER_MACHINE_IP>:19875/mcp" \
  --auth header
```
