# API 文档变更说明

> 变更日期：2026-05-24
> 涉及版本：@提及功能 + Redis Stream 异步任务队列 + 鉴权机制

---

## 一、变更概览

本次变更主要涉及以下三大功能：

1. **@提及功能**：群聊和多人私聊中支持 @提及成员和 Bot，触发 Bot 回复
2. **Redis Stream 异步任务队列**：Bot 回复从同步 RPC 调用改为异步 Redis Stream 队列处理
3. **鉴权机制**：在 Bot 触发和任务消费两个环节验证发送者是否为会话成员

---

## 二、具体修改位置及内容

### 2.1 消息内容格式（5.1 获取消息列表）

**修改位置**：`api_doc.md` 第 5.1 节 messages 字段说明

**修改内容**：

| 修改项 | 修改前 | 修改后 |
|--------|--------|--------|
| content 字段说明 | `消息内容` | `消息内容（JSON 格式，详见下方消息内容格式说明）` |
| 新增内容 | 无 | 添加"消息内容格式说明"章节，包含 text/image/file/voice 四种类型的 JSON 结构说明 |
| 新增内容 | 无 | 添加 `mentions` 字段说明（MentionItem 结构：user_id + name） |

**目的和作用**：
- 原文档中 `content` 字段仅描述为"消息内容"，未说明其 JSON 结构
- 新增的 @提及功能需要在文本消息中携带 `mentions` 数组，必须明确文档化
- 统一说明所有消息类型的 JSON 格式，便于前端开发者正确解析

---

### 2.2 将 Bot 拉入会话（6.5 AddBotToConversation）

**修改位置**：`api_doc.md` 第 6.5 节

**修改内容**：

| 修改项 | 修改前 | 修改后 |
|--------|--------|--------|
| 接口描述 | "将 Bot 拉入群聊或创建与 Bot 的单聊会话" | "将 Bot 拉入群聊、已有私聊，或创建与 Bot 的一对一单聊会话" |
| Bot 使用说明 | "通过 `/api/bot/chat` 发送消息" | "群聊和多人私聊中通过 @提及触发，一对一 Bot 单聊中直接发送消息即可触发" |
| conversation_id 说明 | "群聊会话 ID（conversation_type=2 时必填）" | "会话 ID（conversation_type=2 时必填；conversation_type=1 且拉入已有私聊时必填；创建新 Bot 单聊时传 `0`）" |
| 请求示例 | 仅群聊和新建单聊两种 | 新增"已有私聊场景请求示例" |
| 说明部分 | 群聊 + 单聊两种场景 | 群聊 + 已有私聊 + 新 Bot 单聊三种场景 |

**目的和作用**：
- 支持将 Bot 拉入已有的双人私聊（如用户5和用户6的私聊），这是用户明确需求
- 明确不同会话类型下 Bot 的触发方式差异（@提及 vs 直接发消息）
- 会话类型仍为 `1`（private），不改变会话类型定义

---

### 2.3 与 Bot 对话（6.6 Bot Chat）

**修改位置**：`api_doc.md` 第 6.6 节

**修改内容**：

| 修改项 | 修改前 | 修改后 |
|--------|--------|--------|
| 接口描述 | "接口立即返回，Bot 会在后台调用 LLM 生成完整回复" | "消息会被推送到 Redis Stream 分布式任务队列（`bot:task:stream`），由 work_service 异步消费处理" |
| 新增说明 | 无 | "在群聊和多人私聊中，Bot 仅在消息中 @提及 Bot 时才会被触发；在一对一 Bot 单聊中，所有消息都会触发 Bot 回复" |
| history 参数 | 存在（`string[]`，可选） | **已移除** |
| success 说明 | "已成功触发 AI 处理" | "任务已成功推送到 Redis Stream 队列" |
| 新增说明 | 无 | "鉴权机制：work_service 消费任务时会验证发送者是否为会话成员" |

**目的和作用**：
- 反映后端从同步 RPC 调用改为 Redis Stream 异步队列的架构变更
- 移除 `history` 参数，因为历史对话由 work_service 内部管理
- 明确 Bot 触发规则，避免客户端误用
- 说明鉴权机制的存在

---

### 2.4 Bot 消息推送机制（6.7）

**修改位置**：`api_doc.md` 第 6.7 节

**修改内容**：

| 修改项 | 修改前 | 修改后 |
|--------|--------|--------|
| content 字段值 | `"你好！我是一个AI助手..."` | `"{\"type\":\"text\",\"text\":\"你好！我是一个AI助手...\"}"` |
| 客户端建议 | "调用 `/api/bot/chat` 后，在 UI 上展示" | "在群聊或多人私聊中 @Bot 后，在 UI 上展示" |

**目的和作用**：
- 修正 content 字段为 JSON 格式的准确表示
- 更新客户端处理建议，反映 @提及触发机制

---

### 2.5 新增：Bot 触发机制与鉴权（6.8）

**修改位置**：`api_doc.md` 第 6.8 节（新增）

**新增内容**：

1. **触发规则表格**：明确三种会话类型下 Bot 的触发条件
2. **异步处理流程图**：从消息发送到 Bot 回复的完整流程
3. **鉴权机制说明**：双层鉴权（msg_gateway 层 + work_service 层）
4. **Redis Stream 配置表**：Stream 名称、消费者组、最大长度等参数
5. **任务消息格式**：Redis Stream 中任务消息的 JSON 结构

