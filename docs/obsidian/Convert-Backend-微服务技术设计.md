# Convert Backend 微服务技术设计

> 项目目标：建设一个轻量、可扩展的后端微服务系统。前端统一访问后端网关服务，网关负责鉴权、任务编排、文件上传下载、状态查询，并通过 RPC 调用后端 PDF、图片、音频、视频等处理服务。

## 1. 背景与目标

### 1.1 背景

Convert 项目面向多媒体与文档转换/处理场景，典型能力包括：

- PDF：合并、拆分、压缩、转图片、转 Word、OCR。
- 图片：格式转换、压缩、缩放、裁剪、去背景、批量处理。
- 音频：格式转换、压缩、提取音轨、音量标准化、转文字。
- 视频：格式转换、压缩、截帧、提取音频、生成封面、转码。

这些能力计算特征差异较大，部分任务耗时较长，适合拆分为独立处理服务，通过统一网关暴露给前端。

### 1.2 设计目标

- 前端只依赖统一网关 API，隐藏后端多服务细节。
- 网关使用 Go 实现，保持轻量、高并发、易部署。
- 具体处理服务按场景选择合适语言：
  - PDF 服务优先 Go 或 Python。
  - 图片服务优先 Python 或 Go。
  - 音视频服务优先 Python + FFmpeg。
- 同步轻任务支持直接 RPC 返回结果。
- 耗时任务通过异步任务模型处理。
- 所有文件统一存放在对象存储中；本地开发使用 MinIO 作为 S3 兼容对象存储，避免 RPC 传输大文件。
- 单机 Docker Compose 可启动，后续可平滑迁移 Kubernetes。

### 1.3 非目标

- 第一阶段不做复杂工作流编排平台。
- 第一阶段不做多租户计费系统。
- 第一阶段不把二进制大文件直接塞进数据库或 RPC 消息。
- 第一阶段不强依赖重量级服务治理框架。

## 2. 总体架构

```mermaid
flowchart LR
    FE["Frontend Web / Desktop"] --> GW["API Gateway<br/>Go + HTTP"]
    GW --> AUTH["Auth / Session<br/>JWT"]
    GW --> DB["PostgreSQL<br/>metadata"]
    GW --> STORE["Object Storage<br/>MinIO / S3"]
    GW --> MQ["Queue<br/>NATS JetStream / Redis Streams"]

    GW -- gRPC --> PDF["PDF Service<br/>Go/Python"]
    GW -- gRPC --> IMG["Image Service<br/>Python/Go"]
    GW -- gRPC --> AV["Audio Video Service<br/>Python + FFmpeg"]

    MQ --> PDFW["PDF Workers"]
    MQ --> IMGW["Image Workers"]
    MQ --> AVW["Media Workers"]

    PDFW --> STORE
    IMGW --> STORE
    AVW --> STORE
    PDFW --> DB
    IMGW --> DB
    AVW --> DB
```

### 2.1 服务划分

| 服务 | 推荐语言 | 框架 | 职责 |
| --- | --- | --- | --- |
| gateway | Go | chi 或 gin | HTTP API、鉴权、限流、任务编排、RPC 客户端、状态查询 |
| pdf-service | Go/Python | gRPC + 原生库 | PDF 拆分、合并、压缩、转图片、OCR 编排 |
| image-service | Python/Go | gRPC + Pillow/libvips | 图片压缩、缩放、裁剪、格式转换 |
| media-service | Python | gRPC + FFmpeg | 音频/视频转码、压缩、截帧、提取音频 |
| worker | Go/Python | Queue consumer | 执行异步任务、更新状态、写入结果 |
| storage | MinIO/S3 | - | 存储原始文件与处理结果 |
| metadata-db | PostgreSQL | - | 用户、文件、任务、结果元数据 |
| queue | NATS JetStream 或 Redis Streams | - | 异步任务分发、重试、削峰 |

### 2.2 通信方式

