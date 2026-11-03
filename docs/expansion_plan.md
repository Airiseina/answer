# Answer IM 扩展规划（评审稿 v4）

> 状态：待评审　|　作者：Codex　|　范围：需求 1~6 + Token 用量统计 + 扩展方向
> 说明：本文只描述设计与实施计划，不含代码改动。评审通过后再进入编码。
> v2 变更：依据第一轮评审答复，重写了开放平台、多租户、Bot 市场、多 Agent、日/周总结五节；新增计费需求；把「个人 Skill」与「性能与规模」提升为重点专项。
> v3 变更：依据第二轮评审答复，计费改为「统计用户自带 apikey 的 token 用量与预估成本」（非售卖服务）；平台管理员改为独立管理后台账号体系；组织租户入群本轮仅管理员直接添加；总结空档发占位消息；个人 Skill 限定为资料型；开放平台确认支持回调；并新增「十二、实施步骤清单」。所有设计均已对照源码核对。
> v4 变更：依据第三轮评审答复，参考单价由后台手工维护；用量看板仅本人可见；初始管理员通过配置文件创建；部门树纳入本轮；OpenAPI 采用文档给定的默认取值。所有未决问题清零，可进入编码。

---

## 一、背景与目标

`README.md` 的「后续方向」（第 1061 行起）列出五项待办，本规划把它们细化为可落地的设计，并追加评审阶段确认的两项新需求：

| 序号 | 需求 | 对应 README 待办 |
| --- | --- | --- |
| 1 | 支持用户通过 OpenAPI 自行部署机器人，并提供完整接口文档 | 新增（开放平台） |
| 2 | 增加类飞书的多租户能力 | 多租户 |
| 3 | 扩展 Bot 市场，支持用户上传 Bot，他人仅需 apikey + 模型即可使用 | 编写用户 bot 市场 |
| 4 | 实现多 Agent 协作（可视化编排画布） | 实现多 agent 协助 |
| 5 | 扩展系统 Agent 能力，增加日总结 / 周总结 | 扩展 agent 能力（…日和周总结） |
| 6 | 继续思考可扩展方向（重点：个人 Skill、性能与规模） | 增加用户个人 skill 等 |
| 7 | **新增**：统计用户自带 apikey 的 token 用量与预估成本（OpenAPI 接入的 Bot 不统计） | 新增（评审确认） |

目标：在**不破坏现有单租户使用方式**的前提下，逐步演进为「多租户 + 开放平台 + Bot 生态 + 多 Agent + 用量可统计」的平台形态；所有改动沿用现有分层（`api_gateway` 路由 → `rpc/*` → Kitex 服务 → `internal/{handler,service,dal,model}`）、现有 IDL 生成链路与 GORM `AutoMigrate` 建表方式。

---

## 二、现状分析

### 2.1 现有架构（已核对源码）

- 网关：`api_gateway`（Hertz，:1234，`cmd/main.go`）、`msg_gateway`（WebSocket，:8081，暴露 `/ws`、`/push`）。
- Kitex 微服务：`user_service` :4320、`group_service` :4321、`chat_service` :4322、`bot_service` :4323、`work_service` :4324、`knowledge_service` :4326。
- 基础设施：MySQL（元数据，GORM `AutoMigrate`）、PostgreSQL（消息 + JSONB）、ClickHouse（冷消息）、Qdrant（向量）、Neo4j（知识图谱）、Meilisearch（BM25）、Kafka、Garnet/Redis、SeaweedFS、Traefik、OTEL/Prometheus/Grafana/Jaeger。
- 前端：`web/test_frontend`（React 19 + TS + Vite），`BotsPanel.tsx` 单文件承载「我的 Bot / 创建 Bot / MCP 工具 / 知识库」四个 Tab；依赖仅 `react`、`react-dom`、`react-markdown`、`remark-gfm`。

### 2.2 关键代码事实

**鉴权与路由**

- `api_gateway/middleware/middleware.go` 用 hertz-jwt，登录态结构体为 `Resp{Id, Account}`，claims 仅含 `id`/`account`，`IdentityKey = "user"`；受保护路由统一挂在 `h.Group("/api", ...)`。
- `api_gateway/routes/routes.go` 汇总全部路由；`/register`、`/login`、`/files/*filepath` 免鉴权。
- DTO 风格：`api_gateway/handle/bot.go` 结构体带 `json` + `vd:` 校验标签、`c.BindAndValidate`，统一走 `response.Success/Error`；`int64` 出参用 `fmt.Sprintf("%d", id)` 转字符串。
- 统一响应：`api_gateway/response/response.go`，`{code, msg, data}`，`SUCCESS=0 / ERROR=1`。

**Bot 与 Agent**

- `kitex_service/bot_service/internal/model/bot.go`：`bot_table` 字段 `ID/UserID/CreatorID/SystemPrompt/ApiKey/Model/BaseURL/IsSystem`——**凭证与定义耦合**，这是需求 3 与需求 7 要拆分的点。
- `bot_service/internal/service/service.go`：`CreateBot` 内含 prompt 安全审核（`filterUserPrompt` / `sanitizeUserPrompt` / `stripSafetyPrompt`，见 60~135 行）；`InitSystemBot` / `syncSystemBotFromFiles` / `initSkillKnowledgeBase`（350~600 行）负责系统 Bot 初始化，其中 Skill 走「创建知识库 + 上传 markdown 文件」。
- `work_service/internal/agent/react.go`：Eino `react.NewAgent`，`maxReActSteps=12`，内置 `search_knowledge` 工具 + MCP 工具。
- `work_service/internal/service/service.go`：`HandleMessage`（224 行）→ `handleWithAgent`（299 行）；`buildSafeSystemPrompt`（68 行）把安全提示词追加在末尾；超时 `llmTimeout=120s`、`agentTimeout=180s`。

**消息链路**

- `msg_gateway/core/hub.go` 的 `triggerBots`（895 行）写 `bot-task-topic`；`work_service` 的 `BotTaskConsumer` 按 Bot 分队列消费（`maxConcurrentBots=10`）；回复后写 `bot-reply-topic`；`msg_gateway` 的 `StartBotReplyConsumer`（965 行）再推 WS。

**总结能力**

- `work_service` 现有 `SummarizeConversation`（581 行）/ `SuggestReplies` / `TranslateMessage` 均按「单个会话」维度、走 `getSystemBotLLMConfig`（533 行，取系统 Bot 的 apikey/model）；**没有**按用户跨会话、按时间窗的聚合，也没有定时调度。

**系统 Bot 可作为投递方**

- `InitSystemBot` 调 `rpc.CreateBotUser(ctx, name, avatarURL)` 为系统 Bot 建立真实用户记录并回写 `bot_table.user_id`。
- `chat.thrift` 有 `GetOrCreatePrivateConversation(user_id_a, user_id_b)`；`bot_service` 的 `AddBotToConversation` 私聊分支正是用它建立「用户 ↔ Bot」的私聊（service.go 334 行）。→ 总结由系统 Bot 以私聊消息投递在现有链路上可行。

**用量统计可用的埋点**

- `pkg/observability/meter/meter.go` 已定义 `BotTokenUsage`（`aim.bot.token.usage`）与 `BotRequestTotal` / `BotResponseLatency`，但 `BotTokenUsage` **全库无调用点**（`rg` 零命中），属于预留未接线。
- `work_service/internal/llm/client.go` 的 `Chat` 只返回 `resp.Content`；eino `schema.Message.ResponseMeta.Usage` 已含 `PromptTokens / CompletionTokens / TotalTokens`，但当前未采集。

**知识库与多端**

- `knowledge_service/internal/model/knowledge_base.go` 以 `OwnerID` 归属；`bot_knowledge` 做 bot↔kb 绑定。→ 市场分发边界（不分发 KB）据此设计。
- `msg_gateway/core/hub.go` 538 行留有 Phase 7 TODO：`Clients` 目前是 `map[int64]*Client`，多端需改 `map[int64][]*Client`。
- README 522~541 行已描述消息投递的写/读扩散（成员 ≤100 写扩散，>100 读扩散）。

**缺失项**

- **无租户概念**：全库检索 `tenant|租户|workspace|组织` 零命中，当前是完全扁平的「单租户」模型。
- **无平台管理员体系**：`user_service/internal/model/user.go` 只有 `Account/Name/Hash/AvatarURL/IsBot`，没有 `is_admin`/`role` 字段。→ 市场人工审核需要先引入管理员能力；评审确认采用**独立的管理后台账号体系**（不复用普通用户表，见 3.2）。

### 2.3 现状与目标的差距

1. 归属维度只有 `user_id/creator_id/owner_id`，缺少租户隔离层 → 需求 2 是其余多项功能的地基。
2. Bot「定义」与「凭证」未拆分，没有版本、发布、审核、安装概念 → 需求 1、3、7 都受此约束。
3. 对外只有 JWT 用户态，没有面向第三方的 AppID/AppSecret 鉴权与事件投递 → 需求 1。
4. Agent 是单体 ReAct，没有角色/编排概念，前端也没有画布 → 需求 4。
5. 系统 Agent 无定时任务与跨会话聚合查询能力 → 需求 5。
6. LLM 调用不采集 token usage，无价格表/账本 → 需求 7。

---

## 三、总体设计原则与公共改造

### 3.1 设计原则

