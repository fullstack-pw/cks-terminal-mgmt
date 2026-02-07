# cks-terminal-mgmt

Terminal management microservice for the CKS (Certified Kubernetes Security Specialist) practice platform. Provides browser-based terminal access to KubeVirt VMs using [ttyd](https://github.com/tsl0922/ttyd).

## Architecture

```
Browser (iframe) → cks-terminal-mgmt → SSH → KubeVirt VM
                        │
                   Go service + ttyd
                   (sandboxy cluster)
```

The service runs on the sandboxy cluster alongside KubeVirt VMs, providing direct SSH access without requiring virtctl. It spawns ttyd processes on-demand for each terminal connection, which handle xterm.js rendering, WebSocket communication, and terminal resize natively.

## API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check with active session count |
| `/terminal?vmIP=10.244.x.x` | GET | Terminal connection (WebSocket upgrade) |
| `/metrics` | GET | Prometheus metrics |

## Development

### Prerequisites

- Go 1.24+
- ttyd binary (installed automatically in Docker image)
- SSH access to target VMs (ed25519 key)

### Run Locally

```bash
make run
```

### Build Docker Image

```bash
make docker-build
```

### Run in Docker

```bash
make docker-run
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `SSH_KEY_PATH` | `/home/appuser/.ssh/id_ed25519` | Path to SSH private key |
| `SSH_USER` | `suporte` | SSH username for VM connections |
| `LOG_LEVEL` | `INFO` | Log level |

## Deployment

Deployed to the **sandboxy** cluster via ArgoCD, using kustomize overlays.

### Kustomize Structure

```
kustomize/
├── base/              # ArgoCD-managed (Istio VirtualService)
├── ephemeral-base/    # PR environments (Traefik Ingress)
└── overlays/
    ├── sandboxy/      # Production overlay
    └── ephemeral/     # PR environment overlay
```

### CI/CD

- **Push to main**: Build image, push to Harbor, update sandboxy kustomization tag
- **PR opened**: Create ephemeral K3s cluster, deploy PR build, post URL as comment
- **PR closed**: Destroy ephemeral cluster and release IP pool slot