| 链路 | 协议 | 说明 |
| --- | --- | --- |
| 前端 -> 网关 | HTTP REST | 上传文件、创建任务、查询任务、下载结果 |
| 前端 -> 网关 | WebSocket/SSE | 可选，用于推送任务进度 |
| 网关 -> 处理服务 | gRPC | 轻量、强类型、适合内部 RPC |
| 网关/服务 -> 对象存储 | S3 API | 传文件地址，不传大文件内容 |
| 网关/服务 -> 队列 | NATS/Redis | 异步任务 |
| 服务 -> 数据库 | SQL | 只存元数据和状态 |

## 3. 技术选型

### 3.1 网关服务

推荐：

- 语言：Go
- HTTP 框架：`chi`
- RPC：`grpc-go`
- 配置：环境变量 + YAML
- 日志：`zap` 或 `zerolog`
- 校验：`go-playground/validator`
- 数据库访问：`sqlc` 或 `gorm`

选择理由：

- `chi` 足够轻量，标准库风格强，适合 API Gateway。
- gRPC 适合内部服务间调用，有明确 protobuf 契约。
- Go 的并发与网络性能适合网关层。

### 3.2 PDF 服务

可选方案：

- Go：适合合并、拆分、简单压缩，部署简单。
- Python：适合 OCR、复杂 PDF 处理、生态更完整。

建议第一阶段：

- 基础 PDF 能力放在 Go 服务。
- OCR 或复杂格式转换拆到 Python worker。

### 3.3 图片服务

推荐：

- Python + FastAPI/gRPC + Pillow/libvips。
- 大批量、高性能场景可引入 Go + bimg/libvips。

第一阶段可用 Python，方便处理格式和图像算法。

### 3.4 音视频服务

推荐：

- Python + gRPC + FFmpeg 命令行封装。

设计原则：

- 服务只编排 FFmpeg，不自行实现编解码。
- 对输入参数做白名单校验，避免命令注入。
- 每个任务设置超时、CPU/内存限制和输出文件大小限制。

### 3.5 队列

推荐第一阶段：NATS JetStream。

备选：Redis Streams。

| 方案 | 优点 | 适用 |
| --- | --- | --- |
| NATS JetStream | 轻量、部署简单、消息语义清晰 | 微服务异步任务 |
| Redis Streams | 团队熟悉度高、可兼做缓存 | 小规模系统 |
| RabbitMQ | 功能完整 | 复杂路由与可靠投递 |
| Kafka | 吞吐高 | 大规模日志与事件流 |

本项目第一阶段建议使用 NATS JetStream，保持轻量和清晰。

### 3.6 存储

开发环境：

- MinIO

生产环境：

- S3、OSS、COS、MinIO 集群均可。

路径规范：

```text
uploads/{user_id}/{file_id}/{original_name}
results/{user_id}/{job_id}/{result_name}
temp/{service}/{job_id}/{filename}
```

## 4. 核心业务流程

### 4.1 文件上传流程

```mermaid
sequenceDiagram
    participant FE as Frontend
    participant GW as Gateway
    participant DB as PostgreSQL
    participant S3 as Object Storage

    FE->>GW: POST /api/v1/files/presign
    GW->>DB: create file metadata
    GW->>S3: create presigned upload url
    GW-->>FE: upload url + file_id
    FE->>S3: upload file
    FE->>GW: POST /api/v1/files/{file_id}/complete
    GW->>DB: mark uploaded
    GW-->>FE: file metadata
```

### 4.2 同步处理流程

适合小文件、快速操作，例如图片尺寸查询、小图压缩预览。

```mermaid
sequenceDiagram
    participant FE as Frontend
    participant GW as Gateway
    participant SVC as Process Service
    participant S3 as Object Storage

    FE->>GW: POST /api/v1/convert/sync
    GW->>SVC: gRPC ProcessSync(file_uri, options)
    SVC->>S3: read input and write result
    SVC-->>GW: result_uri
    GW-->>FE: result download url
```

### 4.3 异步任务流程

适合大文件、音视频转码、PDF OCR、批量处理。

```mermaid
sequenceDiagram
    participant FE as Frontend
    participant GW as Gateway
    participant DB as PostgreSQL
    participant MQ as Queue
    participant W as Worker
    participant S3 as Object Storage

    FE->>GW: POST /api/v1/jobs
    GW->>DB: create job status = queued
    GW->>MQ: publish job
    GW-->>FE: job_id

    W->>MQ: consume job
    W->>DB: status = running
    W->>S3: read input
    W->>W: process
    W->>S3: write result
    W->>DB: status = succeeded / failed

    FE->>GW: GET /api/v1/jobs/{job_id}
    GW-->>FE: job status + progress + result
```

