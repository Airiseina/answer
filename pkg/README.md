根据你当前的工程结构（基于 Kitex微服务、Gin Api网关、Kafka MQ、MinIO存储、Qdrant向量数据库、MySQL数据持久化的分离架构），你已经具备了构建这个混合系统非常扎实的底层骨架。

为了实现 AIM 的所有考核目标，结合你已有代码的目录情况（如已有的 chat_service、user_service、qdrant、minio、kafka），我为你梳理了自下而上、分阶段打磨的详细实现步骤：

第一阶段：打通纯 IM 基础设施 (通讯底座开发)
这是整个 AIM 的地基，AI 能力要依附于稳定的消息流转之上。

完善连接层 (internal/connect)

- 实现长连接维持： 使用 WebSocket 实现一套 Session 管理器（维护 UserID -> Connection 或 ClientID -> Connection 的映射）。
- 心跳与状态： 客户端定时发 Ping，服务端回 Pong。同时维护用户的"在线/离线/输入中"状态并同步给相关好友。
- 单机/多机消息路由： 因为是分布式架构，用户 A 连接在节点 1，用户 B 连接在节点 2，需要使用网关+路由层机制。
消息发布与解耦 (internal/mq + worker)

- 利用你现成的 Kafka 模块。一条消息到来后，先存入数据库生成 MsgId，然后将消息投递到 Kafka 的 topic。
- worker/main.go 消费消息，判断目标用户所在的 connect 节点位置（通过 Redis 记录的状态），然后推给对应的长连接机器。
完成核心聊天 API (internal/kitex_service/chat_service)

- 补全单聊/群聊基础方法（发消息、拉取离线列表、漫游记录历史）。
- 实现消息读回执（记录对方 read_seq）、消息撤回（软删）。
- 数据流存储： 把现有 service.go 里的 CreateChat 细化，支持不同的内容载体（文本、MinIO 的图片/文件 URL、语音）。
第二阶段：用户与群组关系闭环
改造用户服务 (internal/kitex_service/user_service)

- 扩展 MySQL schema：增加friend、group、group_member等关系表。
- 加解好友与分组： 添加相关的 RPC 接口（包含添加请求的审批流）。
- 群组管理全家桶： 实现群创建、入群审批、群主转让、踢人、群公告更新等 API，权限校验要在接口层做严。
第三阶段：AI Bot 与核心计费接入 (通讯+AI)
机器人网关与消息拦截

- 在处理或消费消息的地方拦截带有 @Bot 或者是向专属 BotID 发送的消息。
- 异步处理AI回复： 利用 cmd/worker 独立出专门响应 AI 的协程或服务进程去消费该事件，不阻塞正常聊天，最后伪装成 Bot 用户走 IM 下发消息。
统一的大模型接口层 (internal/ai)

- 对接不同 LLM (如 OpenAI、阿里千问、或者用户自带 API Key)，提供统一的 Request/Response。
- 一键总结/待办提取功能： 在 chat_service 提供 API 抓取过去 N 条聊天内容（或某段时间区间），在此上面封装预设 Prompt 调用你的 internal/ai 模块，返回提炼结果。
平台化运营计费管理

- 若用户没有填自己的 Key，走平台的 Key 时：每次调用 internal/ai 层都对 Token 进行计数。
- 走 Kafka 发计费消协记录，最终在 MySQL users 的余额表里扣费。
第四阶段：RAG、MCP 与多机器人协作 (进阶架构)
利用你现在的 internal/service/document.go 和 qdrant。

构建私有知识库 Bot (RAG)

- 文件上传到 internal/storage/minio.go，触发解析任务。
- internal/ai/embedding.go 将文档（MD, PDF等文本块）切片并嵌入为 Vector。
- 将向量存进 internal/dal/qdrant 中。对话带有对应 Prompt 时，先去 Qdrant 检索 Top-K 原文丢给 LLM。
实现 MCP (Model Context Protocol) 插件系统

- 在 internal/ai 当中给通用的大模型加上 function_calling / tools。
- 注册天气 API、代码执行容器交互脚本、Web 搜索等。
长期记忆与多态身份管理

- 新建表维护Bot 角色库（不同的 System Prompt 和温度参数）。
- 针对个人长时期记忆，把用户跟机器人的对话事实经过萃取同样存入 Qdrant 向量库。下次聊天根据历史语义自动附加上下文。
第五阶段：服务治理与可观测性 (高可用目标)
在此必须体现你对高并发 IM 框架的架构能力。

服务治理 (Kitex Middlewares)

- 在 internal/kitex_service/ 的模块里实现 / 注册熔断（Circuit Breaker）和限流（Token Bucket）的中间件。
- 确保 connect 网关有重试机制保护下游核心。
Prometheus + Grafana 监控

- 在 Go 侧引入 prometheus 的 SDK，上报在线长连接数、收发 QPS、消息延迟延时。配置 Grafana 面板。
- 利用 OTel（OpenTelemetry）串联 HTTP Request -> Kitex rpc_user -> Kafka 的分布式链路追踪。
- API 增加鉴黄/暴库的审核接入（也可调轻量小模型审核）。
第六阶段：客户端开发与部署
打造炫酷的 CLI/TUI (命令行界面)

- 另外开一个工程，用 Go 的 bubbletea 等库实现聊天界面。利用 WebSocket Dial 连接到你的 网关层。
- 终端里实现 Markdown 表格和流式输出（Server-Sent Events 风格或通过 websocket 推分片）。
Docker 编排

- 完善你的 docker-compose.yml，把网关（API）、聊天 RPC、用户 RPC、Worker、MySQL 服务、Qdrant 服务、Minio、Kafka 容器全自动化拉起，一键部署演示。