**目的和作用**：
- 为开发者提供 Bot 触发机制的完整技术文档
- 明确鉴权流程，确保安全性
- 记录 Redis Stream 的配置参数，便于运维和扩展

---

### 2.6 WebSocket 客户端发送消息（8.1）

**修改位置**：`api_doc.md` 第 8.1 节

**修改内容**：

| 修改项 | 修改前 | 修改后 |
|--------|--------|--------|
| chat 关键字段 | `content, conversation_id, peer_account, client_seq` | 增加 `mentioned_ids` |
| 新增内容 | 无 | chat 消息详细字段表格（含 mentioned_ids 字段说明） |
| 新增内容 | 无 | 两个 chat 消息示例（群聊 @Bot、一对一 Bot 单聊） |

**目的和作用**：
- 文档化 `mentioned_ids` 字段，客户端需在发送消息时携带 @提及的用户 ID 列表
- 提供具体的消息示例，降低前端开发者的接入成本
- `mentioned_ids` 是触发 Bot 的关键数据，必须明确说明

---

### 2.7 WebSocket 服务端推送消息（8.2）

**修改位置**：`api_doc.md` 第 8.2 节

**修改内容**：

| 修改项 | 修改前 | 修改后 |
|--------|--------|--------|
| chat 关键字段 | 不含 `mentioned_ids` | 增加 `mentioned_ids` |
| 新增内容 | 无 | chat 推送消息额外字段表格（mentioned_ids + conversation_type） |

**目的和作用**：
- 服务端推送消息时携带 `mentioned_ids`，客户端可据此高亮显示被 @的用户
- 明确 `conversation_type` 字段，便于客户端区分会话类型

---

## 三、后端代码变更清单

### 3.1 msg_gateway 变更

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `msg_gateway/config/config.go` | 修改 | 添加 `redis.addr` 默认配置 |
| `msg_gateway/cmd/main.go` | 修改 | 添加 Redis 连接初始化，调用 `core.InitRedis` |
| `msg_gateway/go.mod` | 修改 | 添加 `github.com/redis/go-redis/v9`；移除 `work_service` 依赖 |
| `msg_gateway/core/hub.go` | 修改 | 添加 `InitRedis`；`triggerBots` 改为 `rdb.XAdd` 推送 Redis Stream；添加发送者鉴权 |
| `msg_gateway/rpc/work.go` | **删除** | 不再需要同步 RPC 调用 work_service |
| `msg_gateway/rpc/init.go` | 修改 | 移除 `ConnectWorkService` |

### 3.2 work_service 变更

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `work_service/internal/config/config.go` | 修改 | 添加 `redis.addr` 默认配置 |
| `work_service/main.go` | 修改 | 添加 Redis 连接和 BotTaskConsumer 启动 |
| `work_service/go.mod` | 修改 | 添加 `github.com/redis/go-redis/v9` |
| `work_service/internal/consumer/bot_task.go` | **新建** | Redis Stream 消费者，包含新消息消费、Pending 消息认领、鉴权验证、ACK 确认 |

### 3.3 chat_service 变更

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `chat_service/internal/model/chat.go` | 修改 | 添加 `MentionItem` 结构体；`TextContent` 添加 `Mentions` 字段 |
| `chat_service/internal/service/service.go` | 修改 | `AddConversationMembers` 允许向 private 会话添加成员 |

### 3.4 bot_service 变更

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `bot_service/internal/service/service.go` | 修改 | `AddBotToConversation` 支持将 Bot 拉入已有私聊；`IsBot` 返回 `bot_id` |
| `idl/bot.thrift` | 修改 | `IsBotRes` 添加 `optional bot_id` 字段 |

### 3.5 msg_gateway WebSocket 变更

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `msg_gateway/core/client.go` | 修改 | `WsMessage` 添加 `MentionedIds` 字段 |

### 3.6 前端变更

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `test_frontend/src/components/ChatPanel.tsx` | 修改 | 添加 @提及弹窗和渲染功能 |
| `test_frontend/src/components/ChatPanel.css` | 修改 | 添加 mention 标签和弹窗样式 |
| `test_frontend/src/api/client.ts` | 修改 | `buildTextContent` 函数支持 mentions 参数 |

---

## 四、操作步骤

### 4.1 应用修改

1. **拉取最新代码**：确保所有上述文件变更已合并到当前分支

2. **更新 Go 依赖**：
   ```powershell
   cd e:\Documents\Github\answer\msg_gateway; go mod tidy
   cd e:\Documents\Github\answer\kitex_service\work_service; go mod tidy
   ```

3. **确保 Redis 服务运行**：
   ```powershell
   redis-cli ping
   ```
   预期返回 `PONG`。如未安装 Redis，需先安装并启动：
   ```powershell
   redis-server
   ```

4. **重新编译并启动服务**：
   ```powershell
   # 启动顺序：etcd → chat_service → bot_service → work_service → msg_gateway → api_gateway
   ```

5. **前端更新**：
   ```powershell
   cd e:\Documents\Github\answer\test_frontend; npm install; npm run dev
   ```

### 4.2 验证修改效果

#### 验证 1：@提及功能

1. 创建群聊，添加 Bot 和多个用户
2. 在群聊中输入 `@`，应弹出成员列表
3. 选择一个 Bot 进行 @提及，发送消息
4. 验证消息的 `content` 字段包含 `mentions` 数组
5. 验证消息的 `mentioned_ids` 字段包含被 @的 Bot 用户 ID

#### 验证 2：Bot 异步回复