## 5. API 设计

### 5.1 通用响应

```json
{
  "request_id": "req_01HX...",
  "code": "OK",
  "message": "success",
  "data": {}
}
```

错误响应：

```json
{
  "request_id": "req_01HX...",
  "code": "INVALID_ARGUMENT",
  "message": "unsupported target format",
  "details": {
    "field": "target_format"
  }
}
```

### 5.2 文件接口

#### 创建上传链接

```http
POST /api/v1/files/presign
Content-Type: application/json
Authorization: Bearer <token>
```

请求：

```json
{
  "filename": "demo.pdf",
  "content_type": "application/pdf",
  "size": 10485760,
  "sha256": "optional"
}
```

响应：

```json
{
  "file_id": "file_01HX...",
  "upload_url": "https://minio.example.com/...",
  "expires_in": 900
}
```

#### 上传完成确认

```http
POST /api/v1/files/{file_id}/complete
```

响应：

```json
{
  "file_id": "file_01HX...",
  "status": "uploaded",
  "uri": "s3://convert/uploads/u_001/file_01HX/demo.pdf"
}
```

### 5.3 创建异步任务

```http
POST /api/v1/jobs
Content-Type: application/json
Authorization: Bearer <token>
```

请求：

```json
{
  "type": "pdf.compress",
  "input_file_ids": ["file_01HX..."],
  "options": {
    "quality": "medium"
  },
  "callback_url": ""
}
```

响应：

```json
{
  "job_id": "job_01HX...",
  "status": "queued"
}
```

### 5.4 查询任务

```http
GET /api/v1/jobs/{job_id}
```

响应：

```json
{
  "job_id": "job_01HX...",
  "type": "pdf.compress",
  "status": "running",
  "progress": 45,
  "message": "compressing pages",
  "result_files": [],
  "created_at": "2026-06-15T09:00:00+08:00",
  "updated_at": "2026-06-15T09:01:00+08:00"
}
```

### 5.5 下载结果

```http
GET /api/v1/jobs/{job_id}/results/{result_file_id}/download
```

响应：

```json
{
  "download_url": "https://minio.example.com/...",
  "expires_in": 900
}
```

### 5.6 任务取消

```http
POST /api/v1/jobs/{job_id}/cancel
```

响应：

```json
{
  "job_id": "job_01HX...",
  "status": "canceling"
}
```

## 6. RPC 设计

### 6.1 Protobuf 目录建议

```text
api/proto/common/v1/common.proto
api/proto/processor/v1/processor.proto
api/proto/pdf/v1/pdf.proto
api/proto/image/v1/image.proto
api/proto/media/v1/media.proto
```

### 6.2 通用 Processor 服务

```proto
syntax = "proto3";

package processor.v1;

option go_package = "convert/api/proto/processor/v1;processorpb";

service ProcessorService {
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Validate(ValidateRequest) returns (ValidateResponse);
  rpc ProcessSync(ProcessSyncRequest) returns (ProcessSyncResponse);
  rpc StartJob(StartJobRequest) returns (StartJobResponse);
  rpc CancelJob(CancelJobRequest) returns (CancelJobResponse);
}

message HealthCheckRequest {}

message HealthCheckResponse {
  string status = 1;
  string version = 2;
}

message FileRef {
  string file_id = 1;
  string uri = 2;
  string content_type = 3;
  int64 size = 4;
}

message ProcessOption {
  string key = 1;
  string value = 2;
}

message ValidateRequest {
  string job_type = 1;
  repeated FileRef inputs = 2;
  map<string, string> options = 3;
}

message ValidateResponse {
  bool ok = 1;
  repeated string errors = 2;
}

message ProcessSyncRequest {
  string request_id = 1;
  string job_type = 2;
  repeated FileRef inputs = 3;
  map<string, string> options = 4;
}

message ProcessSyncResponse {
  repeated FileRef outputs = 1;
  string message = 2;
}

message StartJobRequest {
  string request_id = 1;
  string job_id = 2;
  string job_type = 3;
  repeated FileRef inputs = 4;
  map<string, string> options = 5;
}

message StartJobResponse {
  string job_id = 1;
  string accepted = 2;
}

message CancelJobRequest {
  string job_id = 1;
}

message CancelJobResponse {
  string status = 1;
}
```

