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

Files are stored in object storage only. The gateway creates short-lived S3-compatible presigned URLs, records file metadata, and passes file IDs through jobs instead of moving file bytes through HTTP or RPC.

Upload flow:

1. `POST /api/v1/files/presign` with `filename`, `content_type`, and `size`.
2. Upload the file directly to the returned `upload_url`.
3. `POST /api/v1/files/{file_id}/complete`.
4. Create jobs with `input_file_ids`.

The first milestone is to wire durable metadata persistence, queue publishing, worker execution, and result lookup.