1. **地基先行**：先落「租户上下文」与「Bot 定义/实例拆分」两个公共模型，再叠加 1/3/4/5/7。
2. **向后兼容**：Thrift 一律「新增 optional 字段 / 新增方法」，不改既有字段语义；DDL 用「可空列 + 默认值」增量迁移。
3. **沿用既有模式**：新服务/新表/新路由的命名、分层、错误串风格、ID 生成、日志与 metrics 全部对齐现有代码，不引入第二套风格。
4. **隔离可验证**：租户隔离通过「请求上下文 → RPC 透传 → DAO 强制过滤」三层保证，并补一组越权用例作为验收。
5. **渐进式拆分**：优先在既有服务内扩展（复用 DAO/IDL），流量或职责膨胀后再拆独立服务。

### 3.2 公共改造（所有需求共享）

- **租户上下文包**：新增 `pkg/tenant`，提供 `WithTenant(ctx, id)` / `FromContext(ctx)` / `MustFromContext`，以及 HTTP 头常量 `X-Tenant-Id`。
- **独立管理后台账号体系**（评审确认）：不复用普通用户表，新增独立的 `admin_service` 与 `admin_user` 表（`id/account/hash/name/role/status/last_login_at`），独立的登录与 JWT（claims 带 `admin_role`），`api_gateway` 新增独立路由组 `/api/admin/*` 与 `middleware.RequireAdmin`。市场审核、参考单价维护、租户治理都走这套后台账号。**初始管理员通过配置文件创建**（`config` 中 `admin.bootstrap` 列表，含 `account`/`name`/`role` 与初始口令或口令哈希，服务启动时幂等写入）。
- **ID 生成约定**：沿用 `pkg/snowflake`。既有分片 user=1、group=2、chat msg=3、chat conv=4、bot=5、knowledge=6；新增服务续 7 起：`open=7`、`billing=8`、`admin=9`。若市场/总结/编排暂不拆服务，则复用宿主服务的节点（market→bot=5，summary/agent→work 新分配），避免 ID 跨服务冲突。
- **审计日志**：新增 `pkg/audit` + 各服务 `audit_log` 表，先记录「开放平台调用、Bot 发布/审核/安装、租户管理、参考单价维护、总结生成」五类事件。
- **指标**：沿用 `pkg/observability/meter`，新增 `open_api_request_total`、`market_install_total`、`summary_generate_total`、`agent_graph_run_total`、`billing_cost_total`，并把已存在的 `BotTokenUsage` 接线。

---

## 四、需求 1：开放平台（AppID + AppSecret 自部署机器人 + 完整文档）

### 4.1 目标形态

第三方（企业/开发者）无需 JWT，仅凭一对 **AppID + AppSecret**，即可通过 REST 完成：创建 Bot → 配置（提示词/模型/工具）→ 收发消息 → （可选）接收事件回调。同时交付 `openapi.yaml` + 可浏览文档站点 + 多语言接入示例。

评审已确认：**鉴权只用 AppID + AppSecret，不引入 OAuth2**（OAuth2 授权第三方代表用户操作列为后续可选，不在本轮）。

### 4.2 鉴权设计

**新增独立服务 `open_service`（:4327，snowflake=7）**，与 `api_gateway` 的 `/open/v1/*` 路由组配套，避免把第三方入口混进用户态网关逻辑。

**新增中间件 `api_gateway/middleware/open_auth.go`**：

- 请求头：`X-App-Id`、`X-Timestamp`、`X-Nonce`、`X-Signature`。
- 签名：`HMAC-SHA256`，待签串 `stringToSign` 为 6 段以 `\n` 连接：
  `appID \n timestamp \n nonce \n METHOD \n path \n sha256Hex(body)`，
  其中 `path` **不含查询串**；`body` 为空时 `sha256Hex("")`。做法沿用
  `msg_gateway/core/hub.go` 里 `signPushRequest` / `verifyPushSignature` 同款写法
  （`crypto/hmac` + `sha256` + `hex`）。
- 防重放：`timestamp` 窗口（±300s）+ `nonce` 在 Garnet 去重（`SetNX` + TTL=窗口）。
- 作用域：AppID 绑定 `scopes`，中间件按路由所需 scope 校验。

**已落地作用域**：`bot:read`、`bot:write`、`message:send`、`message:read`、
`conversation:read`、`webhook:manage`。
`mcp:manage`、`knowledge:manage` 属后续迭代（本轮开放平台不分发 MCP / 知识库）。

### 4.3 事件获取：拉取与回调（评审确认：支持回调）

评审确认「开放平台的用户 Bot 支持回调」。这里先把两种取事件方式讲清楚，**回调与轮询同时提供**：

- **没有回调（拉取 / polling）**：第三方若要知道"用户给你的 Bot 发了消息""Bot 需要你生成回复"，只能自己定时轮询 `GET /open/v1/events`（或 `GET /open/v1/messages`）。缺点：有延迟（取决于轮询间隔），且大量请求是空转，浪费 QPS。
- **有回调（推送 / webhook）**：平台在事件发生时（例如用户在会话里 @ 了你的 Bot、或有人给 Bot 发私聊），主动用 HTTPS POST 把事件推到你的服务器；你处理完再调 `POST /open/v1/messages/send` 回复。优点：近实时、无需轮询、省流控配额。

因此本设计：

1. **拉取是基线能力**，默认可用，不要求第三方有公网地址，也不要求配置回调。
2. **回调（Webhook）支持**：第三方提供一个公网 HTTPS 地址并注册，平台用 `webhook.secret` 对推送体做 `HMAC-SHA256` 签名（头 `X-Signature` / `X-Timestamp` / `X-Nonce`），第三方验签后再处理；平台带指数退避重试与死信记录（`open_webhook_delivery` 表）。
3. 支持 `POST /open/v1/events/ack` 显式确认，未确认按重试策略重投，避免漏事件。

> 一句话：**配了回调就不用轮询，能第一时间收到消息；没配回调也能用，靠 `/open/v1/events` 轮询。**

### 4.4 对外接口（`/open/v1`，均需签名）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/open/v1/bots` | 创建 Bot（等价现有 `CreateBot`，带租户与 app 归属） |
| GET | `/open/v1/bots` | 列出名下 Bot |
| GET | `/open/v1/bots/{bot_id}` | Bot 详情 |
| PATCH | `/open/v1/bots/{bot_id}` | 更新（提示词/模型/base_url） |
| DELETE | `/open/v1/bots/{bot_id}` | 删除 |
| POST | `/open/v1/bots/{bot_id}/conversations` | 拉 Bot 进入会话（私聊/群聊） |
| GET/POST/PATCH/DELETE | `/open/v1/bots/{bot_id}/mcp` | MCP Server 增删改查（**后续迭代**，本轮不分发 MCP） |
| POST/DELETE | `/open/v1/bots/{bot_id}/knowledge` | 知识库绑定/解绑（**后续迭代**，本轮不分发 KB） |
| POST | `/open/v1/messages/send` | 以 Bot 身份发送消息 |
| GET | `/open/v1/messages` | 拉取会话历史（分页） |
| GET | `/open/v1/conversations` | 会话列表 |
| GET | `/open/v1/events` | **事件拉取（轮询基线）**，游标分页 |
| POST | `/open/v1/webhooks` | 注册回调（可选），注册时即时校验地址 |
| GET | `/open/v1/webhooks` | 回调列表（可按 `bot_id` 过滤） |
| GET | `/open/v1/webhooks/{id}` | 回调详情（含验签密钥） |
| DELETE | `/open/v1/webhooks/{id}` | 删除回调 |
| POST | `/open/v1/events/ack` | 回调事件确认 |

> 回调地址校验在**注册时即时执行**：`RegisterWebhook` 会向目标地址发起一次携带
> `verify_challenge` 的签名 POST，地址原样回显即视为校验通过，因此未单独暴露
> `/webhooks/{id}/verify` 路由。

**事件投递**：Bot 回复产出后 `work_service` 已写 `bot-reply-topic`；新增 `open_dispatch` 消费者（放 `open_service`）读取该主题：有 webhook 的 App 走签名推送，无 webhook 的 App 写入事件表供 `/open/v1/events` 轮询拉取。两种方式共用同一事件表，保证不丢。

### 4.5 新增数据表（GORM，`open_service`）

```go
// open_app 开放平台应用（一个租户/用户可创建多个）
type OpenApp struct {
    ID            int64
    TenantID      int64     // 归属租户（0=个人）
    OwnerID       int64     // 创建者
    Name          string
    AppID         string    `gorm:"type:varchar(64);uniqueIndex;not null"`
    AppSecretHash string    `gorm:"type:varchar(128);not null"` // SHA-256 摘要，快速失效判定
    AppSecretCipher string  `gorm:"type:varchar(256);not null"` // AES-256-GCM 密文，验签时还原明文
    Scopes        string    `gorm:"type:varchar(512);not null"` // 逗号分隔
    Status        int16     // 0 启用 / 1 停用
    QPSLimit      int32
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// open_webhook Bot 回调地址（可选）
type OpenWebhook struct {
    ID        int64
    AppID     string
    BotID     int64
    URL       string
    Secret    string
    Events    string // 逗号分隔，如 message.received, message.reply
    Enabled   bool
    CreatedAt time.Time
}

// open_event 事件（轮询与回调共用的源头）
type OpenEvent struct {
    ID        int64
    AppID     string    `gorm:"index"`
    BotID     int64
    EventType string
    Payload   string    `gorm:"type:text"`
    Status    int16     // 0 待投递 1 已投递 2 已确认 3 死信
    CreatedAt time.Time
}
```

> AppSecret 需要参与 HMAC 验签，而哈希不可逆，因此库里同时保存
> `AppSecretHash`（SHA-256，用于快速判定）与 `AppSecretCipher`（AES-256-GCM 密文，
> 验签时用配置口令派生的密钥解密还原）。明文只在创建/轮换响应中返回一次。