### 6.3 服务能力声明

每个处理服务启动时应向网关注册或通过配置声明能力。

```yaml
processors:
  pdf-service:
    endpoint: pdf-service:9001
    capabilities:
      - pdf.merge
      - pdf.split
      - pdf.compress
      - pdf.to_images
  image-service:
    endpoint: image-service:9002
    capabilities:
      - image.resize
      - image.compress
      - image.convert
      - image.crop
  media-service:
    endpoint: media-service:9003
    capabilities:
      - audio.convert
      - video.convert
      - video.compress
      - video.extract_audio
      - video.snapshot
```

第一阶段可以用静态配置。后续如果服务数量增加，再引入 Consul、etcd 或 Kubernetes Service Discovery。

## 7. 任务模型

### 7.1 任务状态

```mermaid
stateDiagram-v2
    [*] --> queued
    queued --> running
    running --> succeeded
    running --> failed
    queued --> canceled
    running --> canceling
    canceling --> canceled
    failed --> retrying
    retrying --> queued
    succeeded --> [*]
    failed --> [*]
    canceled --> [*]
```

### 7.2 状态说明

| 状态 | 说明 |
| --- | --- |
| queued | 已创建，等待执行 |
| running | 正在处理 |
| succeeded | 成功完成 |
| failed | 处理失败 |
| retrying | 等待重试 |
| canceling | 取消中 |
| canceled | 已取消 |

### 7.3 重试策略

- 参数错误不重试。
- 文件不存在不重试。
- 处理服务临时不可用可重试。
- FFmpeg 进程异常退出可重试 1 到 2 次。
- 默认指数退避：10s、30s、120s。
- 超过最大重试次数后标记 `failed`。

### 7.4 幂等设计

- 前端创建任务时可传 `Idempotency-Key`。
- 网关以 `user_id + idempotency_key` 去重。
- Worker 执行前检查任务状态，避免重复执行。
- 输出路径包含 `job_id`，重复写入同一路径可被识别。

## 8. 数据库设计

### 8.1 users

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE,
    password_hash TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 8.2 files

```sql
CREATE TABLE files (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size BIGINT NOT NULL,
    sha256 TEXT,
    storage_uri TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_user_id_created_at ON files(user_id, created_at DESC);
```

### 8.3 jobs

```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    progress INT NOT NULL DEFAULT 0,
    options JSONB NOT NULL DEFAULT '{}',
    message TEXT,
    error_code TEXT,
    error_message TEXT,
    idempotency_key TEXT,
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_jobs_user_id_created_at ON jobs(user_id, created_at DESC);
CREATE INDEX idx_jobs_status_created_at ON jobs(status, created_at);
CREATE UNIQUE INDEX idx_jobs_idempotency
ON jobs(user_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;
```

### 8.4 job_files

```sql
CREATE TABLE job_files (
    job_id TEXT NOT NULL REFERENCES jobs(id),
    file_id TEXT NOT NULL REFERENCES files(id),
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, file_id, role)
);
```

role 取值：

- `input`
- `output`
- `preview`
- `log`

## 9. 配置设计

### 9.1 网关配置

```yaml
server:
  name: convert-gateway
  env: dev
  http_addr: ":8080"
  read_timeout: 30s
  write_timeout: 30s

jwt:
  issuer: convert
  access_token_ttl: 2h

database:
  dsn: postgres://convert:convert@postgres:5432/convert?sslmode=disable

storage:
  provider: s3
  endpoint: http://minio:9000
  bucket: convert
  region: us-east-1
  force_path_style: true

queue:
  provider: nats
  url: nats://nats:4222
  stream: convert_jobs

processors:
  pdf:
    endpoint: pdf-service:9001
  image:
    endpoint: image-service:9002
  media:
    endpoint: media-service:9003

limits:
  max_upload_size_mb: 500
  sync_timeout: 20s
  async_job_timeout: 30m
```