1. 在群聊中 @Bot 发送消息
2. 验证 msg_gateway 日志显示 `XADD bot:task:stream` 推送成功
3. 验证 work_service 日志显示消费任务并开始处理
4. 验证 Bot 回复通过 WebSocket 推送到客户端
5. 验证用户发送消息后无需等待 Bot 回复即可继续操作

#### 验证 3：鉴权机制

1. 使用非会话成员身份尝试触发 Bot（可通过修改 WebSocket 消息中的 sender_id 模拟）
2. 验证 msg_gateway 日志显示"发送者不在会话成员列表中，拒绝触发Bot"
3. 验证 Redis Stream 中未收到该任务
4. 或：直接向 Redis Stream 推送伪造任务（sender_id 不在会话成员中）
5. 验证 work_service 日志显示"鉴权失败: 发送者不在会话成员列表中"

#### 验证 4：将 Bot 拉入已有私聊

1. 创建用户5和用户6的私聊
2. 调用 `/api/bot/add_to_conversation`，传入 `conversation_id` 和 `conversation_type=1`
3. 验证 Bot 成功加入私聊，会话成员列表包含 Bot
4. 在私聊中 @Bot 发送消息，验证 Bot 正常回复
5. 验证会话类型仍为 `1`（private）

#### 验证 5：一对一 Bot 单聊

1. 调用 `/api/bot/add_to_conversation`，传入 `conversation_id=0` 和 `conversation_type=1`
2. 在创建的单聊中直接发送消息（无需 @Bot）
3. 验证 Bot 自动回复

### 4.3 注意事项

1. **Redis 依赖**：msg_gateway 和 work_service 现在都依赖 Redis，启动前必须确保 Redis 服务可用
2. **Redis Stream 持久化**：Redis Stream 中的消息在 Redis 重启后会丢失（除非配置了 AOF/RDB 持久化）。生产环境建议开启 Redis 持久化
3. **消费者组**：首次启动 work_service 时会自动创建消费者组 `bot-worker-group`，如果 Stream 不存在也会自动创建
4. **Pending 消息**：如果 work_service 异常退出，未 ACK 的消息会在 5 分钟后被重新认领处理，确保消息不丢失
5. **消息顺序**：Redis Stream 保证消息的全局顺序，但多个 Bot 任务的执行顺序取决于消费速度
6. **history 参数移除**：`/api/bot/chat` 接口不再接受 `history` 参数，历史对话由 work_service 内部管理。如果有外部调用依赖此参数，需要移除
7. **content 字段格式变更**：消息的 `content` 字段现在是 JSON 格式而非纯文本，前端需要先 `JSON.parse` 再读取 `text` 和 `mentions` 字段
8. **IsBotRes 变更**：`IsBot` RPC 响应新增 `bot_id` 字段（optional），现有调用方无需修改，但可利用此字段获取 Bot ID

---

## 五、影响范围

| 影响面 | 说明 |
|--------|------|
| 前端 | 需要更新消息发送逻辑（携带 mentioned_ids）、消息渲染逻辑（解析 content JSON、高亮 @提及） |
| 后端 - msg_gateway | triggerBots 改为推 Redis Stream，移除 work_service RPC 依赖 |
| 后端 - work_service | 新增 Redis Stream 消费者，启动时自动开始消费 |
| 后端 - chat_service | AddConversationMembers 允许向 private 会话添加成员 |
| 后端 - bot_service | AddBotToConversation 支持已有私聊场景；IsBot 返回 bot_id |
| 运维 | 需确保 Redis 服务可用，建议配置持久化 |
| API 兼容性 | `/api/bot/chat` 移除 history 参数（不兼容变更），content 字段格式变更（不兼容变更） |

---

## 六、函数调用链与调用逻辑详细说明

### 6.1 主调用链：用户发送消息 → Bot 异步回复

这是系统中最核心的端到端调用链，从用户在 WebSocket 中发送消息到 Bot 回复推送给所有会话成员。

```
[客户端 WebSocket]
    │ type="chat", conversation_id, content, mentioned_ids, client_seq
    ▼
[Client.ReadMessage()]                          msg_gateway/core/client.go
    │ json.Unmarshal → WsMessage
    │ 写入 manager.Message channel
    ▼
[Manager.Start()] → handleMessage()             msg_gateway/core/hub.go
    │ 根据 wsMsg.Type 分发：chat/mark_read/typing/recall/edit/sync
    ▼
[Manager.handleMessage()]                       msg_gateway/core/hub.go:139
    │ 校验 content 非空、conversation_id/peer_account 至少一个非空
    │ 若有 peer_account → rpc.GetUserIdMap() 转换为 peerID
    ▼
[rpc.SendMessage()]                             msg_gateway/rpc/chat.go
    │ Kitex RPC → chat_service.SendMessage
    │ 请求：SendMessageReq{SenderId, ConversationId, PeerId, Content, ClientSeq}
    │ 返回：SendMessageRes{MsgId, Seq, Timestamp, ConversationId, ConversationType, MemberIds, Content}
    ▼
[chat_service.SendMessage()]                    chat_service/internal/service/service.go:80
    │ 1. NormalizeContent(content) → 统一为 JSON 格式
    │ 2. 确定 convID（conversationID != 0 直接用；否则 GetOrCreatePrivateConversation）
    │ 3. GetConversationMembers(convID) → 校验 senderID 是否为成员
    │ 4. 若群聊 → rpc.CheckMuted() 检查禁言
    │ 5. Redis SetNX 消息去重（dedupeKey = msg:dedup:{senderID}:{clientSeq}）
    │ 6. 生成 msgID（雪花算法）、IncrConvMaxSeq 获取 seq
    │ 7. dao.CreateMessage() 写入 PostgreSQL
    │ 8. Redis 设置撤回窗口键 recall:msg:{msgID}，TTL=2min
    │ 返回：SendMessageResult{MsgID, Seq, Timestamp, ConversationID, ConversationType, MemberIDs, Content}
    ▼
[Manager.handleMessage() 续]                    msg_gateway/core/hub.go:249
    │ 构造 chatMsg（WsMessage{type="chat", ...}）
    │ sender.Send(chatMsg) → 发送者自己收到确认
    │ go manager.pushToMembers() → 异步推送给其他在线成员
    │ go manager.triggerBots() → 异步触发 Bot 处理
    ▼
┌──────────────────────────────────────────────────────────────┐
│  分支 A：pushToMembers()                                      │
│  [Manager.pushToMembers()]        hub.go:261                  │
│      │ rpc.GetOnlineStatus() → 查询成员在线状态               │
│      │ 本地在线 → client.Send(chatMsg)                        │
│      │ 远端网关 → pushToGateway() → HTTP POST /push           │
│      │   签名：HMAC-SHA256(pushSecret, body)                  │
│      │   对端：HandlePush() → 验签 → client.Send()            │
│                                                               │
│  分支 B：triggerBots()                                        │
│  [Manager.triggerBots()]          hub.go:780                  │
│      │ 详见 6.2 节                                            │
└──────────────────────────────────────────────────────────────┘
```

