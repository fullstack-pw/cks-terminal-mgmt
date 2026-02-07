# CLAUDE.md

## Rules

* YOU ARE FORBIDDEN TO COMMIT AND PUSH
* You are forbidden to add 'Claude' reference as author to anywhere (commits, docs, etc)
* You don't put commentaries on code
* You don't use emoticons
* You're not allowed to commit and push directly neither mention yourself
* You're not allowed to create/edit resources directly via kubectl/vault, you can only make these type of changes via OpenTofu or manually like this to test something
* At the end of your task you always review what was done and repo README to properly update it
* **IMPORTANT**: All secrets from `secrets/common/cluster-secret-store/secrets` are automatically available as environment variables on self-hosted runners (including VAULT_TOKEN, VAULT_ADDR, etc.). DO NOT explicitly set these in workflows unless absolutely necessary

## Repository Overview

Public portfolio repository showcasing production-grade infrastructure-as-code for managing Kubernetes clusters on Proxmox VE using OpenTofu with GitOps workflows.

**Key Architecture:**
- Two-tier OpenTofu structure: base modules + application modules
- Cluster provisioning via Cluster API (tools cluster as management cluster)
- Multi-environment isolation using OpenTofu workspaces (dev, stg, prod, tools, home, observability, sandboxy)
- Secrets: SOPS (age encryption) for git storage, HashiCorp Vault for runtime
- Distributions: K3s (legacy single-node), Talos Linux, vanilla Kubernetes (both via Cluster API)

**Security Posture:**
- ✅ All secrets SOPS-encrypted (age key: `age15vvdhaj90s3nru2zw4p2a9yvdrv6alfg0d6ea6zxpx3eagyqfqlsgdytsp`)
- ✅ No credentials in git history (verified clean)
- ✅ Externally-usable credentials (SSH keys, GitHub PAT, Cloudflare token) protected
- ⚠️ Public repository - workflow logs visible (internal details only, homelab is private)

## Common Commands

### OpenTofu Operations
```bash
# Plan/apply specific environment
make plan ENV=dev
make apply ENV=dev

# All environments
make plan
make apply

# Utilities
make init
make fmt
make validate
```

### Secrets Management
```bash
# Create/edit/view secrets (SOPS-encrypted)
./secret_new.sh path/to/secret.yaml
./secret_edit.sh path/to/secret.yaml
./secret_view.sh path/to/secret.yaml
```

**Note:** Make commands automatically decode SOPS secrets to `clusters/tmp/` before OpenTofu runs.

### Cluster Management (Cluster API)
```bash
# Update Talos kubeconfigs in Vault
make build-kubeconfig-tool
make update-kubeconfigs ENV=sandboxy
```

## Key Patterns

### Workspace-Based Environments

Each workspace represents an environment. Configuration in `clusters/variables.tf`:

```hcl
variable "workload" {
  default = {
    dev = ["externaldns", "cert_manager", "istio", "argocd", ...]
    tools = ["postgres", "redis", "vault", "github_runner", ...]
  }
}

variable "config" {
  default = {
    dev = {
      kubernetes_context = "dev"
      argocd_domain = "dev.argocd.fullstack.pw"
    }
  }
}
```

**Pattern:** Modules check `contains(local.workload, "module_name")` for conditional deployment.

### Cluster API Provisioning (Current Standard)

1. Define cluster in `clusters/variables.tf` under tools workspace
2. OpenTofu creates Cluster API resources on tools cluster
3. Cluster API provisions VMs and bootstraps Kubernetes
4. `cicd-update-kubeconfig` tool extracts kubeconfigs to Vault
5. Cluster available immediately to OpenTofu and CI/CD

**Key Files:**
- `.github/workflows/opentofu.yml` - Main deployment pipeline
- `modules/apps/proxmox-talos-cluster/` - Talos cluster module
- `modules/apps/proxmox-kubeadm-cluster/` - Kubeadm cluster module

### Secrets Lifecycle

```
Developer → secret*.sh → SOPS YAML → Git → CI/CD (decrypt) → Vault → External Secrets → K8s → Pods
```

**In OpenTofu:**
```hcl
locals {
  secrets_json = jsondecode(file("${path.module}/tmp/secrets.json"))
}
vault_token = local.secrets_json["kv/cluster-secret-store/secrets/VAULT_TOKEN"]["VAULT_TOKEN"]
```

## Critical Workflows

### Adding a New Module

1. Create in `modules/apps/module-name/`
2. Add to `clusters/modules.tf`:
   ```hcl
   module "module_name" {
     count  = contains(local.workload, "module_name") ? 1 : 0
     source = "../modules/apps/module-name"
   }
   ```
3. Add to workspace in `clusters/variables.tf`:
   ```hcl
   workload = { dev = ["existing", "module_name"] }
   config = { dev = { module_name = {} } }
   ```