### 9.2 环境变量

```text
CONVERT_ENV=dev
CONVERT_CONFIG=configs/gateway.dev.yaml
CONVERT_JWT_SECRET=change-me
CONVERT_S3_ACCESS_KEY=convert
CONVERT_S3_SECRET_KEY=convert-secret
```

## 10. 代码仓库结构

建议使用单仓多服务结构，方便初期开发和统一契约管理。

```text
convert-backend/
  api/
    proto/
      common/v1/
      processor/v1/
      pdf/v1/
      image/v1/
      media/v1/
    openapi/
      gateway.yaml
  cmd/
    gateway/
      main.go
    pdf-service/
      main.go
  internal/
    gateway/
      handler/
      middleware/
      service/
      repository/
      rpcclient/
    pkg/
      config/
      logger/
      storage/
      queue/
      idgen/
  services/
    image-service/
      app/
      pyproject.toml
      Dockerfile
    media-service/
      app/
      pyproject.toml
      Dockerfile
  migrations/
  configs/
  deploy/
    docker-compose.yaml
    minio/
    postgres/
  docs/
```

## 11. 网关内部模块

```mermaid
flowchart TB
    HTTP["HTTP Handler"] --> MW["Middleware<br/>auth/rate limit/request id"]
    MW --> APP["Application Service"]
    APP --> REPO["Repository"]
    APP --> STORE["Storage Client"]
    APP --> QUEUE["Queue Producer"]
    APP --> RPC["Processor RPC Client"]
    REPO --> DB["PostgreSQL"]
    STORE --> S3["MinIO/S3"]
    QUEUE --> NATS["NATS"]
    RPC --> SVC["PDF/Image/Media Service"]
```

### 11.1 Handler 层

职责：

- 参数绑定与基础校验。
- 调用 application service。
- 统一响应格式。
- 不写业务细节。

### 11.2 Application Service 层

职责：

- 任务类型路由。
- 文件权限检查。
- 同步/异步任务策略选择。
- 事务编排。
- RPC 调用。

### 11.3 Repository 层

职责：

- 用户、文件、任务元数据读写。
- 保证 SQL 清晰可测试。

### 11.4 RPC Client 层

职责：

- 维护不同 processor 的 gRPC 连接。
- 超时与重试。
- 熔断与降级。
- 将内部错误转换为统一错误码。

## 12. 处理服务设计

### 12.1 PDF Service

能力：

- `pdf.merge`
- `pdf.split`
- `pdf.compress`
- `pdf.to_images`
- `pdf.ocr`

输入约束：

- 单个文件默认最大 200MB。
- 页数默认最大 1000 页。
- OCR 默认异步执行。

依赖：

- Go：pdfcpu、unidoc 可选。
- Python：pypdf、pymupdf、ocrmypdf、tesseract 可选。

### 12.2 Image Service

能力：

- `image.resize`
- `image.compress`
- `image.convert`
- `image.crop`
- `image.watermark`
- `image.metadata`

输入约束：

- 单图默认最大 50MB。
- 解码后像素数限制，例如 100MP。
- 输出格式白名单：jpg、png、webp、avif。

### 12.3 Media Service

能力：

- `audio.convert`
- `audio.normalize`
- `audio.extract_from_video`
- `video.convert`
- `video.compress`
- `video.snapshot`
- `video.extract_audio`

输入约束：

- 单文件默认最大 1GB。
- 单任务最长运行 30 分钟。
- FFmpeg 参数必须由后端模板生成，禁止透传用户命令。

FFmpeg 命令模板示例：

```text
ffmpeg -y -i {input} -c:v libx264 -preset medium -crf {crf} -c:a aac {output}
```

`crf` 取值必须限制在安全范围内，例如 18 到 35。

## 13. 安全设计

### 13.1 鉴权

- 前端请求使用 JWT。
- 上传、下载链接使用短期 presigned URL。
- 内部 RPC 可使用 mTLS 或内网访问控制。

### 13.2 文件安全

