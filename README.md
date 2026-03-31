# plexmatch-webhook

A lightweight Go service that receives Sonarr webhook events and writes `.plexmatch` hint files into series folders so Plex matches shows correctly using provider IDs rather than title/year guessing.

## How it works

1. Sonarr fires a webhook on **Download**, **Upgrade**, or **Rename** events.
2. The service extracts the series path and provider IDs (TVDB, TMDB, IMDB).
3. A `.plexmatch` file is atomically written into the series folder.
4. Optionally, a Plex library refresh is triggered so Plex picks up the hint immediately.

### Example `.plexmatch` output

```
# managed by sonarr-plexmatch
tvdbid: 76156
guid: tvdb://76156
title: Scrubs
year: 2001
```

## Endpoints

| Method | Path               | Description                              |
|--------|--------------------|------------------------------------------|
| POST   | `/sonarr/webhook`  | Receive Sonarr webhook events            |
| POST   | `/reconcile`       | Bulk-write `.plexmatch` for many shows   |
| GET    | `/healthz`         | Liveness/readiness probe (returns `ok`)  |
| GET    | `/metrics`         | Prometheus metrics                       |

## Configuration

All configuration is via environment variables.

| Variable              | Default                   | Description                                                        |
|-----------------------|---------------------------|--------------------------------------------------------------------|
| `LISTEN_ADDR`         | `0.0.0.0`                 | Bind address                                                       |
| `PORT`                | `8080`                    | Listen port                                                        |
| `WEBHOOK_SECRET`      | _(empty)_                 | Shared secret validated against `X-Webhook-Secret` request header  |
| `PLEX_BASE_URL`       | _(empty)_                 | Plex server URL, e.g. `http://plex:32400`                          |
| `PLEX_TOKEN`          | _(empty)_                 | Plex authentication token                                          |
| `PLEX_SECTION_ID`     | _(empty)_                 | Plex library section ID to refresh after writes                    |
| `PREFER_ID_SOURCE`    | `tvdb`                    | Preferred provider: `tvdb`, `tmdb`, or `imdb`                      |
| `WRITE_GUID`          | `true`                    | Include `guid: provider://id` line in `.plexmatch`                 |
| `DRY_RUN`             | `false`                   | Log actions without writing files or calling Plex                  |
| `ALLOWED_EVENT_TYPES` | `Download,Upgrade,Rename` | Comma-separated Sonarr event types to process                      |

Plex integration is enabled only when all three of `PLEX_BASE_URL`, `PLEX_TOKEN`, and `PLEX_SECTION_ID` are set.

### Finding your Plex token and section ID

**Plex token**: Open any media item in Plex web, click "Get Info" > "View XML", and look for `X-Plex-Token` in the URL.

**Section ID**: Run:

```bash
curl -s "http://<PLEX_HOST>:32400/library/sections?X-Plex-Token=<TOKEN>" | xmllint --xpath '//Directory/@key | //Directory/@title' -
```

Or visit `http://<PLEX_HOST>:32400/library/sections?X-Plex-Token=<TOKEN>` in a browser and note the `key` attribute of your TV library section.

## Deployment

### Docker

The image is published to GHCR on tagged releases:

```bash
docker pull ghcr.io/mkurdziel/plexmatch-webhook:latest
```

Run directly:

```bash
docker run -d \
  --name plexmatch-webhook \
  -p 8080:8080 \
  -v /path/to/tv:/tv \
  -e WEBHOOK_SECRET=my-secret \
  -e PLEX_BASE_URL=http://plex:32400 \
  -e PLEX_TOKEN=your-plex-token \
  -e PLEX_SECTION_ID=2 \
  ghcr.io/mkurdziel/plexmatch-webhook:latest
```

The container must be able to write to the same filesystem paths that Sonarr reports in `series.path`. Mount the media volume accordingly.

### Kubernetes / Helm

Below is a complete set of Kubernetes manifests. Adjust namespaces, volume paths, and secret values for your cluster.

#### Namespace (optional)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: plexmatch
```

#### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: plexmatch-webhook
  namespace: plexmatch
type: Opaque
stringData:
  WEBHOOK_SECRET: "your-webhook-secret"
  PLEX_TOKEN: "your-plex-token"
```

#### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: plexmatch-webhook
  namespace: plexmatch
data:
  PLEX_BASE_URL: "http://plex.plex.svc.cluster.local:32400"
  PLEX_SECTION_ID: "2"
  PREFER_ID_SOURCE: "tvdb"
  WRITE_GUID: "true"
  ALLOWED_EVENT_TYPES: "Download,Upgrade,Rename"
```

#### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: plexmatch-webhook
  namespace: plexmatch
  labels:
    app: plexmatch-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: plexmatch-webhook
  template:
    metadata:
      labels:
        app: plexmatch-webhook
    spec:
      containers:
        - name: plexmatch-webhook
          image: ghcr.io/mkurdziel/plexmatch-webhook:latest
          ports:
            - containerPort: 8080
              name: http
          envFrom:
            - configMapRef:
                name: plexmatch-webhook
            - secretRef:
                name: plexmatch-webhook
          volumeMounts:
            - name: tv-media
              mountPath: /tv
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 2
            periodSeconds: 10
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
            limits:
              memory: 64Mi
      volumes:
        - name: tv-media
          persistentVolumeClaim:
            claimName: tv-media
```

