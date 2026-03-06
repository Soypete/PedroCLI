# Tailscale Kubernetes Operator Setup

This document covers installing the Tailscale Kubernetes Operator, configuring OAuth credentials, and exposing the whisper web UI over your tailnet.

## Prerequisites

- A Tailscale account (free tier works)
- `helm` CLI installed
- `kubectl` configured for your cluster
- Admin access to the [Tailscale admin console](https://login.tailscale.com/admin)

## 1. Create an OAuth Client

1. Go to **Settings** > **OAuth clients** in the [Tailscale admin console](https://login.tailscale.com/admin/settings/oauth).
2. Click **Generate OAuth client**.
3. Grant the following scopes:
   - `devices` — **Write** (required for the operator to register devices)
4. Optionally restrict to specific tags (e.g., `tag:k8s`) for least-privilege access.
5. Copy the **Client ID** and **Client Secret** — you'll need both for the Helm install.

## 2. Create ACL Tags (Recommended)

In the Tailscale ACL editor (**Access Controls** > **Edit ACL file**), add a tag for Kubernetes-managed devices:

```json
{
  "tagOwners": {
    "tag:k8s": ["autogroup:admin"]
  }
}
```

This allows the operator to tag exposed services with `tag:k8s`, which you can then use in ACL rules to restrict access.

## 3. Install the Tailscale Operator

```bash
# Add the Tailscale Helm repo
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm repo update

# Create the namespace
kubectl create namespace tailscale

# Create the OAuth secret
kubectl create secret generic tailscale-operator \
  --namespace tailscale \
  --from-literal=client_id=<YOUR_CLIENT_ID> \
  --from-literal=client_secret=<YOUR_CLIENT_SECRET>

# Install the operator
helm install tailscale-operator tailscale/tailscale-operator \
  --namespace tailscale \
  --set oauth.clientId="" \
  --set oauth.clientSecret="" \
  --set operatorConfig.defaultTags="tag:k8s"
```

> **Note:** We create the secret manually and leave `oauth.clientId`/`oauth.clientSecret` empty in the Helm values so credentials never appear in Helm release metadata. The operator reads from the `tailscale-operator` secret in its namespace.

Verify the operator is running:

```bash
kubectl get pods -n tailscale
# Expected: tailscale-operator-xxx   Running
```

## 4. How Service Exposure Works

When a Kubernetes Service has the annotation `tailscale.com/expose: "true"`, the Tailscale operator:

1. Creates a Tailscale proxy pod alongside the service.
2. Registers the proxy as a device on your tailnet.
3. Assigns the hostname from `tailscale.com/hostname` (e.g., `whisper`).
4. Issues a valid HTTPS certificate automatically via Tailscale's built-in ACME integration.

The web UI Helm chart's service template includes these annotations:

```yaml
annotations:
  tailscale.com/expose: "true"
  tailscale.com/hostname: "whisper"
```

## 5. MagicDNS Resolution

With MagicDNS enabled (on by default), the service becomes accessible at:

```
https://whisper.<tailnet-name>.ts.net
```

For example, if your tailnet is `example`, the URL is `https://whisper.example.ts.net`.

Any device on your tailnet can resolve this hostname. No external DNS or cert-manager is needed.

## 6. HTTPS / TLS Certificates

Tailscale automatically provisions and renews TLS certificates for exposed services:

- Certificates are issued by Let's Encrypt via Tailscale's ACME integration.
- No cert-manager, Ingress controller, or manual certificate management required.
- The certificate is valid for `whisper.<tailnet>.ts.net`.
- Renewal is handled transparently by the Tailscale proxy pod.

## 7. Access Control (ACLs)

Restrict who can reach the web UI by adding ACL rules. Example — only allow users with the `developer` group:

```json
{
  "acls": [
    {
      "action": "accept",
      "src": ["group:developers"],
      "dst": ["tag:k8s:*"]
    }
  ],
  "groups": {
    "group:developers": ["user@example.com", "user2@example.com"]
  }
}
```

This ensures only members of `group:developers` can reach any service tagged with `tag:k8s`, including the whisper web UI.

To lock it down further to only the whisper service:

```json
{
  "acls": [
    {
      "action": "accept",
      "src": ["group:developers"],
      "dst": ["whisper:80"]
    }
  ]
}
```

## 8. Troubleshooting

| Symptom | Check |
|---------|-------|
| Operator pod not starting | `kubectl logs -n tailscale deploy/tailscale-operator` |
| Service not appearing on tailnet | Verify `tailscale.com/expose: "true"` annotation on the Service |
| DNS not resolving | Ensure MagicDNS is enabled in Tailscale admin > DNS settings |
| TLS certificate errors | Wait 1-2 minutes after first exposure; cert issuance is async |
| 403 from ACLs | Check your ACL rules allow the source device/user to reach `tag:k8s` |

## 9. Uninstalling

```bash
# Remove the web UI (cleans up Tailscale proxy automatically)
helm uninstall whisper-web-ui

# Remove the operator
helm uninstall tailscale-operator -n tailscale
kubectl delete namespace tailscale
```

The operator cleans up tailnet device registrations when services or the operator itself are removed.
