# MinIO / S3 Storage

Convert Backend stores uploaded files and generated results in S3-compatible object storage. Local development uses MinIO.

## Bucket

Default bucket:

```text
convert
```

## Object Keys

```text
uploads/{user_id}/{file_id}/{safe_filename}
results/{user_id}/{job_id}/{safe_filename}
temp/{service}/{job_id}/{safe_filename}
```

## Docker Compose Init

The local Docker Compose setup includes a `minio-init` container that creates the `convert` bucket:

```sh
mc alias set local http://minio:9000 convert convert-secret
mc mb --ignore-existing local/convert
```

## Manual Init

If MinIO is already running locally, create the bucket manually:

```powershell
docker run --rm --network host minio/mc `
  mb --ignore-existing local/convert
```

or use the MinIO console at:

```text
http://localhost:19001
```

Default local credentials:

```text
access key: convert
secret key: convert-secret
```
