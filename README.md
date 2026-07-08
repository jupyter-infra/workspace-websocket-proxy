# workspace-websocket-proxy

WebSocket-to-TCP proxy sidecar for [jupyter-k8s](https://github.com/jupyter-infra/jupyter-k8s). Enables remote IDE connections (VS Code, Kiro, Cursor) to workspace pods over WebSocket without cloud-specific dependencies.

## What it does

Accepts incoming WebSocket connections and bridges them to a TCP server (the remote access server on `localhost:2222`). Runs as a sidecar container in the workspace pod.

## License

MIT


TBD