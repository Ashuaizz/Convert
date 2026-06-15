# Convert Backend v0 详细设计与 Commit 拆分

## 1. v0 目标

v0 要完成一个可运行、可演示、可继续扩展的后端主链路。

核心验收流程：

1. 用户或测试客户端请求 Gateway 创建上传链接。
2. 客户端把文件上传到 MinIO。
3. 客户端通知 Gateway 上传完成。
4. 客户端创建一个处理任务，例如 `image.resize`。
5. Gateway 写入任务元数据并发布队列消息。
6. Worker 消费任务。
7. Worker 调用处理服务或执行处理器。
8. 处理结果写回 MinIO。
9. Worker 更新任务状态为 `succeeded`。
10. 客户端查询任务并拿到下载链接。

v0 的重点不是支持所有文件格式，而是证明“网关、存储、任务、队列、处理服务”这条链路成立。

## 2. v0 技术边界

### 2.1 必须实现

- Go Gateway。
- HTTP API。
- PostgreSQL 元数据存储。
- MinIO 对象存储。
- NATS JetStream 队列。
- Worker 消费任务。
- 至少一个处理能力：建议 `image.resize`。
- Docker Compose 本地启动。
- 基础日志、错误码、request_id。

### 2.2 暂不实现

- 用户注册登录完整体系。
- 多租户计费。
- 管理后台。
- Kubernetes。
- 服务自动发现。
- 复杂 OCR。
- 视频完整转码能力。
- OpenTelemetry 全链路追踪。

这些能力可以在 v1 或后续版本增加。

## 3. 服务划分

### 3.1 gateway

语言：Go。

推荐依赖：

- HTTP 路由：chi。
- 数据库：pgx 或 sqlc。
- 对象存储：minio-go 或 AWS S3 SDK。
- 队列：nats.go。
- 日志：slog 起步，后续可切 zap/zerolog。

职责：

- HTTP API。
- request_id。
- 鉴权预留。
- 上传链接签发。
- 下载链接签发。
- 文件元数据写入。
- 任务创建。
- 任务查询。
- 任务取消。
- 发布队列消息。

### 3.2 worker

语言：Go。

职责：

- 订阅 NATS 任务。
- 查询任务元数据。
- 加载输入文件信息。
- 根据任务类型选择 executor。
- 更新任务状态。
- 写入输出文件记录。
- 处理失败重试。

v0 可以先把 worker 和 executor 放在一个 Go 进程里。后续再把不同类型 worker 拆成独立服务。

### 3.3 image-service

语言：Python。

职责：

- 提供图片处理能力。
- v0 实现 `image.resize`。
- 从对象存储读取输入。
- 处理后写回对象存储。

v0 也可以先由 worker 直接调用 Python HTTP 接口。等 proto 稳定后再切换为 gRPC。

### 3.4 pdf-service

v0 只保留骨架和健康检查。

如果时间允许，增加 `pdf.merge`。

### 3.5 media-service

v0 只保留骨架和健康检查。

如果时间允许，增加 `video.snapshot`。

## 4. HTTP API 设计

### 4.1 健康检查

```http
GET /healthz
```

返回：

```json
{
  "code": "OK",
  "data": {
    "status": "ok"
  }
}
```

### 4.2 创建上传链接

```http
POST /api/v1/files/presign
```

请求：

```json
{
  "filename": "demo.jpg",
  "content_type": "image/jpeg",
  "size": 1048576,
  "sha256": "optional"
}
```

返回：

```json
{
  "code": "OK",
  "data": {
    "file_id": "file_xxx",
    "upload_url": "http://localhost:9000/...",
    "storage_uri": "s3://convert/uploads/dev-user/file_xxx/demo.jpg",
    "expires_in": 900
  }
}
```

### 4.3 上传完成确认

```http
POST /api/v1/files/{file_id}/complete
```

返回：

```json
{
  "code": "OK",
  "data": {
    "file_id": "file_xxx",
    "status": "uploaded"
  }
}
```

