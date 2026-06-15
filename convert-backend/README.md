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

The first milestone is to wire upload metadata, job creation, queue publishing, worker execution, and result lookup.