### 4.6 接口文档（重点交付物）

1. **机器可读规范**：`docs/openapi/open_platform.yaml`（OpenAPI 3.1），覆盖所有 `/open/v1` 路由、请求/响应 Schema、错误码、签名说明、事件类型。
2. **可浏览文档**：用 Redoc/Swagger UI 静态页挂到 `/open/docs`，直接渲染上面的 YAML。
3. **人工文档**：新增 `docs/open_platform.md`，延续 `docs/api_doc.md` 的章节风格（通用响应格式 → 认证机制 → 分组接口 → 附录），并补：签名算法示例、错误码表（对齐 `response` 的 `code/msg/data`）、限流与配额、幂等（`X-Request-Id`）、事件与验签、快速接入（curl + Go/TS/Python 示例）。
4. **README 索引**：在 `README.md` 的「API 文档」小节增加开放平台入口链接。

### 4.7 迁移与兼容

- 现有 `/api/bot/*` 保持不变；开放平台只是「另一扇门」，内部复用 `bot_service`、`chat_service`、`knowledge_service` 的既有 RPC。
- 开放平台创建的 Bot 记为 `bot_table.origin = openapi`（见第九节），**不计入用量统计**。

### 4.8 默认取值（评审确认：使用默认值）

| 项 | 默认值 | 说明 |
| --- | --- | --- |
| 单 AppID QPS | **20 次/秒** | 超出返回 `code=1, msg=请求过于频繁`（HTTP 200 统一封装），写审计 |
| 时间戳窗口 | **±300 秒** | `X-Timestamp` 偏差超出即拒绝 |
| nonce 去重 TTL | **300 秒** | Garnet `SetNX` |
| 回调重试 | **最多 5 次**，指数退避 | 第 1 次即时，之后 1s/5s/30s/120s |
| 投递超时 | **10 秒/次** | 连接与读取合计 |
| 死信 | 重试耗尽后置 `open_webhook_delivery.status=死信` | 后台可查、可手动重投 |
| 事件保留 | **30 天** | 过期清理 |
| 单 App 创建 Bot 上限 | **50 个** | 需调整走管理后台 |

以上均落在配置项中，可按需覆盖，文档需同步写明。

---

## 五、需求 2：多租户（飞书式）

### 5.1 目标形态（评审确认：按飞书式部署多租户）

- 一个「租户（企业/组织）」内拥有独立的成员、部门、群组、会话、Bot、知识库与配额。
- 一个用户可以属于**多个**租户（飞书式）：登录后选择/切换租户，令牌绑定当前租户。
- 租户管理员可管理成员、部门、租户级设置（允许的模型供应商、配额等）。

### 5.2 双形态租户：个人租户 + 组织租户（评审确认：存量按用户各自拆成独立租户）

飞书式（用户可属多租户）与「按用户各自拆分」并不冲突，方案是**两种租户并存**：

- **个人租户**（`is_personal = 1`）：每个用户注册时自动创建一个属于自己的租户，用户是 owner。存量数据的迁移目标就是各自的个人租户；新用户同样自动获得个人租户。
- **组织租户**（`is_personal = 0`）：用户主动创建或加入的企业/团队租户，成员关系为多对多。

这样既满足「老数据天然分到各用户名下」，又满足「用户还能再加入组织租户」的飞书式体验。

### 5.3 存量数据迁移（评审确认：按用户各自拆成独立租户）

1. 迁移脚本为每个已存在的 `user` 生成一个个人租户，`tenant.owner_id = user.id`，`tenant_member(user, tenant, role=owner)`。
2. 按归属列回填：`user_id`/`creator_id`/`owner_id` 指向的用户 → 该用户的个人租户。逐表处理（`bot_table`、`knowledge_base`、`friend*`、`group*`、`conversation*`、`message_table`、`inbox_message` 等）。
3. 系统 Bot（`creator_id=0`）归入一个固定的「平台租户」，全局可见。
4. 回填后校验：`tenant_id=0` 的记录数应为 0（构建期断言/迁移报告）。

> 与 v1 的差异：v1 是「统一默认租户」，v2 改为「一人一租户」，隔离更彻底，用户之间天然互不可见。

### 5.4 数据模型（新增）

```go
// tenant 租户
type Tenant struct {
    ID         int64
    Name       string
    Code       string `gorm:"type:varchar(32);uniqueIndex;not null"` // 租户标识（本轮仅展示用，入群走管理员添加）
    LogoURL    string
    OwnerID    int64
    IsPersonal bool   `gorm:"default:false;index"` // true=个人租户
    Status     int16  // 0 正常 / 1 停用
    Plan       string // free/pro...
    Settings   string `gorm:"type:json"` // 允许供应商、配额、市场开关
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// tenant_member 成员（用户↔租户多对多）
type TenantMember struct {
    TenantID int64 `gorm:"primaryKey"`
    UserID   int64 `gorm:"primaryKey"`
    Role     int16 // 2=owner 1=admin 0=member
    DeptID   int64
    Status   int16 // 0 正常 / 1 待接受 / 2 已移除
    JoinedAt time.Time
}

// tenant_department 部门树
type TenantDepartment struct {
    ID       int64
    TenantID int64
    ParentID int64
    Name     string
    Order    int32
}
```

### 5.5 隔离维度贯穿

**给既有表增列 `tenant_id BIGINT NOT NULL DEFAULT 0`（并建索引）**，覆盖：`user`、`friend`、`friend_group`、`friend_request`、`group`、`group_member`、`conversation`、`conversation_member`、`message_table`、`inbox_message`、`message_edit_history`、`bot_table`、`bot_mcp_server`、`knowledge_base`、`document`、`bot_knowledge`，以及后续新增的 open/market/summary/billing/user_skill 表。

**取值策略**：不把 tenant 编进全局 ID，而让每条记录显式带 `tenant_id`；唯一约束按「租户内唯一」调整。`account` 保持全局唯一以降低改动面（`user_service` 的 `Account` 已 `uniqueIndex`），其余业务键（群号、Bot 名等）改为 `(tenant_id, key)` 复合唯一。

各存储的隔离方式：

| 存储 | 隔离方式 |
| --- | --- |
| MySQL | 所有 DAO 查询强制 `WHERE tenant_id = ?`；新增中央辅助（`dal` 层注入租户过滤） |
| PostgreSQL | `message_table` / `inbox_message` 增列 `tenant_id` 并进索引前缀 |
| Qdrant | collection 内 payload 增 `tenant_id`，检索时 filter 过滤 |
| Meilisearch | 文档增 `tenant_id`，查询加 filter |
| Neo4j | 节点/关系属性增 `tenant_id`，Cypher 带条件 |
| ClickHouse | 冷表增 `tenant_id` 列，查询条件过滤 |
| Garnet/Redis | 会话缓存 key 前缀补租户段（在线状态可保持全局） |
| Kafka | `BotTask` 等消息体新增 `tenant_id`，消费侧按需隔离 |

### 5.6 令牌与上下文

- JWT claims 增加 `tenant_id`；`middleware.Resp` 扩展为 `{Id, Account, TenantId}`；`IdentityHandler` 解析。
- 登录返回租户列表与「当前租户」；新增 `POST /api/tenant/switch` 重新签发绑定目标租户的令牌。
- `HandleMessage` 等入口从 `ctx` 取 `tenant_id`，透传到 RPC（各 IDL 请求体新增 `optional i64 tenant_id`）。

### 5.7 接口

`POST /api/tenant/create`、`GET /api/tenant/list`、`POST /api/tenant/switch`、`GET /api/tenant/info`、`POST /api/tenant/update`、`GET /api/tenant/members`、`POST /api/tenant/member/add`（**本轮入群仅支持管理员直接添加**，按账号/用户 ID 添加）、`POST /api/tenant/member/set_role`、`POST /api/tenant/member/remove`。**部门树本轮一起做**：`GET/POST /api/tenant/department/create`、`POST /api/tenant/department/rename`、`POST /api/tenant/department/move`、`POST /api/tenant/department/remove`、`POST /api/tenant/department/set_member`（把成员归入部门）。租户管理路由需校验 `TenantMember.Role >= admin`（新增 `middleware.RequireTenantAdmin`）；跨租户的创建/停用等治理动作走独立管理后台（`/api/admin/tenant/*`）。

> 评审确认：本轮不做邀请码/邀请链接入群，`tenant.Code` 仅作展示与后续扩展保留。
> 评审确认：**部门树（`tenant_department`）纳入本轮**，包含部门增删改移与成员归属；更细的组织架构权限（部门级管理范围）留后续。

### 5.8 风险

- **侵入面最大**：一旦某处 DAO 漏加 `tenant_id` 过滤即造成越权/串租。缓解：集中式租户过滤辅助、DAO 单测覆盖、验收用例专门测「A 租户访问 B 租户资源应 403/空」，必要时后续引入 DB 行级安全（RLS）。
- IDL 全量变更需重新生成 `kitex_gen`，要一次性对齐避免半迁移状态。

---

## 六、需求 3：Bot 市场（全局可见 + 人工审核 + 一键安装）

### 6.1 目标形态（评审确认）

- 创作者发布**不绑定凭证的 Bot 定义**（名称/头像/描述/提示词/推荐模型）。
- 使用者「仅提供 apikey + 模型」即可一键安装为**自己的 Bot 实例**，在自己的会话里使用。
- **市场全局可见**（不区分租户私有市场）。
- **需要人工审核**后才能上架；审核由平台管理员执行。

### 6.2 分发边界（评审确认：关键约束）