- 校验 content type 和真实文件魔数。
- 文件扩展名只作为展示，不作为可信依据。
- 解压类任务需要限制解压后大小，避免 zip bomb。
- 所有临时文件使用隔离目录。
- 处理完成后清理临时文件。

### 13.3 命令安全

音视频处理涉及 FFmpeg：

- 禁止拼接用户原始命令。
- 使用参数数组启动进程。
- 所有参数走枚举或范围校验。
- 进程运行设置超时。
- 容器限制 CPU、内存和磁盘。

### 13.4 权限隔离

- 用户只能访问自己的文件和任务。
- job 查询必须校验 `user_id`。
- result download 必须从 job 关系反查。

## 14. 可观测性

### 14.1 日志

统一字段：

```json
{
  "ts": "2026-06-15T09:00:00+08:00",
  "level": "info",
  "service": "convert-gateway",
  "request_id": "req_01HX...",
  "user_id": "u_001",
  "job_id": "job_01HX...",
  "message": "job created"
}
```

### 14.2 指标

建议暴露 Prometheus 指标：

- `http_requests_total`
- `http_request_duration_seconds`
- `jobs_created_total`
- `jobs_running`
- `jobs_failed_total`
- `processor_rpc_duration_seconds`
- `processor_rpc_errors_total`
- `storage_operation_duration_seconds`

### 14.3 链路追踪

推荐 OpenTelemetry：

- 网关生成 `request_id`。
- RPC metadata 透传 `request_id` 和 `trace_id`。
- Worker 日志带上 `job_id`。

## 15. 部署设计

### 15.1 开发环境 Docker Compose

```yaml
services:
  gateway:
    build:
      context: .
      dockerfile: cmd/gateway/Dockerfile
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - minio
      - nats

  pdf-service:
    build:
      context: .
      dockerfile: cmd/pdf-service/Dockerfile
    ports:
      - "9001:9001"

  image-service:
    build:
      context: services/image-service
    ports:
      - "9002:9002"

  media-service:
    build:
      context: services/media-service
    ports:
      - "9003:9003"

  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: convert
      POSTGRES_PASSWORD: convert
      POSTGRES_DB: convert
    ports:
      - "5432:5432"

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: convert
      MINIO_ROOT_PASSWORD: convert-secret
    ports:
      - "9000:9000"
      - "19001:9001"

  nats:
    image: nats:2
    command: ["-js"]
    ports:
      - "4222:4222"
```

注意：MinIO 控制台端口示例映射为 `19001`，避免和 PDF 服务 `9001` 冲突。

### 15.2 生产部署

第一阶段：

- Docker Compose + 反向代理。
- 服务按机器资源拆分。
- 音视频服务单独部署在高 CPU 机器。

后续：

- Kubernetes。
- HPA 按队列长度或 CPU 扩缩容。
- 对象存储换成云厂商 S3 兼容服务。
- PostgreSQL 使用托管数据库。

## 16. 扩展点

### 16.1 新增处理能力

新增一个能力的步骤：

1. 在能力表中新增 job type。
2. 在 protobuf 或服务配置中声明能力。
3. 处理服务实现对应 handler。
4. 网关增加参数校验 schema。
5. 前端展示对应操作入口。
6. 补充集成测试。

### 16.2 插件化 Processor

后续可以把处理服务抽象为 Processor 插件：

```yaml
capability: image.resize
processor: image-service
mode: sync_or_async
max_input_size_mb: 50
timeout: 60s
option_schema:
  width:
    type: integer
    min: 1
    max: 10000
  height:
    type: integer
    min: 1
    max: 10000
```

网关据此完成：

- 参数校验。
- 服务路由。
- 同步/异步选择。
- 前端表单生成。

## 17. 错误码

| 错误码 | HTTP 状态 | 说明 |
| --- | --- | --- |
| OK | 200 | 成功 |
| INVALID_ARGUMENT | 400 | 参数错误 |
| UNAUTHORIZED | 401 | 未登录或 token 无效 |
| FORBIDDEN | 403 | 无权限 |
| NOT_FOUND | 404 | 资源不存在 |
| PAYLOAD_TOO_LARGE | 413 | 文件过大 |
| UNSUPPORTED_MEDIA_TYPE | 415 | 文件类型不支持 |
| RATE_LIMITED | 429 | 请求过于频繁 |
| JOB_FAILED | 500 | 任务执行失败 |
| PROCESSOR_UNAVAILABLE | 503 | 处理服务不可用 |
| DEADLINE_EXCEEDED | 504 | 处理超时 |