### Adding a New Cluster (Cluster API)

Add to `clusters/variables.tf` under tools workspace:
```hcl
config = {
  tools = {
    proxmox-talos-cluster = [{
      name = "dev"
      kubernetes_version = "v1.33.0"
      control_plane_endpoint_ip = "192.168.1.50"
      ip_range_start = "192.168.1.51"
      ip_range_end = "192.168.1.56"
      gateway = "192.168.1.1"
      prefix = 24
      dns_servers = ["192.168.1.3", "8.8.4.4"]
      source_node = "node03"
      template_id = 9005
      cp_replicas = 1
      wk_replicas = 2
      # ... resource allocations
    }]
  }
}
```

Commit changes - CI/CD handles provisioning.

### Handling Secrets

**Never commit unencrypted secrets.**

```bash
# Create new secret
./secret_new.sh secrets/path/to/secret.yaml

# Reference in OpenTofu
cd clusters && python3 load_secrets.py
# Access via local.secrets_json
```

## Important File Locations

**OpenTofu:**
- `clusters/modules.tf` - Module orchestration
- `clusters/variables.tf` - Workspace configs + Cluster API definitions
- `clusters/providers.tf` - Provider configurations
- `clusters/load_secrets.py` - SOPS decryption

**Modules:**
- `modules/base/` - Building blocks (helm, namespace, ingress, credentials, etc.)
- `modules/apps/` - Applications (argocd, vault, postgres, istio, cert_manager, etc.)

**CI/CD:**
- `.github/workflows/opentofu.yml` - Main deployment
- `.github/workflows/ansible.yml` - Legacy K3s maintenance

**Secrets:**
- `secrets/` - SOPS-encrypted secrets (all environments)
- `.sops.yaml` - Encryption rules

## State Management

- Backend: S3-compatible (MinIO) at `s3.fullstack.pw`
- Workspace-specific state files
- Daily backups to Oracle Cloud Object Storage

```bash
cd clusters && tofu workspace select dev
make workspace  # List workspaces
```

## Physical Infrastructure

- 3 Proxmox nodes: NODE01 (16GB), NODE02 (32GB), NODE03 (128GB)
- Management cluster: `tools` (K3s on NODE02)
- Legacy K3s: dev, stg, prod, tools, home, observability
- Cluster API: sandboxy workspace (Talos/kubeadm multi-node)
- Network: 192.168.1.0/24 (private, not internet-exposed)

## Security Notes

**Public Repository Context:**
- Repository is PUBLIC for portfolio showcase
- Homelab is PRIVATE (192.168.x.x not internet-routable)
- Externally-usable credentials (SSH keys, GitHub PAT, Cloudflare token) are protected
- Workflow logs are public but contain only internal details

**Best Practices Demonstrated:**
- SOPS encryption for all secrets
- GitHub Secrets for CI/CD credentials
- Self-hosted runners (controlled environment)
- No debug logging of sensitive values
- Clean git history (no committed secrets)

**Security Audit Results (Latest):**
- ✅ No externally-usable credential exposure found
- ✅ Proper secrets management (SOPS + Vault)
- ✅ Docker login using `--password-stdin`
- ⚠️ Internal details visible in logs (acceptable for portfolio)

## Troubleshooting

**OpenTofu plan shows unwanted changes:**
- Run: `cd clusters && python3 load_secrets.py`
- Verify workspace: `tofu workspace show`

**Module not deploying:**
- Check `workload` list in `clusters/variables.tf`
- Verify workspace: `tofu workspace select <env>`

**Kubeconfig not in Vault (Cluster API):**
- Verify cluster ready: `kubectl --context tools get cluster -n <namespace> <cluster>`
- Check secret: `kubectl --context tools get secret -n <namespace> <cluster>-kubeconfig`
- Run manually: `./cicd-update-kubeconfig --cluster-name <cluster> --namespace <namespace> --vault-path kv/cluster-secret-store/secrets --vault-addr $VAULT_ADDR --management-context tools`

**CRD chicken-and-egg:**
- Set `create_default_gateway = false` or `install_crd = false` initially
- Run `tofu apply` to install CRDs
- Set to `true` and apply again

## Testing Workflow

Always test in order:
1. `make fmt` - Format
2. `make validate` - Validate
3. `make plan ENV=<test>` - Review changes
4. `make apply ENV=<test>` - Apply to test
5. `kubectl --context <test> get all -A` - Verify
6. Apply to production

## References

- [README.md](README.md) - Comprehensive architecture
- [Security Audit](.claude/plans/lovely-foraging-otter.md) - Latest security assessment
- [docs/SECRETS_ROTATION.md](docs/SECRETS_ROTATION.md) - Secret rotation procedures
- [cicd-update-kubeconfig/README.md](cicd-update-kubeconfig/README.md) - Kubeconfig tool docs