- **不分发知识库**：创作者自定义上传的知识库**不随 Bot 分发**。安装者如需知识库，必须自建或另行授权。
- **不分发 MCP**：MCP Server 配置（尤其是 token）**不随 Bot 分发**。安装者需要时自行在「MCP 工具」里配置自己的 MCP。
- 因此市场发布物只包含：**名称、头像、描述、分类、标签、系统提示词、推荐模型**。安装者提供自己的 `api_key`（+ 可选 `base_url`）与 `model`。

> 与 v1 的差异：v1 设计了 `McpTemplates` / `KbTemplate`「模板分发」，v2 **全部删除**，彻底避免创作者密钥/知识资产外泄。

### 6.3 关键拆分

- **定义（发布物）**：可共享、可版本化、**不含任何密钥与知识资产**。
- **实例（安装物）**：落在 `bot_table`，`CreatorID = 安装者`，`SystemPrompt` 取自定义，`ApiKey/Model/BaseURL` 来自安装者，`origin = market`。

### 6.4 新增模型（复用 `bot_service` 分层）

```go
// market_bot 市场中的 Bot 定义（全局可见，不含凭证/知识库/MCP）
type MarketBot struct {
    ID               int64
    PublisherID      int64
    Name             string
    Description      string
    AvatarURL        string
    Category         string
    Tags             string // 逗号分隔
    SystemPrompt     string // 发表前经 filterUserPrompt 审核
    RecommendedModel string
    Version          int32
    Status           int16  // 0 草稿 1 待审 2 已发布 3 驳回 4 下架
    InstallCount     int32
    RatingSum        int32
    RatingCount      int32
    ReviewNote       string // 审核意见
    ReviewedBy       int64
    ReviewedAt       time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

// market_bot_install 安装记录（定义 → 实例）
type MarketBotInstall struct {
    ID          int64
    MarketBotID int64
    InstallerID int64
    TenantID    int64
    BotID       int64 // 生成的 bot_table.id
    Version     int32
    CreatedAt   time.Time
}
```

### 6.5 审核流程（评审确认：人工审核）

状态机：`草稿 → 待审 → 已发布 / 驳回 → 下架`。

- 提交发布时复用 `bot_service` 的 `filterUserPrompt`/`sanitizeUserPrompt` 做自动预审（拒绝明显违规）。
- 预审通过进入「待审」，由**平台管理员**（独立管理后台账号，见 3.2）在管理后台人工审核：通过则 `Status=已发布`，驳回则填 `ReviewNote`。
- 每次上架/驳回/下架写审计日志（`audit_log`），记录审核人。
- 反滥用：限制单用户发布数量；记录发布者，支持下架与封禁。
- 版本与更新：`market_bot.version` 自增；若修改提示词需重新审核。安装记录保存 `Version`，可提示「有新版本」。

### 6.6 接口

`GET /api/market/bots`（分类/标签/关键词/排序）、`GET /api/market/bots/{id}`、`POST /api/market/bots`（发布，进入待审）、`POST /api/market/bots/{id}/version`（更新版本，重新待审）、`POST /api/market/bots/{id}/offline`、`POST /api/market/bots/{id}/install`（body：`api_key/model/base_url/name?` → 返回 `bot_id`）、`POST /api/market/bots/{id}/rate`、`GET /api/market/my-published`、`GET /api/market/my-installed`。

管理员审核接口（挂独立管理后台 `/api/admin/*`，用后台账号鉴权）：`GET /api/admin/market/reviews`、`POST /api/admin/market/reviews/{id}/approve`、`POST /api/admin/market/reviews/{id}/reject`。

### 6.7 前端

`BotsPanel.tsx` 现有四个 Tab 上新增 `market`（浏览/安装）与 `publish`（发布/我的发布）；管理后台新增「市场审核」列表页。沿用其 `api()` 调用与现有卡片/表单样式。

### 6.8 服务归属取舍

**推荐**：先在 `bot_service` 内新增 market 相关 model / DAO / RPC / handler（复用提示词审核与 `bot_table` 写入，改动集中）。若市场流量与审核逻辑膨胀，再拆 `market_service`。

### 6.9 与用量统计的关系

市场安装出来的 Bot 属于「用户自定义配置 Bot」，**纳入用量统计**（见第九节，`origin = market`）。

---

## 七、需求 4：多 Agent 协作（可视化编排画布）

### 7.1 目标形态（评审确认：使用可视化编排画布）

不再是「后台表单填团队成员」，而是提供一个**可视化画布**：用户拖拽节点、连线，定义多 Agent 的协作拓扑；保存后由 `work_service` 编译成 Eino 图执行。单 Agent ReAct（`internal/agent/react.go`）作为图中的节点执行器继续复用。

### 7.2 画布交互

- **前端技术**：引入 React Flow（`@xyflow/react`，当前 `package.json` 没有图编辑依赖，需新增）。
- **节点类型**：
  - `start` 触发节点（用户消息 / @ 触发）；
  - `agent` 专家节点（配 name、system prompt、model、可用工具/KB）；
  - `knowledge` 检索节点（调 `SearchKnowledge`）；
  - `tool` 工具节点（MCP 工具）；
  - `condition` 分支节点；
  - `parallel`/`join` 并发与汇聚；
  - `end` 输出节点。
- **连线**：表示数据流/执行依赖，画布自动做连线的合法性提示。
- **预设模板**：提供「主管 + 专家」预设（一键生成 supervisor 拓扑），降低上手成本；保留 v1 的 Supervisor 思路作为模板之一。

### 7.3 图模型与存储

```go
// bot_agent_graph Bot 的协作编排（一份当前版本 + 历史）
type BotAgentGraph struct {
    ID        int64
    BotID     int64
    Version   int32
    GraphJSON string `gorm:"type:longtext"` // {nodes:[{id,type,data}], edges:[{source,target}]}
    Status    int16  // 0 草稿 1 已启用 2 已停用
    CreatedAt time.Time
    UpdatedAt time.Time
}

// bot_agent_graph_history 历史版本，便于回滚
type BotAgentGraphHistory struct {
    ID        int64
    BotID     int64
    Version   int32
    GraphJSON string `gorm:"type:longtext"`
    CreatedAt time.Time
}
```

在 `bot.thrift` 的 `BotInfo` 增 `optional bool is_team`、`optional i32 graph_version`；新增 `SaveAgentGraph` / `GetAgentGraph` / `ValidateAgentGraph` / `PublishAgentGraph` 方法。

### 7.4 执行引擎（Eino Graph）

- `work_service` 新增 `internal/agent/graph.go`：读取 `GraphJSON` → 用 Eino `compose.NewGraph` 构建执行图（`agent` 节点→Agent/Lambda 节点，`condition`→Branch，`parallel`/`join`→并发节点，`knowledge`→检索 Lambda，`tool`→工具节点）。
- `HandleMessage` 判断 Bot 是否有已启用的图：有则走图执行，无则走现有单 Agent（`handleWithAgent`）。
- 子 Agent 复用 `mcp.Pool` 与 `search_knowledge` 工具，但按节点配置**限定工具/知识库范围**。

### 7.5 校验、限额与降级

- **保存校验**（`ValidateAgentGraph`）：必须有 `start`/`end`；默认必须是 DAG（无环，循环需显式开启且受限）；节点数/边数上限；每节点工具白名单；prompt 过 `filterUserPrompt`。
- **运行限额**：团队总步数、总 token、总耗时上限（延续 `agentTimeout` 思路，团队可放宽到 300s 并单独配置）；子 Agent 并发上限复用 `maxConcurrentBots` 信号量思路。
- **降级**：超预算/超时退回单 Agent 或返回已完成部分，并提示。
- **可观测**：复用 `BuildAgentCallback`，新增 `agent_graph_run_total`、`agent_team_step_total`、`agent_team_latency`，每步归属到具体节点。

### 7.6 接口与前端

- 接口：`GET/POST /api/bot/{bot_id}/graph`（读/存草稿）、`POST /api/bot/{bot_id}/graph/validate`、`POST /api/bot/{bot_id}/graph/publish`、`POST /api/bot/{bot_id}/graph/rollback`。
- 前端：`BotsPanel` 新增「协作编排」页签，内嵌 React Flow 画布 + 节点配置侧栏 + 校验/发布按钮。

### 7.7 风险

画布表达力强但复杂度高：图编译/执行与 Eino API 的适配是主要工作量；成本与延迟可能成倍上升，必须有硬性限额与降级。

---

## 八、需求 5：系统 Agent 扩展（日总结 / 周总结）

### 8.1 目标形态（评审确认）

- **范围**：总结**用户全部会话**（跨会话聚合），不是单会话。
- **投递**：**由系统 Bot 发送**（以系统 Bot 身份发消息给用户）。
- **触发时间**：日总结每天 **21:00**；周总结每周日 **21:00**（时区固定 `Asia/Shanghai`）。

### 8.2 触发与调度

- 在 `work_service` 内新增 `internal/scheduler`（引入 `robfig/cron` 或轻量 ticker），cron 表达式：日 `0 21 * * *`、周 `0 21 * * 0`（`CRON_TZ=Asia/Shanghai`）。
- 多实例部署时用 Garnet 分布式锁保证单实例执行；**21:00 洪峰**按 `user_id` 取模分桶、在 21:00~21:30 内抖动错峰（分片细节与第十节的性能专项呼应）。
- 规模化后改造为「调度器投 Kafka → 消费者生成」。

### 8.3 数据来源

- `chat.thrift` 新增 `GetMessagesByTimeRange(user_id, tenant_id, start, end, limit, offset)`（复用现有 PG/ClickHouse 冷热查询），返回某用户在该时间窗内可见的全部消息（跨会话）。
- 周总结可先按天聚合再汇总，控制上下文长度。