### 6.2 triggerBots 调用链：Bot 触发与 Redis Stream 推送

```
[Manager.triggerBots(senderID, conversationID, convType, memberIDs, mentionedIDs, content)]
    │                                            hub.go:780
    │
    │ 步骤1：鉴权 — 验证发送者是会话成员
    │ 遍历 memberIDs，检查 senderID 是否在其中
    │ 若不在 → klog.Error("拒绝触发Bot") → return
    │
    │ 步骤2：构建 mentionedSet
    │ mentionedIDs → map[int64]struct{}  （O(1) 查找）
    │
    │ 步骤3：遍历 memberIDs（跳过 senderID 自身）
    │ 对每个 memberID：
    │   ▼
    │ [rpc.IsBot(ctx, memberID)]                  msg_gateway/rpc/bot.go:52
    │   │ Kitex RPC → bot_service.IsBot
    │   │ 请求：IsBotReq{UserId: memberID}
    │   │ 返回：IsBotRes{IsBot: bool, BotId: *int64}
    │   │ 若 RPC 失败 → klog.Error → continue
    │   │ 若 !isBot → continue
    │   ▼
    │ [触发条件判断]
    │   │ convType == 2 (群聊)：
    │   │   必须 isMentioned（memberID 在 mentionedSet 中），否则 continue
    │   │ convType == 1 (私聊)：
    │   │   若 len(memberIDs) == 2 → continue（一对一 Bot 单聊，无需触发）
    │   │   若 !isMentioned → continue（多人私聊需 @Bot）
    │   │ 其他 convType → continue
    │   ▼
    │ [构造任务并推送到 Redis Stream]
    │   task = {bot_id, conversation_id, sender_id, content}  （值均为 string）
    │   taskJSON = json.Marshal(task)
    │   ▼
    │ [rdb.XAdd(ctx, &XAddArgs{                        go-redis/v9
    │     Stream: "bot:task:stream",
    │     Values: {"task": string(taskJSON)},
    │     MaxLen: 10000, Approx: true,
    │ })]
    │   │ 成功 → 任务进入 Redis Stream
    │   │ 失败 → klog.Error("推送Bot任务到Redis Stream失败")
    │
    │ 返回（不等待 Bot 处理结果）
```

**参数传递说明**：

| 参数 | 来源 | 类型 | 说明 |
|------|------|------|------|
| senderID | clientMsg.Client.UserId | int64 | 发送者用户 ID |
| conversationID | resp.ConversationId | int64 | chat_service 返回的会话 ID |
| convType | resp.ConversationType | int16 | 1=私聊，2=群聊 |
| memberIDs | resp.MemberIds | []int64 | chat_service 返回的会话成员列表 |
| mentionedIDs | wsMsg.MentionedIds | []int64 | 客户端 WebSocket 消息中携带的 @提及用户 ID 列表 |
| content | pushContent | string | 消息内容（经 NormalizeContent 后的 JSON） |

**异常处理**：

| 异常场景 | 处理方式 |
|----------|----------|
| 发送者不在成员列表 | 记录错误日志，直接 return，不推送任务 |
| rpc.IsBot 调用失败 | 记录错误日志，continue 跳过该成员 |
| rdb.XAdd 推送失败 | 记录错误日志，任务丢失（依赖 Pending 检查机制兜底） |

### 6.3 BotTaskConsumer 调用链：Redis Stream 消费与 Bot 回复