The `tv-media` PVC must point to the same storage backing your Sonarr and Plex media. The mount path must match what Sonarr reports in `series.path`. If Sonarr sees shows at `/tv/Show Name/`, then mount your PVC at `/tv`.

If you use NFS or a hostPath, replace the PVC volume with the appropriate type:

```yaml
volumes:
  - name: tv-media
    nfs:
      server: nas.local
      path: /volume1/tv
```

#### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: plexmatch-webhook
  namespace: plexmatch
spec:
  selector:
    app: plexmatch-webhook
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
```

#### ServiceMonitor (optional, for Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: plexmatch-webhook
  namespace: plexmatch
spec:
  selector:
    matchLabels:
      app: plexmatch-webhook
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

### Connecting Sonarr

1. In Sonarr, go to **Settings > Connect > Add > Webhook**.
2. Set the **URL** to the service address within your cluster:
   ```
   http://plexmatch-webhook.plexmatch.svc.cluster.local/sonarr/webhook
   ```
   Or if Sonarr is outside the cluster, use the external Service/Ingress URL.
3. Set **Method** to `POST`.
4. Add a custom header: `X-Webhook-Secret: your-webhook-secret` (matching the secret in your deployment).
5. Under **Events**, enable:
   - On Import
   - On Upgrade
   - On Rename

## Sonarr webhook payload

The service expects the standard Sonarr webhook JSON payload. The relevant fields:

```json
{
  "eventType": "Download",
  "series": {
    "path": "/tv/Scrubs",
    "title": "Scrubs",
    "year": 2001,
    "tvdbId": 76156,
    "tmdbId": 2155,
    "imdbId": "tt0285403"
  }
}
```

- `eventType` must be in `ALLOWED_EVENT_TYPES` or the request is ignored (returns `202`).
- `series.path` is required.
- At least one of `tvdbId`, `tmdbId`, or `imdbId` is required.

## Reconcile endpoint

For bulk backfill (e.g., after a migration or first install), POST an array of shows:

```bash
curl -X POST http://plexmatch-webhook:8080/reconcile \
  -H "Content-Type: application/json" \
  -d '{
    "shows": [
      {"path": "/tv/Scrubs", "title": "Scrubs", "year": 2001, "tvdbId": 76156},
      {"path": "/tv/The Office", "title": "The Office", "year": 2005, "tvdbId": 73244}
    ]
  }'
```

Response:

```json
{
  "written": 2,
  "noop": 0,
  "errors": []
}
```

You can generate the shows list from the Sonarr API:

```bash
curl -s "http://sonarr:8989/api/v3/series?apikey=YOUR_KEY" | \
  jq '{shows: [.[] | {path, title, year, tvdbId, tmdbId, imdbId}]}' | \
  curl -X POST http://plexmatch-webhook:8080/reconcile \
    -H "Content-Type: application/json" -d @-
```

## Metrics

Prometheus counters exported at `/metrics`:

| Metric                                 | Description                             |
|----------------------------------------|-----------------------------------------|
| `plexmatch_webhook_requests_total`     | Total webhook requests received         |
| `plexmatch_ignored_events_total`       | Events ignored (irrelevant event type)  |
| `plexmatch_malformed_payloads_total`   | Invalid payloads rejected               |
| `plexmatch_files_written_total`        | `.plexmatch` files written              |
| `plexmatch_noop_writes_total`          | Writes skipped (content unchanged)      |
| `plexmatch_plex_refresh_success_total` | Successful Plex library refreshes       |
| `plexmatch_plex_refresh_failure_total` | Failed Plex library refreshes           |
| `plexmatch_retry_enqueued_total`       | Tasks enqueued for retry                |

## Retry behavior

- If the series folder doesn't exist yet when a webhook arrives, the write is retried every **10 seconds** up to **5 attempts**.
- If a Plex library refresh fails, it is retried on the same schedule.
- The retry queue is in-memory and does not persist across restarts.

## File writing

- Files are written atomically: content goes to a temp file in the same directory, fsynced, then renamed into place.
- If the existing `.plexmatch` file already matches the expected content, no write occurs (idempotent).
- The service never deletes `.plexmatch` files.

## Building from source

```bash
git clone https://github.com/mkurdziel/plexmatch-webhook.git
cd plexmatch-webhook
CGO_ENABLED=0 go build -o plexmatch-webhook .
./plexmatch-webhook
```

## Docker build

```bash
docker build -t plexmatch-webhook .
```

## CI/CD

- **CI**: Builds and tests on every push (`.github/workflows/ci.yml`).
- **Release**: On tags matching `v*`, builds and pushes the Docker image to `ghcr.io/mkurdziel/plexmatch-webhook` (`.github/workflows/release.yml`). Tag `v1.2.3` produces image tags `1.2.3`, `1.2`, `1`, and `latest`.

## License

See [LICENSE](LICENSE).
