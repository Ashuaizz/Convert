# Convert Backend 核心概念说明

## 1. 对象存储

对象存储是专门保存文件的服务。Convert Backend 中的原始文件、中间文件、结果文件都统一放在对象存储里。

不要把大文件放在：

- Gateway 本地磁盘。
- Worker 本地磁盘长期保存。
- PostgreSQL 数据库字段中。

推荐做法：

```text
对象存储保存文件本体
数据库保存文件元数据和 storage_uri
```

示例：

```text
对象存储:
uploads/user_001/file_abc/demo.pdf

数据库:
file_id = file_abc
filename = demo.pdf
content_type = application/pdf
size = 1024000
storage_uri = s3://convert/uploads/user_001/file_abc/demo.pdf
status = uploaded
```

这样设计的好处：

- Gateway 不需要承载大文件传输压力。
- 数据库不会被大文件拖慢。
- 多个处理服务都能通过统一 URI 读取文件。
- 多机部署时不会出现文件只存在某一台机器上的问题。
- 可以生成短期上传和下载链接。

## 2. MinIO 与 S3

S3 是对象存储接口事实标准。AWS S3 是原始服务，很多云厂商和自建服务都兼容 S3 API。

MinIO 是一个可以本地部署、兼容 S3 API 的对象存储服务。

项目建议：

- 开发环境：MinIO。
- 生产环境：S3、OSS、COS、OBS 或 MinIO 集群。

在代码里尽量面向 S3 兼容接口开发，这样以后从 MinIO 切到云对象存储时改动较小。

## 3. Gateway

Gateway 是前端访问后端的统一入口。

职责：

- 接收 HTTP 请求。
- 鉴权。
- 创建上传和下载链接。
- 创建任务。
- 查询任务。
- 取消任务。
- 调用内部处理服务。
- 发布异步任务。
- 统一响应格式和错误码。

Gateway 不应该直接做 PDF、图片、音视频的重处理。

## 4. chi

`chi` 是 Go 里的轻量 HTTP 路由器。

它负责把前端请求路由到对应 handler，例如：

```text
POST /api/v1/jobs -> CreateJob
GET  /api/v1/jobs/{job_id} -> GetJob
```

它适合 Gateway 的原因：

- 接近 Go 标准库 `net/http`。
- 中间件模型简单。
- 路由分组清晰。
- 依赖轻。
- 后续替换或集成其他标准库组件比较自然。

## 5. chi 与 gin

| 对比 | chi | gin |
| --- | --- | --- |
| 定位 | 轻量路由器 | Web 框架 |
| 风格 | 标准库风格 | 框架风格 |
| Context | 使用 `context.Context` 和 `http.Request` | 使用 `gin.Context` |
| JSON 绑定 | 通常自己封装 | 内置较多便利方法 |
| 适合场景 | API Gateway、微服务入口 | 快速业务接口、后台 CRUD |

本项目推荐 chi，因为 Gateway 更像协议入口和任务编排层，不需要很重的 Web 框架。

## 6. grpc-go

`grpc-go` 是 Go 语言的 gRPC 实现。

在本项目中：

- chi 处理前端到 Gateway 的 HTTP 请求。
- grpc-go 处理 Gateway 到后端处理服务的 RPC 调用。

链路示例：

```text
Frontend -> HTTP -> Gateway -> gRPC -> image-service/pdf-service/media-service
```

RPC 中只传文件引用、任务类型、参数，不传大文件二进制。

## 7. 异步任务

文件处理任务通常耗时不稳定，尤其是视频、OCR、大 PDF。

所以 v0 必须支持异步任务：

```text
创建任务 -> queued
Worker 消费 -> running
处理成功 -> succeeded
处理失败 -> failed
```

前端通过轮询或后续 SSE/WebSocket 获取任务进度。

## 8. 队列

队列用于削峰和异步处理。

v0 推荐 NATS JetStream。

作用：

- Gateway 快速返回 job_id。
- Worker 按能力消费任务。
- 处理服务压力可控。
- 后续可以按任务类型拆分队列。