```
[work_service main.go]
    │ rdb = connect.ConnectRedis()
    │ botConsumer = NewBotTaskConsumer(rdb, workService)
    │ go botConsumer.Start(ctx)
    ▼
[BotTaskConsumer.Start(ctx)]                    consumer/bot_task.go:51
    │ ensureGroup() → XGroupCreateMkStream 创建消费者组（幂等）
    │ go consumeNew(ctx)     → 消费新消息
    │ go consumePending(ctx) → 消费超时未确认的消息
    ▼
┌──────────────────────────────────────────────────────────────┐
│  消费者 A：consumeNew()                                       │
│  [BotTaskConsumer.consumeNew(ctx)]            bot_task.go:62  │
│      │ 无限循环：                                             │
│      │ rdb.XReadGroup(XReadGroupArgs{                         │
│      │     Group: "bot-worker-group",                         │
│      │     Consumer: "worker-1",                              │
│      │     Streams: ["bot:task:stream", ">"],                 │
│      │     Count: 1, Block: 5s                                │
│      │ })                                                     │
│      │ err == redis.Nil → 无新消息，继续循环                   │
│      │ err != nil → 记录错误，sleep 1s，继续循环               │
│      │ 成功 → 遍历 stream.Messages → processMessage()         │
│                                                               │
│  消费者 B：consumePending()                                   │
│  [BotTaskConsumer.consumePending(ctx)]        bot_task.go:89  │
│      │ 每 5 分钟执行一次：                                    │
│      │ rdb.XPendingExt(XPendingExtArgs{                       │
│      │     Stream: "bot:task:stream",                         │
│      │     Group: "bot-worker-group",                         │
│      │     Idle: 5min, Count: 10                              │
│      │ })                                                     │
│      │ 对每条 pending 消息：                                   │
│      │ rdb.XClaim() → 认领超时消息                            │
│      │ → processMessage()                                     │
└──────────────────────────────────────────────────────────────┘
    ▼
[BotTaskConsumer.processMessage(ctx, msg)]       bot_task.go:119
    │
    │ 步骤1：解析任务
    │ msg.Values["task"] → taskJSON (string)
    │ json.Unmarshal(taskJSON) → BotTask{BotID, ConversationID, SenderID, Content}
    │ 解析失败 → ack(msg.ID) → return
    │
    │ 步骤2：鉴权 — 二次验证发送者是会话成员
    │ ▼
    │ [validateTask(ctx, &task)]                  bot_task.go:143
    │   │ rpc.GetConversationMembers(ctx, task.ConversationID)
    │   │   │ Kitex RPC → chat_service.GetConversationMembers
    │   │   │ 请求：GetConversationMembersReq{ConversationId}
    │   │   │ 返回：GetConversationMembersRes{MemberIds}
    │   │   │ RPC 失败 → return false（不 ACK，下次重试）
    │   │ 遍历 members 检查 task.SenderID 是否在其中
    │   │ 不在 → klog.Error("鉴权失败") → return false
    │   │ 在 → return true
    │   ▼
    │ validateTask 返回 false → ack(msg.ID) → return
    │
    │ 步骤3：调用 workService.HandleMessage
    │ ▼
    │ [WorkService.HandleMessage(ctx, botId, conversationId, senderId, content, nil)]
    │                                            service/service.go:19
    │   │
    │   │ 3a. 获取 Bot 配置
    │   │ rpc.GetBotConfig(ctx, botId)
    │   │   │ Kitex RPC → bot_service.GetBotConfig
    │   │   │ 请求：GetBotConfigReq{BotId}
    │   │   │ 返回：BotConfig{ApiKey, Model, SystemPrompt, UserID}
    │   │   │ 若 ApiKey 为空 → return false, error
    │   │   ▼
    │   │
    │   │ 3b. 验证 Bot 在会话中
    │   │ 若 botCfg.UserID > 0：
    │   │   rpc.GetConversationMembers(ctx, conversationId)
    │   │   遍历检查 botCfg.UserID 是否在成员列表中
    │   │   不在 → return false, error("请先将Bot拉入会话")
    │   │
    │   │ 3c. 异步调用 LLM（go func）
    │   │ go func() {
    │   │   ▼
    │   │   [llmClient.Chat(apiKey, model, systemPrompt, chatHistory, content)]
    │   │                                            llm/client.go:48
    │   │     │ 构造 messages：[systemPrompt] + history + [userContent]
    │   │     │ POST {BaseURL}/chat/completions
    │   │     │ Header: Authorization: Bearer {apiKey}
    │   │     │ Body: {model, messages, temperature: 0.7}
    │   │     │ HTTP 调用 LLM API
    │   │     │ 成功 → 返回 chatResp.Choices[0].Message.Content
    │   │     │ 失败 → return "", error
    │   │   ▼
    │   │   [rpc.SendMessage(sendCtx, &SendMessageReq{    work_service/rpc/chat.go
    │   │       SenderId: botCfg.UserID,
    │   │       ConversationId: conversationId,
    │   │       Content: result,           // LLM 回复内容
    │   │   })]
    │   │     │ Kitex RPC → chat_service.SendMessage
    │   │     │ 消息入库（Bot 作为发送者，消息写入 PostgreSQL）
    │   │     │ 返回 SendMessageRes{MsgId, Seq, MemberIds, ...}
    │   │     │ 注意：此 RPC 调用会触发 chat_service 的完整消息流程
    │   │     │       但不会再次触发 msg_gateway 的 triggerBots（因为是 RPC 直调）
    │   │   ▼
    │   │   消息入库后，msg_gateway 的 WebSocket 推送由 chat_service 的
    │   │   消息通知机制触发（或由 Bot 的 SendMessage 返回后由 work_service
    │   │   通过其他方式推送）。当前实现中，Bot 回复消息通过 chat_service
    │   │   的 SendMessage RPC 入库，但 WebSocket 推送需要依赖
    │   │   msg_gateway 的在线状态感知机制。
    │   │ }()
    │   │
    │   │ return true, nil（立即返回，不等待 LLM 结果）
    │   ▼
    │
    │ 步骤4：ACK 确认
    │ [ack(ctx, msg.ID)]                          bot_task.go:163
    │   rdb.XAck(ctx, "bot:task:stream", "bot-worker-group", msgID)
    │   失败 → klog.Error（不影响已完成的处理）
```