### 8.4 生成与提示词

- 复用 `getSystemBotLLMConfig`（系统 Bot 的 apikey/model）。
- 新增提示词文件 `work_service/prompt/daily_summary.md`、`weekly_summary.md`，对齐现有 `work_service/prompt/` 目录风格（参考已有的 `user_bot_safety_prompt.md`）。
- 空数据策略（评审确认）：即使当天/当周没有消息，**仍由系统 Bot 发一条占位消息**——日总结发「今日无话可说」，周总结发「本周无话可说」，保持固定节奏提醒。

### 8.5 投递（由系统 Bot 私聊发送）

复用现有链路，无需新推送通道：

1. 取系统 Bot 的用户 ID（`GetSystemBot` + `GetBotConfig` 的 `user_id`，由 `InitSystemBot` 的 `CreateBotUser` 建立）。
2. `GetOrCreatePrivateConversation(用户, 系统Bot用户)` 拿到/创建私聊会话（`bot_service` 私聊分支同款做法）。
3. 以系统 Bot 用户身份 `SendMessage`，写入该私聊会话；回复随后经既有 `bot-reply-topic` 推给 WS。

用户即在自己的「系统助手」私聊里看到每天/每周总结。

### 8.6 存储与接口

```go
// summary_report 生成的总结报告
type SummaryReport struct {
    ID             int64
    TenantID       int64
    UserID         int64
    Type           string // "daily" / "weekly"
    PeriodStart    int64
    PeriodEnd      int64
    Content        string `gorm:"type:text"`
    Model          string
    Status         int16 // 0 待生成 1 生成中 2 完成 3 失败
    Error          string
    CreatedAt      time.Time
}

// summary_schedule 用户的总结订阅设置
type SummarySchedule struct {
    ID        int64
    UserID    int64
    Type      string // daily / weekly
    CronSpec  string // daily="0 21 * * *"，weekly="0 21 * * 0"
    Timezone  string // 默认 Asia/Shanghai
    Enabled   bool
}
```

接口：`GET /api/summary/daily?date=`、`GET /api/summary/weekly?week=`、`GET /api/summary/list`、`GET /api/summary/{id}`、`POST /api/summary/generate`（手动触发）、`GET/POST /api/summary/schedule`。RPC 侧在 `work.thrift` 增 `GenerateDailySummary` / `GenerateWeeklySummary` / `GetSummaryReport`。

### 8.7 与用量统计的关系

总结消耗的是**系统 Bot 的 key**，属平台成本，**不计入用户 Bot 的用量统计**（如需可按租户配额统计，见第九节）。

---

## 九、需求 7（新增）：Token 用量统计与成本核算

### 9.1 目标与范围（评审确认）

**这是「统计 / 记账」，不是「卖服务」。** 平台统计用户用**自己的 apikey** 调用 LLM 时的 token 消耗，折算成预估成本，让用户看清「我这个 Bot 花了多少钱」。平台本身不售卖 Bot 服务，不做钱包、充值、余额扣减。

- **统计对象**：用户在站内创建（或从市场安装）的、**用自己的 apikey** 的 Bot（`origin = user / market`）。
- **不统计**：通过 **OpenAPI（AppID + AppSecret）接入的 Bot**（`origin = openapi`）——它们跑在用户自己部署的服务里，平台不参与，直接跳过。
- 判定方式：`bot_table` 新增 `origin`（`0=user`、`1=market`、`2=openapi`）。

### 9.2 计量数据采集

- 计量对象是 **LLM token**：eino `schema.Message.ResponseMeta.Usage` 提供 `PromptTokens/CompletionTokens/TotalTokens`，但 `work_service/internal/llm/client.go` 目前只回传 `Content`。
- 改造：`llm.Client` 新增 `ChatWithUsage(...)`（或在 `react.go` 回调里取 usage），保持旧 `Chat` 签名不变，向后兼容；Agent 多轮调用**逐轮累加**。
- 已存在的 `meter.M.BotTokenUsage`（`aim.bot.token.usage`）当前无调用点，正好在此接线，同时新增 `billing_cost_total`。

### 9.3 数据模型（新增 `billing_service` :4328，snowflake=8）

```go
// token_usage 用量明细
type TokenUsage struct {
    ID               int64
    BizDate          string `gorm:"index"` // yyyy-mm-dd
    UserID           int64  `gorm:"index"`
    TenantID         int64  `gorm:"index"`
    BotID            int64  `gorm:"index"`
    Origin           int16  // 0=user 1=market（openapi 不写入）
    ConversationID   int64
    MsgID            int64  `gorm:"uniqueIndex"` // 幂等去重
    Model            string
    PromptTokens     int64
    CompletionTokens int64
    Cost             float64 // 按参考单价折算的预估成本
    CreatedAt        time.Time
}

// model_price 参考单价表（管理后台维护，仅用于把 token 折算成估算成本）
type ModelPrice struct {
    ID          int64
    Provider    string
    Model       string `gorm:"index"`
    InputPer1K  float64 // 每千 input token 参考价
    OutputPer1K float64
    Currency    string
    Enabled     bool
    UpdatedAt   time.Time
}

// usage_quota 用量上限（可选：到阈值告警或限制，不做扣费）
type UsageQuota struct {
    ID         int64
    OwnerType  int16 // 0=user 1=tenant
    OwnerID    int64
    Period     int16 // 0=日 1=月
    TokenLimit int64
    Enabled    bool
}
```

> 与 v2 初稿的差异：删除了 `billing_account` / `billing_ledger`（余额与充值），因为不涉及真实付费；保留的只有「用量明细 + 参考单价 + 可选配额」。

### 9.4 记录流程

1. Bot 回复结束后取本轮 usage（含 Agent 多轮累加）。
2. `origin = openapi` → 直接跳过，不写任何记录。
3. 否则按 `cost = prompt/1000 × input_price + completion/1000 × output_price` 计算预估成本，写入 `token_usage`；按 `msg_id` 唯一索引做幂等去重。
4. 聚合：按 `biz_date` / 模型 / Bot / 租户维度汇总，供看板查询；命中 `usage_quota` 时告警（可选拦截）。

### 9.5 接口与看板

- 用户侧：`GET /api/billing/usage`（明细，按日期/模型/Bot 过滤）、`GET /api/billing/usage/summary`（趋势与合计）。**只能查看本人的用量**（按 `user_id` 强制过滤，不含任何跨用户/跨租户汇总）。
- 管理后台：`GET/POST /api/admin/billing/prices`（**参考单价由后台手工维护**）。
- 前端：新增「用量」页（token 趋势、按 Bot/模型占比、预估成本，仅本人数据）；管理后台加「参考单价」维护页。

> 评审确认：用量看板**仅本人可见**，不提供租户管理员或平台管理员的「看他人用量」入口；管理后台只保留参考单价的维护。

### 9.6 风险与边界

- 只统计 LLM token，不含 MCP/存储等其他成本（后续可扩展）。
- 单价是**参考价**，用于估算，可能与用户在其供应商侧的真实账单存在差异，文档需写明。
- 与需求 3/1 联动：市场安装的 Bot 用量归属安装者（`origin = market`）；OpenAPI 接入的 Bot 不计入。

---

## 十、需求 6：进一步可扩展方向（重点：个人 Skill、性能与规模）

### 10.1 重点专项 A：个人 Skill 体系

把系统 Bot 现有的 Skill 机制（`bot_service` 的 `initSkillKnowledgeBase`：创建知识库 + 上传 markdown）**产品化到普通用户**，让用户能上传、管理、绑定自己的 Skill。

> 评审确认：**本轮只支持「资料型 Skill」**——即纯 markdown 资料，运行期作为知识库被检索；**不支持可执行的脚本型 Skill**，也不支持自定义工具/代码执行（无需引入沙箱）。

- **Skill 包结构**：一个目录/zip，含 `SKILL.md`（YAML frontmatter：`name` / `description` / `version`）+ `references/*.md` + `assets/`。
- **存储**：复用 SeaweedFS（`storage.Client.PutObject`，参考 `uploadSkillFile` 的用法）与知识库（每个 skill 建一个独立 KB）。
- **模型**：

```go
type UserSkill struct {
    ID          int64
    OwnerID     int64
    TenantID    int64
    Name        string
    Description string
    Version     string
    Status      int16 // 0 待解析 1 可用 2 失败 3 停用
    KbID        int64 // 承载 skill 内容的知识库
    SizeBytes   int64
    CreatedAt   time.Time
}

type UserSkillFile struct {
    ID        int64
    TenantID  int64
    SkillID   int64
    RelPath   string
    ObjectURL string
    Hash      string
    SizeBytes int64
}

type BotSkill struct {
    TenantID int64 `gorm:"primaryKey"`
    BotID    int64 `gorm:"primaryKey"`
    SkillID  int64 `gorm:"primaryKey"`
    OwnerID  int64
    Enabled  bool
}
```

> 落地说明：`UserSkill` 增加 `FileCount` 便于列表展示；`UserSkillFile` 与 `BotSkill` 都带 `tenant_id`，
> 前者用于租户内清理文件记录，后者作为复合主键的一部分，保证同一 Bot 的绑定关系天然按租户隔离。

- **绑定与运行**：Bot 启用 skill 后，运行时把该 skill 的知识库并入 Agent 检索（复用 `search_knowledge` 与既有 KB 绑定链路）。
- **安全**：上传扫描（大小/类型限制、prompt 注入扫描，复用 `filterUserPrompt` 思路）；**只读、不执行代码**（资料型限定）；skill 仅本人/本租户可用，**不随市场分发**（与需求 3 的分发边界一致）。
- **前端**：`BotsPanel` 新增「Skill」页签（上传/列表/绑定到 Bot）。