## 18. 测试策略

### 18.1 单元测试

- 参数校验。
- job type 路由。
- repository SQL。
- storage key 生成。
- RPC error mapping。

### 18.2 集成测试

- 使用 testcontainers 启动 PostgreSQL、MinIO、NATS。
- 覆盖文件上传、创建任务、worker 完成任务、下载结果。

### 18.3 合约测试

- protobuf 生成代码作为服务间契约。
- OpenAPI 文档作为前后端契约。
- CI 检查 proto 和 OpenAPI 是否可生成。

### 18.4 媒体样本测试

维护小型测试素材：

```text
testdata/
  pdf/
    small.pdf
    encrypted.pdf
  image/
    sample.jpg
    transparent.png
    large.webp
  media/
    short.mp4
    audio.mp3
```

## 19. 开发里程碑

### M1：网关和基础设施

- Go gateway 初始化。
- PostgreSQL migrations。
- MinIO 上传下载。
- NATS JetStream 队列。
- 统一响应、日志、request_id。

### M2：任务系统

- 创建任务。
- 查询任务。
- Worker 消费。
- 状态更新。
- 失败重试。

### M3：PDF 基础能力

- PDF 合并。
- PDF 拆分。
- PDF 压缩。
- 同步/异步策略。

### M4：图片基础能力

- 图片格式转换。
- 图片压缩。
- 图片缩放。
- 批量处理。

### M5：音视频基础能力

- 视频转码。
- 视频压缩。
- 提取音频。
- 截帧生成封面。

### M6：生产化增强

- 限流。
- OpenTelemetry。
- Prometheus 指标。
- 管理后台或任务运维接口。
- 服务扩缩容。

## 20. 第一版推荐最小实现

第一版可以只实现以下闭环：

- Go gateway：`POST /files/presign`、`POST /jobs`、`GET /jobs/{id}`、`GET /download`。
- PostgreSQL：files、jobs、job_files。
- MinIO：上传原始文件，保存结果。
- NATS：异步任务队列。
- image-service：实现 `image.convert` 和 `image.resize`。
- media-service：实现 `video.snapshot`。
- pdf-service：实现 `pdf.merge`。

这样可以快速验证：

- 前端到网关是否顺畅。
- 网关到后端 RPC 是否可用。
- 异步任务状态是否完整。
- 文件上传和结果下载是否稳定。

## 21. 待决策问题

| 问题 | 推荐决策 |
| --- | --- |
| 网关 HTTP 框架 | chi |
| 内部 RPC | gRPC |
| 队列 | NATS JetStream |
| 开发对象存储 | MinIO |
| 数据库 | PostgreSQL |
| 图片服务语言 | Python 第一版，后续按性能迁移 Go |
| 音视频处理 | Python + FFmpeg |
| PDF 基础处理 | Go 或 Python，按库成熟度决定 |
| 服务发现 | 第一版静态配置 |

## 22. 风险与应对

| 风险 | 影响 | 应对 |
| --- | --- | --- |
| 大文件导致网关阻塞 | 网关性能下降 | 前端直传对象存储，网关只签名 |
| FFmpeg 任务耗时长 | Worker 堆积 | 队列削峰，媒体 worker 单独扩容 |
| 用户传入恶意文件 | 安全风险 | 文件魔数校验、沙箱、资源限制 |
| 处理服务崩溃 | 任务失败 | 重试、健康检查、服务隔离 |
| RPC 传输大文件 | 内存和网络压力 | RPC 只传 URI |
| 转换结果过大 | 存储成本上升 | 限制输出大小，设置生命周期清理 |

## 23. 推荐下一步

1. 初始化 Go workspace 和 gateway skeleton。
2. 定义 OpenAPI 与 protobuf。
3. 编写 PostgreSQL migration。
4. 搭建 Docker Compose：PostgreSQL、MinIO、NATS、gateway。
5. 先实现一个最小图片处理服务，跑通完整链路。