### 4.4 创建任务

```http
POST /api/v1/jobs
```

请求：

```json
{
  "type": "image.resize",
  "input_file_ids": ["file_xxx"],
  "options": {
    "width": 800,
    "height": 600,
    "format": "webp",
    "quality": 85
  }
}
```

返回：

```json
{
  "code": "OK",
  "data": {
    "job_id": "job_xxx",
    "status": "queued"
  }
}
```

### 4.5 查询任务

```http
GET /api/v1/jobs/{job_id}
```

返回：

```json
{
  "code": "OK",
  "data": {
    "job_id": "job_xxx",
    "type": "image.resize",
    "status": "succeeded",
    "progress": 100,
    "result_files": [
      {
        "file_id": "file_result_xxx",
        "filename": "demo.webp",
        "content_type": "image/webp",
        "size": 123456
      }
    ]
  }
}
```

### 4.6 获取下载链接

```http
GET /api/v1/files/{file_id}/download
```

返回：

```json
{
  "code": "OK",
  "data": {
    "download_url": "http://localhost:9000/...",
    "expires_in": 900
  }
}
```

## 5. 数据库设计

v0 使用四张核心表：

- `users`
- `files`
- `jobs`
- `job_files`

v0 可以先使用一个固定测试用户：

```text
dev-user
```

这样可以先把文件和任务链路跑通，后续再接入真实鉴权。

### 5.1 files

核心字段：

- `id`
- `user_id`
- `filename`
- `content_type`
- `size`
- `sha256`
- `storage_uri`
- `status`
- `created_at`
- `updated_at`

状态：

- `pending_upload`
- `uploaded`
- `deleted`

### 5.2 jobs

核心字段：

- `id`
- `user_id`
- `type`
- `status`
- `progress`
- `options`
- `message`
- `error_code`
- `error_message`
- `retry_count`
- `created_at`
- `updated_at`
- `started_at`
- `finished_at`

状态：

- `queued`
- `running`
- `succeeded`
- `failed`
- `retrying`
- `canceling`
- `canceled`

### 5.3 job_files

用于表达任务和文件的关系。

role：

- `input`
- `output`
- `preview`
- `log`

## 6. 对象存储设计

v0 使用 MinIO。

Bucket：

```text
convert
```

Key 规范：

```text
uploads/{user_id}/{file_id}/{safe_filename}
results/{user_id}/{job_id}/{safe_filename}
temp/{service}/{job_id}/{safe_filename}
```

注意：

- 文件名必须清洗。
- 不信任用户传入的扩展名。
- 下载链接必须短期有效。
- 处理服务不要长期依赖本地临时文件。

## 7. 队列设计

v0 使用 NATS JetStream。

Stream：

```text
convert_jobs
```

Subject：

```text
jobs.created
```

消息：

```json
{
  "job_id": "job_xxx",
  "type": "image.resize",
  "created_at": "2026-06-15T10:00:00+08:00"
}
```

v0 可以一个 worker 消费所有任务。

后续拆分：

```text
jobs.image
jobs.pdf
jobs.media
```

## 8. image.resize 执行设计

处理流程：

1. Worker 消费到 `image.resize`。
2. Worker 查询 job 和 input file。
3. Worker 把文件 URI 和 options 发给 image-service。
4. image-service 从 MinIO 下载文件到临时目录。
5. image-service 使用 Pillow 打开图片。
6. 校验像素数量。
7. resize。
8. 转换为目标格式。
9. 上传结果到 MinIO。
10. 返回结果文件信息。
11. Worker 写入 output file 记录。
12. Worker 更新 job 为 `succeeded`。

参数限制：

| 参数 | 限制 |
| --- | --- |
| width | 1 到 10000 |
| height | 1 到 10000 |
| format | jpg、png、webp |
| quality | 1 到 100 |

## 9. 错误处理

统一错误码：