### 10.2 重点专项 B：性能与规模优化

逐项对应现有实现（含已留 TODO 的点）：

1. **多端在线**：`msg_gateway/core/hub.go` 538 行的 Phase 7 TODO——`Clients` 由 `map[int64]*Client` 改 `map[int64][]*Client`，并打通跨网关推送。
2. **读/写扩散**：现有按 100 人阈值切换（README 522~541 行）。优化：阈值动态化、Inbox 异步批量写、大群读扩散用游标分页。
3. **冷热分层**：PG → ClickHouse 的迁移策略与查询路由调优（README 478 行描述的三级分层）。
4. **Kafka**：`bot-task-topic` 现按 `bot_id` 分区；多租户下考虑按租户前缀/合并相邻小 Bot 任务；`maxConcurrentBots=10` 改为可配置。
5. **向量库**：Qdrant collection 按租户/规模拆分，减少大 collection 的检索开销。
6. **数据库**：`message_table` 按 `tenant_id` + 时间分区；读写分离（读副本）；连接池调优。
7. **缓存**：key 统一加租户前缀；热点会话缓存。
8. **限流**：按租户/应用/用户的多级限流（复用 `RateLimitHitTotal`）。
9. **调度洪峰**：总结的 21:00 洪峰分片错峰（见 8.2）。
10. **可观测**：现有 OTEL/Prometheus/Jaeger 基础上补租户维度标签，成本/延迟大盘化。

### 10.3 其他方向（按投入产出排序）

1. **多模态输入**：语音消息 ASR、图片 OCR、文件（PDF/DOCX/PPTX 解析器已具备，见 `knowledge_service/internal/parser`）即时解析问答；回复侧 TTS（`llm.Client` 已支持多模态图片入参）。
2. **RAG 强化**：接入已部署的 `rerank` 做检索重排；答案引用溯源（citation）；置信度与「无法回答」兜底；RAGAS 评估面板化（`ragas-eval` 已存在）。
3. **企业能力**：SSO（企业微信/钉钉/飞书登录）、SCIM 用户/部门同步、审计日志中心、数据导出与合规删除。
4. **平台化**：开放平台 OAuth2（第三方代表用户操作）、Webhook 事件市场、官方 SDK（Go/TS/Python）。
5. **市场运营**：评价与排行、创作者分成、模板一键复制、Bot 版本回滚、Prompt A/B 测试。
6. **客户端**：移动端与官方示例工程。
7. **安全**：端到端加密会话、MCP 工具权限沙箱与调用审计配额。

---

## 十一、实施路线图

> 排序原则：地基先行；独立、低风险、高价值项优先交付；多租户单独成阶段并行推进。

| 阶段 | 内容 | 依赖 | 主要交付 |
| --- | --- | --- | --- |
| Phase 0 | 公共地基：`pkg/tenant` 上下文、审计、**独立管理后台账号体系**、迁移约定、ID 分片约定 | 无 | 基础设施包 + 迁移脚本 + 管理后台骨架 |
| Phase 1 | 需求 2 多租户：个人/组织租户、按用户回填、JWT 与 RPC 透传、隔离验收 | Phase 0 | 多租户端到端可用 |
| Phase 2 | 需求 1 开放平台：`open_service` + `/open/v1` + 签名鉴权 + 拉取/回调 + 文档 | Phase 1 | 开放平台 + `openapi.yaml` + 文档站 |
| Phase 3 | 需求 3 Bot 市场：定义/实例拆分、人工审核、全局可见、一键安装 | Phase 1 | 市场前后端 + 审核后台 |
| Phase 4 | 需求 5 日/周总结：时间窗查询 RPC、调度器、系统 Bot 私聊投递 | Phase 1 | 日/周总结 |
| Phase 5 | 需求 7 Token 用量统计与成本核算：token 采集、用量明细、参考单价、用量看板（OpenAPI 不计入） | Phase 1、Phase 3 | 用量统计闭环 |
| Phase 6 | 需求 4 多 Agent 可视化编排：图模型、画布、Eino 图执行、限额降级 | Phase 1 | 协作编排 |
| Phase 7 | 需求 6 选做项：个人 Skill、性能与规模、其他方向（按优先级滚动） | — | 逐个增强 |

并行建议：Phase 3/4 相对独立，可在 Phase 1 完成后并行；Phase 5 依赖 Phase 3 的 `origin` 字段；Phase 6 最复杂，放最后。

> 每个阶段的具体步骤见「十二、实施步骤清单」。

---

## 十二、实施步骤清单（每一步做什么）

> 说明：以下为进入编码后的逐项任务，按依赖顺序排列；每一步完成后应有可独立验证的验收点。括号内为落点参考路径。

### Phase 0：公共地基（已完成）

1. 新建 `pkg/tenant`：实现 `WithTenant` / `FromContext` / `MustFromContext` 与 `X-Tenant-Id` 常量，补单测。
2. 搭建独立管理后台账号体系：
   - 新增服务 `admin_service`（:4329，snowflake=9）与表 `admin_user`（`account/hash/name/role/status/last_login_at`）。
   - 启动时按配置 `admin.bootstrap` 幂等创建**初始管理员**（账号/名称/角色/初始口令）。
   - 实现登录/登出/改密 RPC，签发独立 JWT（claims 带 `admin_id` / `admin_role`）。
   - `api_gateway` 新增 `/api/admin/*` 路由组 + `middleware/admin_auth.go`（`RequireAdmin`）。
3. 新建 `pkg/audit` + `audit_log` 表与写入 API，先接入 5 类事件（开放平台调用、Bot 发布/审核、租户管理、参考单价维护、总结生成）。
4. 固化迁移约定：新增列一律「可空/带默认值」，落地迁移脚本模板与迁移报告（回填后断言 `tenant_id=0` 记录数为 0）。
5. 固化 ID 分片：`open=7`、`billing=8`、`admin=9`，在 `pkg/snowflake` 与各服务初始化处登记。
6. 扩展 `pkg/observability/meter`：新增 `open_api_request_total`、`market_install_total`、`summary_generate_total`、`agent_graph_run_total`、`billing_cost_total`；把 `BotTokenUsage` 的接线点预留好。
7. 前端新增管理后台骨架（独立路由/页面容器 + 管理员登录），供后续市场审核、单价维护复用。

### Phase 1：多租户（需求 2）（已完成）

1. 建表 `tenant` / `tenant_member` / `tenant_department`（含 `is_personal`），在 `user_service` 内实现 CRUD RPC。
2. 改造注册流程：新用户注册时自动创建个人租户并写入 owner 成员。
3. 编写存量迁移脚本：每个已有 user 建个人租户 → 按 `user_id/creator_id/owner_id` 逐表回填 `tenant_id` → 系统 Bot 归平台租户 → 输出校验报告。
4. 各表增列 `tenant_id` + 索引，并把唯一键调整为 `(tenant_id, key)` 复合唯一（表清单见 5.5）。
5. JWT 与上下文：claims 增 `tenant_id`；`middleware.Resp` 扩展为 `{Id, Account, TenantId}`；登录返回租户列表；新增 `/api/tenant/switch` 重签令牌。
6. RPC 透传：各 `*.thrift` 请求体新增 `optional i64 tenant_id`，重新生成 `kitex_gen`；`api_gateway` 与各服务在入口注入/读取租户。
7. DAO 改造：新增统一租户过滤辅助，逐服务（user/group/chat/bot/work/knowledge）把查询改为强制带 `tenant_id`。
8. 存储隔离：PG（message/inbox 增列）、Qdrant（payload 过滤）、Meilisearch（filter）、Neo4j（属性条件）、ClickHouse（冷表列）、Garnet（key 前缀）、Kafka（消息体带 `tenant_id`）。
9. 租户管理接口：`create/list/switch/info/update/members/member/add/set_role/remove`（**本轮入群=管理员直接添加**）+ **部门树**（`department/create|rename|move|remove|set_member`，本轮一起做）。
10. 越权验收：编写「A 租户访问 B 租户资源应返回空/403」用例，覆盖各服务。

### Phase 2：开放平台（需求 1）（已完成）

1. 新建 `open_service`（:4327，snowflake=7）与表 `open_app` / `open_webhook` / `open_event` / `open_webhook_delivery`。
2. 实现 `open_auth` 中间件：HMAC-SHA256 验签 + 时间戳窗口 + nonce 去重 + scope 校验。
3. 实现 `/open/v1/bots` 系列（CRUD、拉入会话、MCP、知识库绑定），内部复用现有 RPC；创建时置 `origin=openapi`。
4. 实现 `/open/v1/messages/send`、`/open/v1/messages`、`/open/v1/conversations`。
5. 事件源头：新增 `open_dispatch` 消费者订阅 `bot-reply-topic`，写入 `open_event`。
6. 拉取基线：`GET /open/v1/events`（游标分页，游标落库）。
7. 回调：`POST /open/v1/webhooks` + `/verify` 挑战回显 + 签名 POST 投递 + 指数退避重试 + 死信 + `POST /open/v1/events/ack`。
8. 限流与审计：按 AppID 的 QPS 限制与请求审计。
9. 文档：`docs/openapi/open_platform.yaml`、`/open/docs`（Redoc/Swagger UI）、`docs/open_platform.md`、README 索引。

### Phase 3：Bot 市场（需求 3）（已完成）

