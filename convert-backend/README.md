# convert-backend

Microservice backend skeleton for file conversion and processing.

## Services

- `gateway`: public HTTP API gateway.
- `worker`: asynchronous job consumer.
- `pdf-service`: placeholder processor service for PDF tasks.
- `image-service`: Python processor service skeleton.
- `media-service`: Python processor service skeleton.

## Local development

```powershell
cd convert-backend
go test ./...
go run ./cmd/gateway
```

The gateway loads configuration in this order:

1. Built-in defaults.
2. YAML file from `CONVERT_CONFIG`, defaulting to `configs/gateway.dev.yaml`.
3. Environment variable overrides.

Useful local commands:

```powershell
# Run gateway with the default YAML config.
go run ./cmd/gateway

# Override the HTTP address.
$env:CONVERT_HTTP_ADDR=':18080'; go run ./cmd/gateway

# Run the worker skeleton.
go run ./cmd/worker

# Run the PDF service skeleton.
go run ./cmd/pdf-service
```

The gateway, worker, and processor services all use structured JSON logs through `slog`. Every logger is created with a `service` field, for example `convert-gateway`, `convert-worker`, or `pdf-service`.

Files are stored in object storage only. The gateway creates short-lived S3-compatible presigned URLs, records file metadata, and passes file IDs through jobs instead of moving file bytes through HTTP or RPC.

Upload flow:

1. `POST /api/v1/files/presign` with `filename`, `content_type`, and `size`.
2. Upload the file directly to the returned `upload_url`.
3. `POST /api/v1/files/{file_id}/complete`.
4. Create jobs with `input_file_ids`.

The first milestone is to wire durable metadata persistence, queue publishing, worker execution, and result lookup.