**参数传递说明**：

| 阶段 | 参数 | 来源 | 类型 | 说明 |
|------|------|------|------|------|
| Redis Stream 消息 | task.bot_id | triggerBots 构造 | string | Bot ID（字符串格式） |
| Redis Stream 消息 | task.conversation_id | triggerBots 构造 | string | 会话 ID（字符串格式） |
| Redis Stream 消息 | task.sender_id | triggerBots 构造 | string | 发送者用户 ID（字符串格式） |
| Redis Stream 消息 | task.content | pushContent | string | 消息内容（JSON 格式） |
| validateTask | conversationId | task.ConversationID (int64) | int64 | 用于查询会话成员 |
| HandleMessage | botId | task.BotID (int64) | int64 | 用于获取 Bot 配置 |
| HandleMessage | conversationId | task.ConversationID | int64 | 用于验证 Bot 在会话中 |
| HandleMessage | senderId | task.SenderID | int64 | 原始发送者（当前未使用） |
| HandleMessage | content | task.Content | string | 用户消息内容 |
| llmClient.Chat | apiKey | botCfg.ApiKey | string | LLM API Key |
| llmClient.Chat | model | botCfg.Model | string | LLM 模型名称 |
| llmClient.Chat | systemPrompt | botCfg.SystemPrompt | string | 系统提示词 |
| llmClient.Chat | history | nil（当前未传入） | []ChatMessage | 历史对话 |
| llmClient.Chat | userContent | content | string | 用户消息内容 |
| rpc.SendMessage | SenderId | botCfg.UserID | int64 | Bot 的用户 ID |
| rpc.SendMessage | ConversationId | conversationId | int64 | 会话 ID |
| rpc.SendMessage | Content | LLM 返回的 result | string | Bot 回复内容 |

**异常处理**：

| 异常场景 | 处理方式 | 是否 ACK |
|----------|----------|----------|
| task 字段缺失 | 记录错误日志，ACK 丢弃 | ✅ |
| JSON 解析失败 | 记录错误日志，ACK 丢弃 | ✅ |
| validateTask 中 RPC 失败 | 记录错误日志，不 ACK（等待重试） | ❌ |
| 发送者不在会话成员列表 | 记录错误日志，ACK 丢弃 | ✅ |
| GetBotConfig 失败 | HandleMessage 返回 error，记录日志，ACK | ✅ |
| Bot 不在会话中 | HandleMessage 返回 error，记录日志，ACK | ✅ |
| LLM 调用失败 | goroutine 内记录错误日志，ACK 已执行 | ✅（先于 LLM） |
| SendMessage 失败 | goroutine 内记录错误日志，ACK 已执行 | ✅（先于 SendMessage） |
| XAck 失败 | 记录错误日志，不影响处理结果 | — |

### 6.4 AddBotToConversation 调用链：将 Bot 拉入会话

```
[API Gateway] POST /api/bot/add_to_conversation
    │ JWT 鉴权 → 获取 operatorId
    │ 请求体：{bot_id, conversation_id, conversation_type}
    ▼
[api_gateway handler]
    │ Kitex RPC → bot_service.AddBotToConversation
    │ 请求：AddBotToConversationReq{OperatorId, BotId, ConversationId, ConversationType}
    ▼
[bot_service.AddBotToConversation(ctx, operatorId, botId, conversationId, convType)]
    │                                            bot_service/internal/service/service.go:183
    │
    │ 步骤1：查询 Bot 信息
    │ dao.GetBot(botId) → bot{ID, CreatorID, UserID, ...}
    │ bot.ID == 0 → return error("bot不存在")
    │
    │ 步骤2：权限校验
    │ bot.CreatorID != operatorId → return error("只有Bot创建者才能将Bot拉入会话")
    │ bot.UserID == 0 → return error("bot用户记录异常，缺少user_id")
    │
    │ 步骤3：根据会话类型分支处理
    │ ▼
    │ ┌─ convType == 2 (群聊) ──────────────────────────────────┐
    │ │ rpc.AddConversationMembers(ctx, conversationId, []int64{bot.UserID})  │
    │ │   │ Kitex RPC → chat_service.AddConversationMembers                    │
    │ │   │ 请求：AddConversationMembersReq{ConversationId, MemberIds}         │
    │ │   ▼                                                                     │
    │ │ [chat_service.AddConversationMembers(ctx, conversationID, memberIDs)]   │
    │ │   │ conversationDao.GetConversationInfo(ctx, conversationID)            │
    │ │   │ 校验会话存在且类型为 group 或 private                               │
    │ │   │ conversationDao.AddMembers(ctx, conversationID, memberIDs)          │
    │ │   │ → INSERT INTO conversation_member                                   │
    │ │ return conversationId, nil                                              │
    │ └─────────────────────────────────────────────────────────────────────────┘
    │
    │ ┌─ convType == 1 (私聊) ─────────────────────────────────┐
    │ │                                                         │
    │ │ 情况 A：conversationId == 0（创建新 Bot 单聊）          │
    │ │   rpc.GetOrCreatePrivateConversation(ctx, operatorId,   │
    │ │       bot.UserID)                                       │
    │ │     │ Kitex RPC → chat_service                          │
    │ │     │ 查找已有私聊或创建新私聊                           │
    │ │     │ 返回 convID                                       │
    │ │   return convID, nil                                    │
    │ │                                                         │
    │ │ 情况 B：conversationId != 0（拉入已有私聊）             │
    │ │   rpc.AddConversationMembers(ctx, conversationId,       │
    │ │       []int64{bot.UserID})                              │
    │ │     │ 同群聊分支的 AddConversationMembers               │
    │ │     │ 会话类型校验允许 private 类型                     │
    │ │   return conversationId, nil                            │
    │ └─────────────────────────────────────────────────────────┘
    │
    │ 其他 convType → return error("不支持的会话类型")
    ▼
[bot_service handler]
    │ 返回 AddBotToConversationRes{Success: true, ConversationId: &convID}
    ▼
[API Gateway]
    │ 返回 JSON: {code: 0, data: {success: true, conversation_id: convID}}
```