| 错误码 | 说明 |
| --- | --- |
| INVALID_ARGUMENT | 参数错误 |
| UNAUTHORIZED | 未登录 |
| FORBIDDEN | 无权限 |
| NOT_FOUND | 资源不存在 |
| PAYLOAD_TOO_LARGE | 文件过大 |
| UNSUPPORTED_MEDIA_TYPE | 文件类型不支持 |
| PROCESSOR_UNAVAILABLE | 处理服务不可用 |
| DEADLINE_EXCEEDED | 处理超时 |
| JOB_FAILED | 任务失败 |

任务失败时必须记录：

- `error_code`
- `error_message`
- `message`
- `finished_at`

## 10. v0 Commit 拆分计划

下面的 commit 顺序按“每次提交都能解释清楚、尽量可编译、逐步跑通链路”的思路设计。

### Commit 01: scaffold project structure

目标：建立项目骨架。

包含：

- `go.mod`
- `cmd/gateway`
- `cmd/worker`
- `cmd/pdf-service`
- `internal/gateway`
- `internal/worker`
- `internal/pkg`
- `services/image-service`
- `services/media-service`
- `api/proto`
- `api/openapi`
- `configs`
- `deploy`
- `migrations`

验收：

```powershell
go test ./...
go build ./cmd/gateway ./cmd/worker ./cmd/pdf-service
```

### Commit 02: add gateway routing and response envelope

目标：让 Gateway 具备稳定 HTTP 入口。

包含：

- 引入 chi。
- 路由分组 `/api/v1`。
- request_id middleware。
- recover middleware。
- JSON response helper。
- 统一错误响应。
- `GET /healthz`。

验收：

```powershell
go run ./cmd/gateway
curl http://localhost:8080/healthz
```

### Commit 03: add config and logging foundation

目标：所有服务使用统一配置和日志。

包含：

- YAML 配置加载。
- 环境变量覆盖。
- slog JSON 日志。
- gateway/worker/pdf-service 配置结构。
- README 中补本地启动说明。

验收：

- 改 `CONVERT_HTTP_ADDR` 后服务监听地址变化。
- 日志包含 service 字段。

### Commit 04: add database migrations and repository interface

目标：建立元数据模型。

包含：

- users/files/jobs/job_files migration。
- repository interface。
- PostgreSQL repository skeleton。
- 本地 dev-user seed。

验收：

- migration 可执行。
- repository 单元测试通过。

### Commit 05: integrate PostgreSQL in gateway

目标：Gateway 读写真实数据库。

包含：

- PostgreSQL 连接池。
- healthz 检查数据库。
- jobs repository 实现。
- files repository 实现。
- 优雅关闭连接。

验收：

- Gateway 启动后可以连接 PostgreSQL。
- 创建 job 后数据库有记录。

### Commit 06: add MinIO storage client

目标：接入对象存储。

包含：

- storage interface。
- MinIO/S3 client。
- bucket 初始化文档。
- storage key 生成工具。
- 文件名清洗。
- presigned upload。
- presigned download。

验收：

- 可以生成上传 URL。
- 可以生成下载 URL。

### Commit 07: implement file upload APIs

目标：跑通文件上传元数据流程。

包含：

- `POST /api/v1/files/presign`
- `POST /api/v1/files/{file_id}/complete`
- `GET /api/v1/files/{file_id}`
- 文件大小限制。
- content_type 白名单预留。

验收：

- 创建 file 记录状态为 `pending_upload`。
- complete 后状态变为 `uploaded`。

### Commit 08: add NATS queue publisher and consumer

目标：接入异步任务队列。

包含：

- NATS JetStream 初始化。
- queue publisher。
- queue consumer。
- job message schema。
- worker 消费循环。
- graceful shutdown。

验收：

- Gateway 发布消息。
- Worker 能收到消息并打印 job_id。

### Commit 09: implement job creation and query with queue

目标：创建任务后进入队列。

包含：

- `POST /api/v1/jobs`
- `GET /api/v1/jobs/{job_id}`
- 写 jobs。
- 写 job_files input。
- 发布队列消息。
- 基础参数校验。

验收：