1. 建 `market_bot` / `market_bot_install` 表（**不含** KB/MCP 字段）。
2. 发布接口：`filterUserPrompt` 预审 → 写「待审」。
3. 管理后台审核：列表 + 通过/驳回（写 `ReviewNote` / `ReviewedBy` / 审计）。
4. 浏览与安装：`GET /api/market/bots`、`install`（body 带 `api_key/model/base_url/name`）→ 写 `bot_table`（`origin=market`）并记录 `market_bot_install`。
5. 评分/下架/版本更新（改提示词需重新待审）。
6. 前端：`BotsPanel` 增 `market` / `publish` Tab；管理后台增「市场审核」页。

交付物（对应落点）：

- 模型与存储：`kitex_service/bot_service/internal/model/market_bot.go`、`market_bot_install.go`、`market_bot_rating.go`，DAO 见 `internal/dal/market_bot.go`。
- 业务与接口：`internal/service/market.go`（发布/更新/下架/安装/评分/审核/列表），`idl/bot.thrift` 新增 10 个 RPC 并重新生成 `kitex_gen`。
- 网关：`api_gateway/handle/market.go`（用户端 8 个 + 管理端 3 个），路由见 `api_gateway/routes/routes.go`（`/api/market/*`、`/api/admin/market/*`）。
- 隔离用例：`internal/dal/market_isolation_test.go`（跨租户不可见、评分汇总维护）。
- 前端：`web/test_frontend/src/components/BotsPanel.tsx`（市场 / 发布 Tab）、`src/admin/AdminMarketReviewPanel.tsx` + `AdminApp.tsx` 导航接线。
- 文档：`docs/api_doc.md` 第 11 章「Bot 市场」（状态机、用户端与管理端接口、字段说明）。

### Phase 4：日/周总结（需求 5）（已完成）

1. `chat.thrift` 新增 `GetMessagesByTimeRange`，实现跨会话、按时间窗查询（复用冷热查询）；`user.thrift` 新增 `ListActiveUsers` 供调度游标枚举用户。
2. 建 `summary_report` / `summary_schedule` 表，`(user_id, type, period_start)` 唯一，支撑调度幂等。
3. `work_service/internal/scheduler`：ticker 扫描（默认 60s，未引入 `robfig/cron`）+ Garnet 分布式 run 锁 + 按 `user_id` 取模错峰（默认 30 分钟窗口）。
4. 新增提示词 `work_service/prompt/daily_summary.md`、`weekly_summary.md`（可配置，缺失时回退内置兜底）。
5. 生成逻辑：跨会话聚合；空档发「今日无话可说 / 本周无话可说」占位，不调用 LLM。
6. 投递：`GetOrCreatePrivateConversation(用户, 系统Bot用户)` + 以系统 Bot 身份 `SendMessage`，并写 `bot-reply-topic` 推送。
7. 接口：`/api/summary/*`（daily/weekly/list/detail/generate/schedule）。
8. 前端：总结查看入口 + 订阅设置。

交付物（对应落点）：

- 模型与存储：`kitex_service/work_service/internal/model/summary.go`（`summary_report`、`summary_schedule`），DAO 见 `internal/dal/summary.go` 与 `internal/dal/interface.go`。
- 业务与调度：`internal/service/summary.go`（生成/幂等/时间线/投递/订阅），`internal/scheduler/summary.go`（ticker + run 锁 + 错峰）。
- RPC 与接口：`idl/work.thrift` 新增 5 个总结 RPC 并重新生成 `kitex_gen`；`idl/chat.thrift` 的 `GetMessagesByTimeRange`、`idl/user.thrift` 的 `ListActiveUsers`；`api_gateway/rpc/work.go`、`api_gateway/handle/summary.go`、`api_gateway/routes/routes.go`（`/api/summary/*`）。
- 提示词：`work_service/prompt/daily_summary.md`、`weekly_summary.md`。
- 配置：`work_service/internal/config/config.go` 与 `config.yaml`（MySQL/Redis、`ai.summary.*`、`summary.*`）。
- 前端：`web/test_frontend/src/components/SummariesPanel.tsx` + `SummariesPanel.css`、`Sidebar.tsx` / `MainPage.tsx` 导航接线。
- 文档：`docs/api_doc.md` 第 12 章「日/周总结」。

### Phase 5：Token 用量统计与成本核算（需求 7）

1. `llm.Client` 采集 usage（`ChatWithUsage` 或回调取 `ResponseMeta.Usage`），Agent 多轮累加；接线 `BotTokenUsage`。
2. 新建 `billing_service`（:4328，snowflake=8）与表 `token_usage` / `model_price` /（可选）`usage_quota`。
3. `work_service` 在 Bot 回复结束后写 `token_usage`（`origin=openapi` 跳过；按 `msg_id` 幂等）。
4. 成本折算：`usage × model_price`；未配置单价则只记 token、成本记 0 并告警。
5. 接口：`/api/billing/usage`、`/api/billing/usage/summary`（**仅本人可见**）；管理后台 `/api/admin/billing/prices`（参考单价手工维护）。
6. 前端：新增「用量」页（趋势/占比/预估成本，仅本人数据）；管理后台「参考单价」页。

### Phase 6：多 Agent 可视化编排（需求 4）（已完成）

1. 建 `bot_agent_graph` / `bot_agent_graph_history`；`bot.thrift` 增 `SaveAgentGraph` / `GetAgentGraph` / `ValidateAgentGraph` / `PublishAgentGraph` / `ListAgentGraphHistory` 五个 RPC。
2. 图 schema 落在公共包 `pkg/agentgraph`（nodes/edges 字段定义 + 校验规则），「保存校验」与「运行前校验」共用同一套规则，避免前后端规则漂移。
3. `work_service/internal/agent/graph.go`：按 `TopoLayers()` 拓扑分层顺序执行，`agent` 节点复用 `react.go` 的单 Agent 执行器，前驱节点输出作为上下文注入，结果在 `end` 节点聚合。
4. `handleWithAgent` 分支：Bot 存在「已发布」编排图时走图执行，否则（未配置/图未发布/运行失败）回退现有单 Agent，并打点区分 `success` 与 `fallback`。
5. 限额与降级：节点数上限 12、单节点步数上限 6、整图执行超时默认 240s（可配 `ai.agent_graph.timeout_seconds`）；超时或异常自动退回单 Agent，不让编排图成为回复失败的单点。
6. 前端：`BotsPanel` 增「编排」Tab（React Flow 画布 + 节点配置侧栏 + 校验/保存/发布/历史回滚）。

**本轮落地范围说明**：为控制风险，节点类型收敛为 `start` / `agent` / `end` 三类（`knowledge`、`tool`、`condition`、`parallel`/`join` 未纳入本轮），工具以 `none` / `knowledge` / `memory` 白名单按节点声明；执行按拓扑分层串行推进，未引入 Eino `compose.Graph`。画布已预留节点类型扩展位，条件分支与并发执行可在 Phase 7 滚动增强。

交付物（对应落点）：

- 图 schema 与校验：`pkg/agentgraph/graph.go` + `graph_test.go`（节点/连线上限、单一 `start`、至少一个 `end`、无环与可达性、工具白名单、prompt 长度）。
- IDL：`idl/bot.thrift`（`AgentGraphInfo` / `AgentGraphHistoryInfo` 与 5 个 RPC，状态机 `0 草稿 / 1 已发布 / 2 已停用`）。
- 存储与业务：`kitex_service/bot_service/internal/model/agent_graph.go`（`bot_agent_graph` / `bot_agent_graph_history`）、`internal/dal/agent_graph.go`（租户维度过滤）、`internal/service/agent_graph.go`（保存即重置为草稿并递增版本、写历史快照与审计）、`handler.go`、`main.go`（AutoMigrate + 装配）。
- 运行引擎：`kitex_service/work_service/internal/agent/graph.go`、`internal/agent/react.go`（`MaxSteps` 可配）、`internal/service/service.go`（`runAgentGraph` 与回退分支）、`internal/config/config.go`（`ai.agent_graph.*`）；`rpc/bot.go` 的 `GetAgentGraph`。
- 网关：`api_gateway/rpc/bot.go`、`api_gateway/handle/agent_graph.go`、`api_gateway/routes/routes.go`（`/api/bot/agent-graph*`）。
- 可观测：`pkg/observability/meter/meter.go` 的 `aim.agent.graph.run.total`（`success` / `fallback`）。
- 前端：`web/test_frontend/src/components/AgentGraphPanel.tsx` + `AgentGraphPanel.css`、`BotsPanel.tsx` 编排 Tab 接线。
- 文档：`docs/api_doc.md` 第 15 章「多 Agent 协作编排」（画布 schema、校验规则、接口与字段说明）。

### Phase 7：扩展方向（需求 6，滚动）（已交付：个人 Skill + 多端在线 + 并发可配置）

1. 个人 Skill（资料型）：建 `user_skill` / `user_skill_file` / `bot_skill`；上传（SeaweedFS + 每 skill 一个 KB）；安全扫描；绑定运行；前端「Skill」Tab。（已完成）
2. 性能与规模（代码侧）：多端 `Clients map[int64][]*Client`、已读回执多端同步与跨网关转发、Bot 任务并发上限配置化。（已完成）
3. 性能与规模（基础设施侧）：读写扩散调优、冷热分层、Kafka 分区、Qdrant 拆分、DB 分区/读副本、缓存前缀、多级限流、总结错峰、租户维度可观测。（按部署环境滚动推进）
4. 其他方向（多模态 / RAG 强化 / 企业能力 / 市场运营 / 客户端）按优先级滚动立项。

