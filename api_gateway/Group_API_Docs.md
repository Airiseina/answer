
## 二、AIM 项目完整指标清单

### 第一层：核心指标（需求明确要求，必须实现）

| 指标名 | 类型 | 单位 | 所属服务 | 说明 | 需求来源 |
|--------|------|------|---------|------|---------|
| `aim.online.users` | Gauge | 人 | msg_gateway | 当前在线用户数 | 进阶要求：在线人数 |
| `aim.message.sent.total` | Counter | 条 | msg_gateway/chat_service | 发送消息总数，带 label: `type`(text/image/file/voice), `chat_type`(single/group/broadcast) | 基本要求：消息收发 |
| `aim.message.latency` | Histogram | ms | msg_gateway | 消息从接收到投递的延迟 | 进阶要求：消息吞吐量 |
| `aim.bot.request.total` | Counter | 次 | chat_service | Bot 请求总数，带 label: `model`, `status`(success/error) | 基本要求：聊天 Bot |
| `aim.bot.response.latency` | Histogram | ms | chat_service | Bot 响应延迟，带 label: `model` | 进阶要求：Bot 响应延迟 |
| `aim.bot.token.usage` | Counter | token | chat_service | Token 消耗量，带 label: `model`, `direction`(input/output) | 基本要求：计费管理 |

### 第二层：运维指标（保障系统稳定运行）

| 指标名 | 类型 | 单位 | 所属服务 | 说明 |
|--------|------|------|---------|------|
| `aim.ws.connect.total` | Counter | 次 | msg_gateway | WebSocket 连接总数 |
| `aim.ws.disconnect.total` | Counter | 次 | msg_gateway | WebSocket 断开连接总数 |
| `aim.message.search.latency` | Histogram | ms | chat_service | 历史消息搜索延迟 |
| `aim.group.operation.total` | Counter | 次 | api_gateway/group_service | 群组操作计数，带 label: `operation`, `status` |
| `aim.user.register.total` | Counter | 次 | user_service | 用户注册总数，带 label: `status` |
| `aim.user.login.total` | Counter | 次 | user_service | 用户登录总数，带 label: `status` |

### 第三层：进阶指标（实现进阶功能时加入）

| 指标名 | 类型 | 单位 | 所属服务 | 说明 | 需求来源 |
|--------|------|------|---------|------|---------|
| `aim.bot.rag.query.latency` | Histogram | ms | chat_service | RAG 知识库查询延迟 | 进阶：RAG Bot |
| `aim.bot.rag.document.process.total` | Counter | 次 | work_service | 文档处理计数，带 label: `status` | 进阶：RAG Bot |
| `aim.bot.mcp.tool.call.total` | Counter | 次 | chat_service | MCP 工具调用计数，带 label: `tool_name`, `status` | 进阶：MCP 工具集成 |
| `aim.bot.mcp.tool.latency` | Histogram | ms | chat_service | MCP 工具调用延迟，带 label: `tool_name` | 进阶：MCP 工具集成 |
| `aim.translate.latency` | Histogram | ms | chat_service | 实时翻译延迟 | 进阶：实时翻译 |
| `aim.content.audit.total` | Counter | 次 | chat_service | 内容审核计数，带 label: `result`(pass/reject) | 进阶：内容审核 |
| `aim.push.offline.total` | Counter | 次 | chat_service | 离线推送计数 | 进阶：离线推送 |

### 第四层：框架自动指标（已有，无需额外代码）

| 指标 | 来源 | 说明 |
|------|------|------|
| HTTP 请求量/延迟/错误率 | Hertz OTel 中间件 | api_gateway 自动产生 |
| RPC 请求量/延迟/错误率 | Kitex tracing suite | user_service/group_service 自动产生 |
| WebSocket 升级请求量/延迟 | otelhttp | msg_gateway 自动产生 |
| Go 运行时指标 | OTel runtime instrumentation | 所有服务自动产生（goroutine 数、GC 等） |

---

## 三、AIM 指标搭建完整路线图

```
阶段0（已完成）          阶段1（聊天功能）          阶段2（Bot功能）           阶段3（进阶功能）
━━━━━━━━━━━━━━━    ━━━━━━━━━━━━━━━━━    ━━━━━━━━━━━━━━━━━━    ━━━━━━━━━━━━━━━━━━
✅ pkg/meter 搭建     开发消息收发时同步加：    开发 Bot 时同步加：       开发进阶功能时同步加：
✅ msg_gateway 集成   · message.sent.total    · bot.request.total     · rag.query.latency
✅ api_gateway 集成   · message.latency       · bot.response.latency   · mcp.tool.call.total
✅ online.users      · message.search.latency · bot.token.usage        · translate.latency
✅ ws.connect.total                                                   · content.audit.total
✅ ws.disconnect.total                                                · push.offline.total
                     
                     开发好友/群组时同步加：                           
                     · group.operation.total                           
                     · user.register.total                            
                     · user.login.total                               
                                                           
                     ──────────────    ──────────────    ──────────────
                     阶段1完成后：       阶段2完成后：       阶段3完成后：
                     配置 Grafana       添加 Bot 面板      添加进阶面板
                     基础仪表盘         + Bot 告警规则     + 完整告警体系
```

### 每个阶段的具体步骤

**阶段1 — 聊天功能开发时**：
1. 在 `client.go` 的 `ReadMessage()` 中加 `meter.M.MessageSentTotal.Add()`
2. 在消息投递逻辑中加 `meter.M.MessageLatency.Record()`
3. 在 chat_service 的搜索接口中加 `meter.M.MessageSearchLatency.Record()`
4. 在 group handler 中加 `meter.M.GroupOpTotal.Add()`
5. 在 user_service 中加 `meter.M.UserRegisterTotal.Add()` / `meter.M.UserLoginTotal.Add()`
6. 配置 Grafana 基础仪表盘（在线人数、消息吞吐量、消息延迟）

**阶段2 — Bot 功能开发时**：
1. 在 Bot 调用大模型前后加 `meter.M.BotResponseLatency.Record()`
2. 在 Bot 调用入口加 `meter.M.BotRequestTotal.Add()`
3. 在计费逻辑中加 `meter.M.BotTokenUsage.Add()`
4. 添加 Bot 专用 Grafana 面板 + 响应延迟告警

**阶段3 — 进阶功能开发时**：
1. 在 RAG 查询中加 `aim.bot.rag.query.latency`
2. 在 MCP 工具调用中加 `aim.bot.mcp.tool.call.total` + `aim.bot.mcp.tool.latency`
3. 在翻译/审核中加对应指标
4. 完善告警体系（在线人数骤降、消息延迟飙升、Bot 错误率等）

---

### 关键原则

**指标跟着功能走，一行代码顺手加**——和写日志一样，写功能代码时就把指标埋进去，不要"后面补"。`meter.M` 已经是全局可用的，任何地方一行代码就能记录。

依旧无法创立单会话，以及发送群聊消息


这能工作的前提是 浏览器能直接访问到 SeaweedFS Filer 的地址 。在开发环境下没问题，但生产环境有两种方案：

方案 做法 适用场景 直接暴露 Filer 前端直连 Filer URL 开发/小规模部署 API 网关代理 Hertz 加一个 /files/* 路由，反向代理到 Filer 生产环境（统一域名、加鉴权、加 CORS）

如果你现在想先跑通，直接用 Filer URL 就行。后续生产化时加一层代理即可。