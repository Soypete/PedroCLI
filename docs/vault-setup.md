# OpenBao + Kubernetes Secret Injection for PedroCLI

This guide covers deploying PedroCLI secrets from 1Password → OpenBao → Kubernetes pods.

OpenBao is the open-source fork of HashiCorp Vault; it is API-compatible so the `vault` CLI works against it.

## Architecture

```
1Password (source of truth)
    ↓  (op CLI reads secrets)
OpenBao at http://100.81.89.62:8200 (secret/apps/pedrocli)
    ↓  (vault CLI at deploy time, via ops/scripts/deploy.sh)
Helm --set-string (env vars injected into pod spec)
    ↓
PedroCLI reads DATABASE_URL, NOTION_TOKEN, CAL_API_KEY
```

## Prerequisites

- [1Password CLI (`op`)](https://developer.1password.com/docs/cli/)
- OpenBao running at `http://100.81.89.62:8200` (managed by foundry)
- `vault` CLI (API-compatible with OpenBao)
- `kubectl` configured for your cluster

## Step 1: Sync Secrets from 1Password to OpenBao

```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update

helm install vault hashicorp/vault \
  --set "injector.enabled=true" \
  --set "server.enabled=false" \
  --set "injector.externalVaultAddr=https://your-vault-server:8200"
```

If running Vault inside K8s:

```bash
helm install vault hashicorp/vault \
  --set "injector.enabled=true" \
  --set "server.dev.enabled=true"  # dev mode for testing only
```

## Step 2: Sync Secrets from 1Password to Vault

Store your secrets in 1Password under the `pedro` vault:

| 1Password Item | Field | Vault Path |
|---|---|---|
| `supabase_database_url` | `credential` | `secret/pedrocli/database` → `url` |
| `notion_api_key` | `credential` | `secret/pedrocli/notion` → `token` |
| `cal.com` | `credential` | `secret/pedrocli/calcom` → `api_key` |

Sync with one command:

```bash
make vault-sync
```

Or manually:

```bash
# Sign in to 1Password
eval $(op signin)

# Sync each secret
op read "op://pedro/supabase_database_url/credential" | \
  vault kv put secret/pedrocli/database url=-

op read "op://pedro/notion_api_key/credential" | \
  vault kv put secret/pedrocli/notion token=-

op read "op://pedro/cal.com/credential" | \
  vault kv put secret/pedrocli/calcom api_key=-
```

## Step 3: Configure Vault Policy and K8s Auth

```bash
make vault-setup
```

Or manually:

### Create Vault policy

```bash
vault policy write pedrocli - <<EOF
path "secret/data/pedrocli/*" {
  capabilities = ["read"]
}
EOF
```

### Enable K8s auth method

```bash
vault auth enable kubernetes

# Configure K8s auth (adjust for your cluster)
vault write auth/kubernetes/config \
  kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443"
```

### Create role binding

```bash
vault write auth/kubernetes/role/pedrocli \
  bound_service_account_names=pedrocli \
  bound_service_account_namespaces=default \
  policies=pedrocli \
  ttl=1h
```

## Step 4: Deploy to Kubernetes

```bash
kubectl apply -f deployments/kubernetes/repo-storage-pvc.yaml
```

The deployment includes:
- Vault Agent Injector annotations that inject secrets as files under `/vault/secrets/`
- A startup command that sources these files into environment variables
- A `pedrocli` ServiceAccount bound to the Vault role

## How It Works in the Pod

1. Vault Agent Injector runs as an init container and sidecar
2. It authenticates to Vault using the pod's ServiceAccount token
3. It writes secret files to `/vault/secrets/`:
   - `/vault/secrets/database` → `export DATABASE_URL="postgres://..."`
   - `/vault/secrets/notion` → `export NOTION_TOKEN="secret_..."`
   - `/vault/secrets/calcom` → `export CAL_API_KEY="cal_live_..."`
4. The container entrypoint sources these files before starting the server
5. PedroCLI reads `DATABASE_URL` to connect to Supabase

## Supabase Connection Details

Get your connection string from: **Supabase Dashboard → Settings → Database → Connection string (URI)**

For pooled connections (recommended for serverless/K8s):
```
postgresql://postgres.<project-ref>:<password>@aws-0-<region>.pooler.supabase.com:6543/postgres?sslmode=require
```

For direct connections (for migrations):
```
postgresql://postgres:<password>@db.<project-ref>.supabase.co:5432/postgres?sslmode=require
```

## Secret Rotation

To rotate secrets:

1. Update the secret in 1Password
2. Re-run `make vault-sync`
3. Vault Agent will automatically pick up changes (default TTL: 1h)
4. No pod restart needed — the sidecar refreshes secrets

## Local Development

For local development, use `op run` to inject secrets directly:

```bash
op run --env-file=.env -- ./pedrocli-http-server
```

Or export DATABASE_URL manually:

```bash
export DATABASE_URL="postgresql://postgres.<ref>:<pass>@aws-0-us-west-1.pooler.supabase.com:6543/postgres?sslmode=require"
./pedrocli-http-server
```
