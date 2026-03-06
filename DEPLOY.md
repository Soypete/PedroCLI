# Deployment Guide: Whisper.cpp K8s + Web UI + Tailscale

Step-by-step instructions to deploy the full whisper transcription pipeline on a homelab Kubernetes cluster.

## Prerequisites

- Kubernetes cluster with `kubectl` configured
- [Longhorn](https://longhorn.io) storage driver installed (provides `longhorn` StorageClass)
- `helm` CLI (v3+)
- `docker` (or compatible builder like `podman`)
- A container registry (Docker Hub, GHCR, Harbor, etc.)
- A Tailscale account with admin access

## Step 1: Build and Push Docker Images

### 1a. Whisper Server

```bash
cd whisper-server/

# Build the image
docker build -t your-registry/whisper-server:latest .

# Push to registry
docker push your-registry/whisper-server:latest
```

> **Note:** The build clones whisper.cpp and compiles from source. This takes 5-10 minutes on first build. The default `ggml-base.en.bin` model is baked into the image.

### 1b. Web UI

```bash
cd web-ui/

# Build the image
docker build -t your-registry/whisper-web-ui:latest .

# Push to registry
docker push your-registry/whisper-web-ui:latest
```

## Step 2: Apply Kubernetes Manifests (Whisper Server)

```bash
# Create namespace
kubectl apply -f k8s/whisper/namespace.yaml

# Create storage (PVCs)
kubectl apply -f k8s/whisper/pvc-models.yaml
kubectl apply -f k8s/whisper/pvc-transcripts.yaml

# Create config
kubectl apply -f k8s/whisper/configmap.yaml

# Deploy whisper-server
kubectl apply -f k8s/whisper/deployment.yaml
kubectl apply -f k8s/whisper/service.yaml
```

Or apply everything at once:

```bash
kubectl apply -f k8s/whisper/
```

### Update the image reference

Before applying, edit `k8s/whisper/deployment.yaml` to set your actual registry:

```yaml
image: your-registry/whisper-server:latest
```

### Verify whisper-server is running

```bash
kubectl get pods -n ai-services
# Wait for STATUS: Running

kubectl logs -n ai-services deploy/whisper-server
# Should show whisper-server starting on port 8080
```

## Step 3: (Optional) Seed a Larger Model

If you want to use a larger model (e.g., `ggml-large-v3.bin`), copy it into the models PVC:

```bash
# Create a temporary pod to copy the model
kubectl run model-seeder -n ai-services \
  --image=busybox \
  --restart=Never \
  --overrides='{
    "spec": {
      "containers": [{
        "name": "seeder",
        "image": "busybox",
        "command": ["sleep", "3600"],
        "volumeMounts": [{
          "name": "models",
          "mountPath": "/models"
        }]
      }],
      "volumes": [{
        "name": "models",
        "persistentVolumeClaim": {
          "claimName": "whisper-models"
        }
      }]
    }
  }'

# Copy model into the PVC
kubectl cp ggml-large-v3.bin ai-services/model-seeder:/models/ggml-large-v3.bin

# Clean up
kubectl delete pod model-seeder -n ai-services

# Update the configmap to point to the new model
kubectl edit configmap whisper-config -n ai-services
# Change WHISPER_MODEL_PATH to /models/ggml-large-v3.bin

# Restart the deployment to pick up the new model
kubectl rollout restart deploy/whisper-server -n ai-services
```

## Step 4: Install the Tailscale Operator

See [docs/tailscale-setup.md](docs/tailscale-setup.md) for detailed instructions.

Quick version:

```bash
# Add Helm repo
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm repo update

# Create namespace and secret
kubectl create namespace tailscale
kubectl create secret generic tailscale-operator \
  --namespace tailscale \
  --from-literal=client_id=<YOUR_TAILSCALE_CLIENT_ID> \
  --from-literal=client_secret=<YOUR_TAILSCALE_CLIENT_SECRET>

# Install operator
helm install tailscale-operator tailscale/tailscale-operator \
  --namespace tailscale \
  --set operatorConfig.defaultTags="tag:k8s"

# Verify
kubectl get pods -n tailscale
```

## Step 5: Helm Install the Web UI

```bash
helm install whisper-web-ui charts/web-ui/ \
  --set image.repository=your-registry/whisper-web-ui \
  --set image.tag=latest \
  --set tailscale.hostname=whisper
```

To customize further, create a `my-values.yaml`:

```yaml
image:
  repository: your-registry/whisper-web-ui
  tag: latest

tailscale:
  enabled: true
  hostname: whisper

whisperBackend:
  url: http://whisper-svc.ai-services.svc.cluster.local:8080
```

```bash
helm install whisper-web-ui charts/web-ui/ -f my-values.yaml
```

### Verify the web UI deployment

```bash
kubectl get pods
# Should show whisper-web-ui-xxx Running

kubectl get svc
# Should show whisper-web-ui with Tailscale annotations
```

## Step 6: Verify the Whisper Endpoint

Port-forward to test the whisper server directly:

```bash
kubectl port-forward -n ai-services svc/whisper-svc 8080:8080
```

In another terminal:

```bash
# Health check
curl http://localhost:8080/health
# Expected: {"status":"ok"}

# Test transcription with a WAV file
curl -X POST http://localhost:8080/inference \
  -F "file=@test-audio.wav" \
  -F "response_format=text"
# Expected: transcribed text output
```

## Step 7: Verify the Web UI via Tailscale

1. Ensure your client device is connected to your tailnet.
2. Open `https://whisper.<your-tailnet>.ts.net` in a browser.
3. The page should load with a valid TLS certificate (issued by Tailscale).
4. Test transcription through the UI or via curl:

```bash
curl -X POST https://whisper.<your-tailnet>.ts.net/api/transcribe \
  -F "file=@test-audio.wav" \
  -F "response_format=text"
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Whisper pod `CrashLoopBackOff` | Check `kubectl logs -n ai-services deploy/whisper-server`. Usually a missing model file or OOM. |
| PVC stuck in `Pending` | Verify Longhorn is installed: `kubectl get sc longhorn`. Check Longhorn UI for node availability. |
| Web UI can't reach whisper | Verify DNS resolution: `kubectl exec -it <web-ui-pod> -- nslookup whisper-svc.ai-services.svc.cluster.local` |
| Tailscale hostname not resolving | Wait 1-2 min after deploy. Check operator logs: `kubectl logs -n tailscale deploy/tailscale-operator` |
| Large audio files rejected | The nginx config allows up to 50MB. For larger files, update `client_max_body_size` in the Helm values. |

## Architecture Overview

```
Internet (blocked)
       │
       ╳ (no external ingress)
       │
┌──────┴──────────────────────────────────────────────┐
│  Kubernetes Cluster                                  │
│                                                      │
│  ┌─────────────────────┐    ┌────────────────────┐  │
│  │  whisper-web-ui      │    │  whisper-server     │  │
│  │  (nginx + SPA)       │───>│  (whisper.cpp)      │  │
│  │  port 80             │    │  port 8080          │  │
│  └──────┬──────────────┘    └──────┬─────────────┘  │
│         │                          │                 │
│   Tailscale Proxy             Longhorn PVCs          │
│   (auto-created)            ┌──────┴──────┐         │
│         │                   │  models (5Gi) │         │
│         │                   │  transcripts  │         │
│         │                   │    (10Gi)     │         │
│         │                   └─────────────┘         │
└─────────┼───────────────────────────────────────────┘
          │
    Tailnet (WireGuard)
          │
    ┌─────┴─────┐
    │  Your      │
    │  Devices   │
    │            │
    │ https://whisper.<tailnet>.ts.net
    └───────────┘
```
