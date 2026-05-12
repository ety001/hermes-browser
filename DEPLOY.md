# Deployment Guide

See [README.md](./README.md) for full setup instructions.

Key sections:
- [Deployment Topologies](./README.md#deployment-topologies) — same‑machine vs cross‑machine
- [Quick Start](./README.md#quick-start) — build or download, configure, start
- [File placement on browser machine](./README.md#file-placement-on-browser-machine)
- [MCP Tools (15)](./README.md#mcp-tools-15) — parameter reference
- [Troubleshooting](./README.md#troubleshooting)

## Quick Reference

```bash
# Download the right binary for the browser machine's platform
#   uname -s → Linux / Darwin
#   uname -m → x86_64 / aarch64 / arm64

# Make executable and start
chmod +x hermes-browser-<os>-<arch>
./hermes-browser-<os>-<arch> -c ~/.hermes-browser/config.yaml

# On Hermes machine:
hermes mcp add hermes-browser \
  --url "http://<BROWSER_MACHINE_IP>:19875/mcp" \
  --auth header
```
