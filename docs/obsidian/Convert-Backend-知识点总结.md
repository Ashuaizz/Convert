# Convert Backend 知识点总结

## 核心定位

Convert Backend 是一个面向 PDF、图片、音频、视频处理的后端微服务系统。前端只访问统一的 Go 网关，具体处理能力由后端独立服务提供，服务之间通过 RPC 协作。

## 总体架构

- 前端通过 HTTP REST 调用 Gateway。
- Gateway 使用 Go 编写，负责鉴权、文件管理、任务创建、状态查询、下载签名、RPC 调用和异步任务投递。
- PDF、图片、音视频处理服务独立部署，按任务类型拆分。
- 大文件不通过 RPC 传输，只传对象存储 URI。
- 异步任务通过队列削峰，Worker 执行实际处理。
- PostgreSQL 存储用户、文件、任务和结果元数据。
- MinIO 或 S3 存储上传文件和处理结果。

## 推荐技术选型

- 网关：Go + chi + grpc-go。
- RPC：gRPC + protobuf。
- 队列：NATS JetStream，备选 Redis Streams。
- 数据库：PostgreSQL。
- 对象存储：开发用 MinIO，生产用 S3/OSS/COS/MinIO 集群。
- PDF 服务：Go 或 Python。
- 图片服务：Python + Pillow/libvips，后续可按性能迁移 Go。
- 音视频服务：Python + FFmpeg。

## 关键设计原则

- 网关保持轻量，不直接做重计算。
- 处理服务可以用最适合该领域的语言实现。
- 同步接口只处理小文件和快速任务。
- 大文件、长耗时任务全部走异步任务。
- RPC 只传文件引用和参数，不传大二进制内容。
- 所有任务都要有状态、进度、错误码和可追踪日志。
- 文件访问必须校验用户权限。

## 典型业务流程

### 文件上传

1. 前端向 Gateway 请求 presigned upload URL。
2. Gateway 创建文件元数据。
3. 前端直传文件到对象存储。
4. 前端通知 Gateway 上传完成。
5. Gateway 标记文件为 uploaded。

### 异步处理

1. 前端创建处理任务。
2. Gateway 写入 jobs 表，状态为 queued。
3. Gateway 把任务发布到队列。
4. Worker 消费任务并更新为 running。
5. Worker 从对象存储读取输入文件。
6. Worker 调用处理逻辑生成结果。
7. Worker 写回对象存储并更新任务为 succeeded 或 failed。
8. 前端轮询或订阅任务状态。

## API 能力

- `POST /api/v1/files/presign`：创建上传链接。
- `POST /api/v1/files/{file_id}/complete`：确认上传完成。
- `POST /api/v1/jobs`：创建异步任务。
- `GET /api/v1/jobs/{job_id}`：查询任务状态。
- `GET /api/v1/jobs/{job_id}/results/{result_file_id}/download`：获取下载链接。
- `POST /api/v1/jobs/{job_id}/cancel`：取消任务。

## RPC 能力

内部处理服务建议统一实现 `ProcessorService`：

- `HealthCheck`：健康检查。
- `Validate`：校验任务参数。
- `ProcessSync`：同步处理。
- `StartJob`：启动异步任务。
- `CancelJob`：取消任务。

## 任务状态

- `queued`：等待执行。
- `running`：执行中。
- `succeeded`：成功。
- `failed`：失败。
- `retrying`：等待重试。
- `canceling`：取消中。
- `canceled`：已取消。

## 数据表重点

- `users`：用户信息。
- `files`：文件元数据和对象存储 URI。
- `jobs`：任务状态、进度、参数、错误信息、重试次数。
- `job_files`：任务和输入/输出文件关系。

## 安全重点

- 前端请求使用 JWT。
- 上传和下载使用短期 presigned URL。
- 校验文件真实类型，不信任扩展名。
- FFmpeg 参数必须由后端模板生成，禁止透传用户命令。
- 处理进程要设置超时、CPU、内存和磁盘限制。
- 用户只能访问自己的文件和任务。

## 可观测性

- 日志统一携带 `request_id`、`user_id`、`job_id`。
- Prometheus 指标覆盖 HTTP、RPC、任务、队列、存储操作。
- OpenTelemetry 透传 trace 信息。
- Worker 日志必须能定位具体 job。

## 第一版最小闭环

优先实现：

- Go Gateway。
- PostgreSQL migrations。
- MinIO 文件上传和下载。
- NATS 异步任务队列。
- 图片服务实现 `image.convert` 和 `image.resize`。
- 媒体服务实现 `video.snapshot`。
- PDF 服务实现 `pdf.merge`。

这个闭环可以验证前端到网关、网关到 RPC、异步任务、文件存储和结果下载是否跑通。

## 后续演进

- 用配置声明服务能力。
- 增加 OpenAPI 和 protobuf 合约测试。
- 加入限流、熔断、重试、服务健康检查。
- 引入 Prometheus、Grafana、OpenTelemetry。
- 迁移到 Kubernetes 后按队列长度或 CPU 自动扩缩容。