**参数传递说明**：

| 参数 | 来源 | 类型 | 说明 |
|------|------|------|------|
| operatorId | JWT Token 解析 | int64 | 操作者用户 ID |
| botId | 请求体 | int64 | 目标 Bot ID |
| conversationId | 请求体 | int64 | 会话 ID（0 表示新建） |
| conversationType | 请求体 | int16 | 1=私聊，2=群聊 |
| bot.UserID | dao.GetBot 返回 | int64 | Bot 在 users 表中的用户 ID |

**异常处理**：

| 异常场景 | 处理方式 |
|----------|----------|
| Bot 不存在 | 返回 error("bot不存在") |
| 非创建者操作 | 返回 error("只有Bot创建者才能将Bot拉入会话") |
| Bot 缺少 user_id | 返回 error("bot用户记录异常，缺少user_id") |
| 会话不存在 | chat_service 返回 error("会话不存在") |
| 不支持的会话类型 | 返回 error("不支持的会话类型") |
| AddConversationMembers 失败 | 返回 error("将Bot加入群聊/私聊会话失败: ...") |
| GetOrCreatePrivateConversation 失败 | 返回 error("创建Bot单聊会话失败: ...") |

### 6.5 IsBot 调用链：判断用户是否为 Bot

```
[rpc.IsBot(ctx, userId)]                        msg_gateway/rpc/bot.go:52
    │ Kitex RPC → bot_service.IsBot
    │ 请求：IsBotReq{UserId: userId}
    ▼
[bot_service.IsBot(userId)]                     bot_service/internal/service/service.go:167
    │ dao.GetBotByUserId(userId) → bot{ID, ...}
    │ bot.ID == 0 → return false, 0, nil
    │ bot.ID != 0 → return true, bot.ID, nil
    ▼
[返回]
    │ IsBotRes{IsBot: bool, BotId: *int64}
    │ msg_gateway 侧：
    │   resp.IsSetBotId() → botId = resp.GetBotId()
    │   return isBot, botId, err
```

**调用场景**：`triggerBots` 中遍历会话成员时，对每个非发送者成员调用 `IsBot` 判断是否为 Bot。

### 6.6 函数依赖关系总图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        msg_gateway                                   │
│                                                                      │
│  Client.ReadMessage()                                                │
│       │                                                              │
│       ▼                                                              │
│  Manager.Start()                                                     │
│       │                                                              │
│       ▼                                                              │
│  Manager.handleMessage()                                             │
│       │                                                              │
│       ├──→ rpc.SendMessage() ──────────→ chat_service.SendMessage   │
│       │         │                         (Kitex RPC)                │
│       │         │ 返回: MsgId, Seq, MemberIds, ConversationType     │
│       │         │ Content (normalized)                                │
│       │         ▼                                                    │
│       │    sender.Send(chatMsg)  ← 发送者确认                        │
│       │         │                                                    │
│       │    ┌────┴────┐                                               │
│       │    ▼         ▼                                               │
│       │ pushToMembers()  triggerBots()                               │
│       │    │                │                                        │
│       │    │                ├── rpc.IsBot() ──→ bot_service.IsBot    │
│       │    │                │     (Kitex RPC)                        │
│       │    │                │     返回: isBot, botId                  │
│       │    │                │                                        │
│       │    │                └── rdb.XAdd() ──→ Redis Stream          │
│       │    │                     "bot:task:stream"                    │
│       │    │                                                        │
│       │    ├── rpc.GetOnlineStatus() ──→ chat_service               │
│       │    ├── client.Send()           ← 本地推送                   │
│       │    └── pushToGateway()         ← 跨网关推送                 │
│       │                                                              │
└───────┼──────────────────────────────────────────────────────────────┘
        │
        │  Redis Stream: bot:task:stream
        │