- 创建 job 返回 `queued`。
- 数据库 jobs 有记录。
- Worker 收到对应 job。

### Commit 10: implement worker state transitions

目标：Worker 能管理任务状态。

包含：

- `queued -> running`
- `running -> succeeded`
- `running -> failed`
- 条件更新避免重复消费。
- error_code/error_message。
- retry_count 基础逻辑。

验收：

- Worker 消费后能更新任务状态。
- 重复消费不会重复执行。

### Commit 11: implement image-service health and process contract

目标：图片服务具备可调用接口。

包含：

- FastAPI healthz。
- `/process/image.resize`。
- 请求/响应模型。
- 参数校验。
- Dockerfile。

验收：

- image-service Docker 可启动。
- healthz 返回 ok。
- 参数错误返回明确错误。

### Commit 12: implement image.resize processing

目标：完成第一个真实处理能力。

包含：

- MinIO 下载输入文件。
- Pillow resize。
- 输出 jpg/png/webp。
- 上传结果到 MinIO。
- 返回 output file metadata。

验收：

- 给定图片可以生成缩放结果。
- 输出文件存在于 MinIO。

### Commit 13: wire worker to image-service

目标：Worker 调用 image-service 完成任务。

包含：

- image executor。
- HTTP client 或 gRPC client。
- job type 路由。
- 写 output file。
- 写 job_files output。
- 更新 job progress。

验收：

- 创建 `image.resize` job 后最终状态为 `succeeded`。
- 查询 job 能看到 result files。

### Commit 14: implement download API

目标：前端可以下载处理结果。

包含：

- `GET /api/v1/files/{file_id}/download`
- 权限检查。
- presigned download URL。

验收：

- result file 可以拿到下载链接。
- 非本人文件后续接鉴权时会被拒绝。

### Commit 15: complete docker compose local workflow

目标：一条命令启动开发依赖和服务。

包含：

- gateway。
- worker。
- postgres。
- minio。
- nats。
- image-service。
- 初始化说明。
- Makefile 或 scripts。

验收：

```powershell
docker compose -f deploy/docker-compose.yaml up --build
```

本地可以完成上传、创建任务、查询、下载。

### Commit 16: add integration test for v0 happy path

目标：用测试保护主链路。

包含：

- 测试图片素材。
- 启动依赖说明。
- 上传文件。
- 创建 `image.resize`。
- 等待任务成功。
- 下载结果。

验收：

- 集成测试可稳定跑通。

### Commit 17: harden validation and limits

目标：补安全边界。

包含：

- 上传大小限制。
- 图片像素数限制。
- content_type 白名单。
- 文件名清洗测试。
- job options schema 校验。
- 处理超时。

验收：

- 非法参数不会进入队列。
- 超限文件被拒绝。

### Commit 18: document v0 operation guide

目标：补齐使用和运维文档。

包含：

- 本地启动指南。
- API 示例。
- MinIO 使用说明。
- NATS 使用说明。
- 常见错误排查。
- v0 已知限制。

验收：

- 新开发者按文档能跑通完整链路。

## 11. v0 完成定义

v0 完成必须满足：

- `go test ./...` 通过。
- Docker Compose 可启动核心依赖。
- Gateway healthz 正常。
- image-service healthz 正常。
- 可以创建上传链接。
- 可以完成文件上传确认。
- 可以创建 `image.resize` 任务。
- Worker 可以消费任务。
- 任务最终成功。
- 查询任务能看到结果文件。
- 可以获取结果下载链接。

## 12. v0 之后的演进

v0 之后建议按以下方向推进：

1. 把 image-service HTTP contract 切换为 gRPC。
2. 增加 `video.snapshot`。
3. 增加 `pdf.merge`。
4. 增加 JWT 鉴权。
5. 增加 SSE/WebSocket 任务进度推送。
6. 增加 Prometheus metrics。
7. 引入 OpenTelemetry。
8. 按任务类型拆分 worker 队列。
9. 引入 Kubernetes 部署。

