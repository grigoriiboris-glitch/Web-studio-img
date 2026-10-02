# Local pi.dev repair bridge

Localhost-only bridge for Web Studio ComfyUI runtime incidents.

Run:
`WEB_STUDIO_PROJECT_ROOT=/absolute/path/to/Web-studio-img node tools/pi-agent-bridge/server.mjs`

Optional environment variables:
- `PI_AGENT_BRIDGE_PORT` (default 8787)
- `PI_BIN` (default pi)
- `PI_AGENT_TIMEOUT_MS` (default 180000)

Endpoints:
- `GET /health`
- `POST /repair`

Repair requires `confirmed: true`. The bridge starts Pi in RPC mode, loads the
policy extension, waits for `agent_settled`, then checks local ComfyUI
`/system_stats`. Web Studio performs the authoritative `test_comfy_flow`
retest afterwards.

Pi's RPC mode is used as the long-lived integration boundary documented by
Pi. The bridge does not expose a generic shell endpoint.