交付物（对应落点）：

- 模型与存储：`kitex_service/bot_service/internal/model/user_skill.go`（`user_skill` / `user_skill_file` / `bot_skill`），DAO 见 `internal/dal/user_skill.go`。
- 业务与接口：`internal/service/skill.go`（zip / markdown 解析、frontmatter 元信息、体积与类型限制、提示词注入扫描、SeaweedFS 落盘、每 Skill 独占知识库、绑定/解绑与回收），`idl/bot.thrift` 新增 6 个 RPC 并重新生成 `kitex_gen`。
- 网关：`api_gateway/handle/skill.go`（multipart 上传 + 5 个读写接口）、`api_gateway/rpc/bot.go`、`api_gateway/routes/routes.go`（`/api/skill/*`）。
- 运行期：绑定即调用 `knowledge_service.BindKnowledgeBase` 写入 `bot_knowledge`，复用既有 `search_knowledge` 检索链路，`work_service` 无需改动。
- 多端在线：`msg_gateway/core/hub.go`（`Clients` 改多连接、`attachDeviceLocked` / `detachDeviceLocked` / `localDevices` / `sendToDevices`、已读回执多端与跨网关转发、单账号连接上限 5）。
- 并发可配置：`work_service/internal/consumer/bot_task.go` 读取 `ai.bot_task.max_concurrent` / `ai.bot_task.per_bot_queue`。
- 用例：`internal/service/skill_test.go`（frontmatter、路径穿越、垃圾文件、套目录拍平、单文件与注入拦截）、`internal/dal/user_skill_isolation_test.go`（租户过滤语句断言）、`msg_gateway/core/hub_test.go`（多端注册/注销/淘汰/快照）。
- 前端：`web/test_frontend/src/components/SkillPanel.tsx` + `SkillPanel.css`、`BotsPanel.tsx` 新增「Skill」页签、`api/client.ts` 抽出 `uploadFileTo`。
- 文档：`docs/api_doc.md` 第 16 章「个人 Skill」。

---

## 十三、依赖与风险

- **依赖**：Phase 1（多租户）是 2/3/4/5/7 的共同前置；Phase 0 的迁移与 ID 约定是全部前置。
- **多租户越权**（最高风险）：集中式 DAO 过滤 + 专门越权用例 + 评审清单；必要时上 DB RLS。
- **IDL/生成代码全量变更**：Thrift 只增不改；一次性对齐各服务 `kitex_gen`，避免半迁移。
- **开放平台安全**：密钥只存哈希、时间戳 + nonce 防重放、回调验签、限流；文档明确错误码与幂等。
- **市场内容安全与资产外泄**：提示词过审核、**不分发 KB/MCP**、人工审核状态机、下架机制。
- **画布执行风险**：图编译适配工作量大；必须设节点/步数/耗时/token 上限与降级。
- **用量统计准确性**：token 采集需覆盖 Agent/多轮调用（逐轮累加）；写入需按 `msg_id` 幂等去重，避免重试导致重复计数。
- **成本与延迟**：多 Agent 与总结均可能放大 LLM 开销，必须设上限与降级。
- **调度负载**：固定时刻的总结洪峰需分布式锁 + 错峰/分片。

---

## 十四、已确认决策与待定问题

### 14.1 已确认（三轮评审汇总）

| # | 问题 | 决策 |
| --- | --- | --- |
| 1 | 多租户粒度 | **按飞书式部署多租户**（组织租户 + 用户可属多租户 + 切换） |
| 2 | 存量数据归属 | **按用户各自拆成独立租户**（每人一个个人租户，老数据归入其中） |
| 3 | 开放平台形态 | **只用 AppID + AppSecret**，不引入 OAuth2；**支持回调（Webhook）**，并同时保留轮询基线（见 4.3） |
| 4 | 市场审核与可见性 | **需要人工审核**；市场**全局可见** |
| 5 | 市场分发内容 | **不分发创作者知识库、不分发 MCP**；只分发定义，安装者自带 apikey + 模型 |
| 6 | 多 Agent 配置方式 | **使用可视化编排画布** |
| 7 | 总结维度与推送 | **用户全部会话**；**由系统 Bot 发送**；日总结 21:00、周总结周日 21:00；**空档发「今日无话可说 / 本周无话可说」占位** |
| 8 | 计费含义 | **统计用户自带 apikey 的 token 消耗与预估成本（记账，不售卖服务）**；**OpenAPI 接入的 Bot 不统计** |
| 9 | 平台管理员 | **独立的管理后台账号体系**（独立服务 + 独立登录 + `/api/admin/*`） |
| 10 | 组织租户入群 | **本轮仅支持管理员直接添加**（不做邀请码/邀请链接） |
| 11 | 个人 Skill 边界 | **仅资料型 Skill**（知识库检索），**不支持可执行脚本型** |
| 12 | 扩展方向 | 重点增加**个人 Skill 体系**与**性能与规模优化** |
| 13 | 参考单价来源 | **由后台手工维护**价格表（不做供应商自动同步） |
| 14 | 用量看板可见范围 | **仅本人可见**自己的用量与成本 |
| 15 | 初始管理员开户 | **通过配置文件创建**（`admin.bootstrap` 引导首个账号） |
| 16 | 部门树范围 | **本轮一起做**（`tenant_department` 及成员归属） |
| 17 | OpenAPI 默认值 | **采用 4.8 节给定的默认取值**（限流、重试、死信、事件保留等） |

### 14.2 待定问题状态（编码前）

**无未决问题（已清零）。** 第三轮评审的 5 个问题均已确认并落实到正文：

- 参考单价来源 → 后台手工维护，见 9.5。
- 用量看板可见范围 → 仅本人可见，见 9.5 与 Phase 5。
- 初始管理员开户 → 配置文件创建，见 3.2 与 Phase 0。
- 部门树范围 → 本轮一起做，见 5.7 与 Phase 1。
- OpenAPI 默认值 → 采用 4.8 节默认取值。

### 14.3 编码落地进度

| 阶段 | 状态 | 说明 |
| --- | --- | --- |
| Phase 0 公共地基 | 已完成 | `pkg/tenant`、`pkg/audit`、`admin_service` + `/api/admin/*`、ID 分片、迁移脚本模板、指标扩展、管理后台骨架 |
| Phase 1 多租户 | 已完成 | `tenant`/`tenant_member`/`tenant_department`、注册建个人租户、存量回填脚本、JWT/RPC 透传、各服务 DAO 租户过滤、越权用例 |
| Phase 2 开放平台 | 已完成 | `open_service`（:4327）、`open_auth` 签名鉴权、`/open/v1/*`、事件轮询 + Webhook 回调、`docs/openapi/open_platform.yaml`、`docs/open_platform.md`、`/open/docs` |
| Phase 3 Bot 市场 | 已完成 | `market_bot` / `market_bot_install` / `market_bot_rating` + DAL/Service、`bot.thrift` 10 个 RPC、`api_gateway/handle/market.go`（`/api/market/*`、`/api/admin/market/*`）、前后端市场与审核页、`docs/api_doc.md` 第 11 章、市场隔离用例 |
| Phase 4 日/周总结 | 已完成 | `GetMessagesByTimeRange` / `ListActiveUsers` RPC、`summary_report` / `summary_schedule` + DAL、`work_service` 生成与投递逻辑、ticker 调度器（run 锁 + 错峰）、5 个总结 RPC、`/api/summary/*`、前端总结面板、`docs/api_doc.md` 第 12 章 |
| Phase 5 Token 用量统计 | 已完成 | `billing_service`（:4328）、Token 用量落库（`token_usage` / `model_price` / `usage_quota`）、`work_service` 在发送成功后按 Bot 来源记账（OpenAPI 不计、市场计入安装者、自建计入发送者）、后台手工维护参考单价 + 审计、用户侧 `/api/billing/usage` 与 `/api/billing/usage/summary`、管理后台参考单价页、前端用量面板、`docs/api_doc.md` 第 13/14 章 |
| Phase 6 多 Agent 编排 | 已完成 | `pkg/agentgraph` 图 schema 与校验、`bot_agent_graph` / `bot_agent_graph_history` + DAL/Service、`bot.thrift` 5 个 RPC、`work_service` 图执行引擎（分层执行 + 单 Agent 复用 + 失败回退）、`agent_graph_run_total` 指标、`/api/bot/agent-graph*`、前端 React Flow 编排画布、`docs/api_doc.md` 第 15 章 |
| Phase 7 扩展方向 | 进行中 | 个人 Skill（资料型）已交付：`user_skill` / `user_skill_file` / `bot_skill` + DAL/Service、`bot.thrift` 6 个 RPC、`/api/skill/*`、SeaweedFS + 每 Skill 独占知识库、提示词注入扫描、绑定复用 `bot_knowledge` 检索链路、前端「Skill」页签、`docs/api_doc.md` 第 16 章；性能与规模代码侧已交付：`msg_gateway` 多端连接（`Clients` 改 `map[int64][]*Client`、已读回执多端与跨网关转发、单账号连接上限）、Bot 任务并发上限配置化；基础设施侧调优（读写扩散、冷热分层、分区/拆分、多级限流、租户维度可观测）随环境滚动推进 |

**当前状态：Phase 0/1/2/3/4/5/6 已交付，Phase 7 的个人 Skill 与多端在线已交付，`billing_service`、`user_service`、`chat_service`、`work_service`、`bot_service`、`msg_gateway`、`api_gateway` 构建全绿，前端 `npm run build` 通过；下一阶段为 Phase 7 剩余项（性能与规模基础设施侧、多模态 / RAG 强化等扩展方向）。**

