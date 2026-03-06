# Transcript Storage Design

This document describes how whisper.cpp transcription output is stored, accessed, and backed up in the Kubernetes deployment.

## Where Transcripts Are Written

The whisper-server writes completed transcriptions to the Longhorn-backed PVC mounted at `/data/transcripts/`. Files are named by job ID:

```
/data/transcripts/
├── {job-id-1}.txt
├── {job-id-2}.txt
└── ...
```

The `whisper-transcripts` PVC is a 10Gi Longhorn volume with `ReadWriteOnce` access mode, attached to the single whisper-server pod.

## Access Patterns

Choose one of the following access patterns based on your needs. All three are valid; pick whichever fits your homelab architecture.

### Option A — Direct PVC Mount

Mount the same Longhorn PVC into another pod (file browser, downstream processing service, etc.).

**Requirements:**
- Change the `whisper-transcripts` PVC to `ReadWriteMany` access mode.
- Both pods must be scheduled on the same node (unless Longhorn RWX is backed by NFS).

**Example:** Add to a consumer deployment:

```yaml
volumes:
  - name: transcripts
    persistentVolumeClaim:
      claimName: whisper-transcripts
containers:
  - name: consumer
    volumeMounts:
      - name: transcripts
        mountPath: /data/transcripts
        readOnly: true
```

**Pros:** Zero additional infrastructure. Direct filesystem access.
**Cons:** Tight coupling between pods. PVC access mode change required.

### Option B — REST Polling

The whisper-server writes transcript files to the PVC. A sidecar container or separate service watches the directory and exposes transcripts over HTTP.

**Flow:**
1. Client sends audio to `POST /inference` on the whisper-server.
2. Whisper-server processes and writes result to `/data/transcripts/{job-id}.txt`.
3. Client polls `GET /transcripts/{job-id}` on the sidecar service.
4. Sidecar reads from the shared volume and returns the content.

**Example sidecar (nginx):**

```yaml
containers:
  - name: transcript-api
    image: nginx:alpine
    volumeMounts:
      - name: transcripts
        mountPath: /usr/share/nginx/html/transcripts
        readOnly: true
    ports:
      - containerPort: 8081
```

**Pros:** Clean API boundary. No PVC access mode changes needed.
**Cons:** Adds a container. Polling latency.

### Option C — S3-Compatible Sink (MinIO)

Use a watcher job (e.g., `inotifywait` or a small Go binary) that monitors `/data/transcripts/` and uploads new files to a MinIO bucket.

**Flow:**
1. Whisper-server writes transcript to PVC.
2. Watcher detects new file via inotify.
3. Watcher uploads to `s3://transcripts/{job-id}.txt` on MinIO.
4. Downstream consumers read from MinIO using standard S3 APIs.

**Pros:** Decoupled storage. S3 API is universally supported. Easy to add lifecycle policies.
**Cons:** Requires MinIO (or compatible S3 service) in the cluster. Additional watcher process.

## Longhorn Backup

Longhorn supports snapshots and backups to an S3-compatible target. Recommended configuration for the transcripts PVC:

1. **Scheduled Snapshots**: Create a recurring snapshot every 6 hours via the Longhorn UI or a `RecurringJob` resource:

```yaml
apiVersion: longhorn.io/v1beta2
kind: RecurringJob
metadata:
  name: whisper-transcripts-snapshot
  namespace: longhorn-system
spec:
  cron: "0 */6 * * *"
  task: snapshot
  groups:
    - default
  retain: 8
  concurrency: 1
  labels:
    app: whisper-transcripts
```

2. **S3 Backup Target**: Configure Longhorn to back up volumes to MinIO or any S3-compatible endpoint via the Longhorn settings UI under **Backup Target**.

3. **Disaster Recovery**: Longhorn volumes can be restored from any snapshot or S3 backup — useful if the transcripts PVC is accidentally deleted.

## Wiring to the Web UI

The PedroCLI web UI can interact with whisper-server directly over the cluster network:

1. **Transcription Request**: The web UI's nginx reverse proxy forwards `POST /api/transcribe` to `http://whisper-svc.ai-services.svc.cluster.local:8080/inference`.

2. **Result Retrieval**: The whisper-server returns the transcription result synchronously in the HTTP response body. No separate polling step is needed for short audio clips.

3. **Long Audio (future)**: For longer recordings, implement an async flow:
   - `POST /inference` returns a `{ "job_id": "..." }` immediately.
   - Client polls `GET /transcripts/{job_id}` until the result is ready.
   - This requires the REST polling sidecar from Option B above.

4. **DNS Resolution**: Within the cluster, the web UI deployment resolves `whisper-svc.ai-services.svc.cluster.local` via CoreDNS. The nginx config in the web UI Helm chart is pre-configured with this address.