┌───────┼──────────────────────────────────────────────────────────────┐
│       ▼           work_service                                       │
│                                                                      │
│  BotTaskConsumer.Start()                                             │
│       │                                                              │
│       ├── ensureGroup()     → XGroupCreateMkStream                   │
│       ├── consumeNew()      → XReadGroup (阻塞读取新消息)            │
│       └── consumePending()  → XPendingExt + XClaim (超时认领)        │
│                │                                                     │
│                ▼                                                     │
│       processMessage()                                               │
│          │                                                           │
│          ├── json.Unmarshal(task) → BotTask                          │
│          │                                                           │
│          ├── validateTask()                                          │
│          │     └── rpc.GetConversationMembers() ──→ chat_service     │
│          │           (Kitex RPC)                                     │
│          │           返回: MemberIds                                  │
│          │           校验: senderID ∈ members                        │
│          │                                                           │
│          ├── WorkService.HandleMessage()                             │
│          │     │                                                     │
│          │     ├── rpc.GetBotConfig() ──→ bot_service.GetBotConfig   │
│          │     │     (Kitex RPC)                                     │
│          │     │     返回: ApiKey, Model, SystemPrompt, UserID       │
│          │     │                                                     │
│          │     ├── rpc.GetConversationMembers() ──→ chat_service     │
│          │     │     校验: botCfg.UserID ∈ members                   │
│          │     │                                                     │
│          │     └── go func() {                                       │
│          │           llmClient.Chat() ──→ LLM API (HTTP)             │
│          │             返回: result (Bot 回复内容)                    │
│          │             │                                              │
│          │           rpc.SendMessage() ──→ chat_service.SendMessage  │
│          │             SenderId = botCfg.UserID                      │
│          │             Content = result                               │
│          │         }()                                                │
│          │                                                           │
│          └── ack() → XAck                                            │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 6.7 关键函数触发条件与执行顺序

| 序号 | 函数 | 所在服务 | 触发条件 | 执行顺序 | 依赖 |
|------|------|----------|----------|----------|------|
| 1 | Client.ReadMessage | msg_gateway | WebSocket 收到消息 | 1 | WebSocket 连接 |
| 2 | Manager.handleMessage | msg_gateway | manager.Message channel 收到消息 | 2 | Client.ReadMessage |
| 3 | rpc.SendMessage | msg_gateway | handleMessage 中 chat 类型消息 | 3 | Manager.handleMessage |
| 4 | chat_service.SendMessage | chat_service | RPC 调用 | 4 | rpc.SendMessage |
| 5 | sender.Send | msg_gateway | SendMessage RPC 成功 | 5 | chat_service.SendMessage 返回 |
| 6 | pushToMembers | msg_gateway | SendMessage RPC 成功（异步） | 5（并行） | chat_service.SendMessage 返回 |
| 7 | triggerBots | msg_gateway | SendMessage RPC 成功（异步） | 5（并行） | chat_service.SendMessage 返回 |
| 8 | rpc.IsBot | msg_gateway | triggerBots 遍历成员时 | 8 | triggerBots |
| 9 | rdb.XAdd | msg_gateway | triggerBots 判断需要触发 Bot | 9 | rpc.IsBot 返回 isBot=true |
| 10 | BotTaskConsumer.consumeNew | work_service | Redis Stream 有新消息 | 10 | rdb.XAdd |
| 11 | BotTaskConsumer.processMessage | work_service | consumeNew/consumePending 收到消息 | 11 | consumeNew |
| 12 | validateTask | work_service | processMessage 解析任务成功 | 12 | processMessage |
| 13 | rpc.GetConversationMembers (validateTask) | work_service | validateTask 调用 | 13 | validateTask |
| 14 | WorkService.HandleMessage | work_service | validateTask 通过 | 14 | validateTask |
| 15 | rpc.GetBotConfig | work_service | HandleMessage 第一步 | 15 | HandleMessage |
| 16 | rpc.GetConversationMembers (HandleMessage) | work_service | HandleMessage 验证 Bot 在会话中 | 16 | rpc.GetBotConfig |
| 17 | llmClient.Chat | work_service | HandleMessage 异步 goroutine | 17 | rpc.GetBotConfig |
| 18 | rpc.SendMessage (Bot回复) | work_service | llmClient.Chat 成功 | 18 | llmClient.Chat |
| 19 | ack | work_service | processMessage 完成（无论成功失败） | 19 | HandleMessage |

### 6.8 Redis Stream 消费者可靠性机制

```
                    ┌─────────────────────────────────┐
                    │     Redis Stream                 │
                    │   bot:task:stream                │
                    │                                  │
                    │  Entry 1 ──→ Entry 2 ──→ Entry 3│
                    └────────┬────────────────────────┘
                             │
                    XReadGroup (">" 读取新消息)
                             │
                             ▼
                    ┌─────────────────┐
                    │  processMessage │
                    │    成功处理      │
                    └────────┬────────┘
                             │
                          XAck ──→ 消息从 Pending 列表移除
                             │
                             ▼
                         完成 ✅

    异常场景：
                    ┌─────────────────┐
                    │  processMessage │
                    │    处理失败      │
                    │  (RPC 超时等)    │
                    └────────┬────────┘
                             │
                      不执行 XAck
                             │
                             ▼
                    消息留在 Pending 列表
                             │
                    ┌────────┴────────┐
                    │ consumePending  │
                    │  每 5 分钟执行   │
                    │  XPendingExt    │ ← 查找 Idle > 5min 的消息
                    │  XClaim         │ ← 认领超时消息
                    └────────┬────────┘
                             │
                             ▼
                    重新 processMessage
                             │
                          XAck ──→ 完成 ✅
```

**可靠性保证**：

1. **至少一次消费**：消息只有被 XAck 后才从 Pending 列表移除，未 ACK 的消息会被重新消费
2. **超时重试**：Pending 消息超过 5 分钟未 ACK，会被 consumePending 协程认领并重新处理
3. **消费者组**：使用消费者组确保每条消息只被一个消费者处理
4. **近似裁剪**：`MaxLen: 10000, Approx: true` 防止 Stream 无限增长，但可能裁剪掉已消费的旧消息（不影响功能，因为已 ACK 的消息无需保留）
5. **阻塞读取**：`Block: 5s` 避免空轮询消耗 CPU
