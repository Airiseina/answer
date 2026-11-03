# 本地启动与功能验收指引

> 适用对象：需要在本地拉起 Answer IM 并验收「个人 Skill」「多端在线」「Bot 任务并发配置」三项能力的开发者。
> 环境基准：Windows + PowerShell 7（`pwsh`）、Docker Desktop、Go 1.26、Node 20+；macOS / Linux 把 PowerShell 片段换成对应 shell 语法即可。
> 说明：本文命令均在仓库根目录执行；Kitex 服务必须在**各自服务目录**下启动，因为 `pkg/config` 只从当前工作目录读取 `config.yaml`。

---

## 一、端口与依赖表

| 组件 | 地址 | 启动方式 | 缺失后果 |
| --- | --- | --- | --- |
| `api_gateway` | `:1234` | `go run` | 前端全部 HTTP 接口不可用 |
| `msg_gateway` | `:8082`（见下文端口冲突） | `go run` | 收不到实时消息 / 多端在线无法验收 |
| `user_service` | `:4320` | `go run` | 注册登录失败 |
| `group_service` | `:4321` | `go run` | 群组功能失败 |
| `chat_service` | `:4322` | `go run` | 会话 / 消息失败 |
| `bot_service` | `:4323` | `go run` | **Skill 上传、绑定全部失败** |
| `work_service` | `:4324` | `go run` | Bot 不回复、总结不生成 |
| `knowledge_service` | `:4326` | `go run` | **Skill 知识库无法创建与解析** |
| 前端 dev | `:3000` | `npm run dev` | 无 UI |

依赖容器（`deploy/docker-compose.yml`）：

| 容器 | 宿主机端口 | 谁在用 | 是否必需 |
| --- | --- | --- | --- |
| `mysql` | 3306 | user/group/bot/work/knowledge | 必需 |
| `postgres` | 5432 | chat | 必需 |
| `etcd` | 2379 | 全部 Kitex 服务注册发现 | 必需 |
| `garnet` | 6379 | chat/msg/work | 必需（缺失时 msg 静默降级） |
| `kafka-1` / `kafka-2` | 9094 / 9095 | msg/work/knowledge | 必需 |
| `seaweedfs-master/volume/filer` | 9333 / **8081** / 8888 | api_gateway、bot_service、knowledge | 必需（filer 不可达会直接 Fatal） |
| `qdrant` | 6333 / 6334 | knowledge（向量库） | 必需 |
| `meilisearch` | 7700 | knowledge（BM25） | 必需 |
| `neo4j` | 7474 / 7687 | knowledge（图谱） | 建议（缺失时图谱检索降级） |
| `clickhouse` | 8123 / 9000 | chat（冷库） | 可选（连不上只告警） |
| `traefik` | 80 / 8080 | 全容器编排 | 本地 go run 可不起 |

> **端口冲突（重要）**：`seaweedfs-volume` 占用宿主机 **8081**，因此本地 `msg_gateway` 使用 **8082**（`msg_gateway/config.yaml` 已如此配置）。
> 但 8082 在部分机器上会被 QQ 等客户端占用（本项目实测到 `QQ.exe` 占用 8082）。启动前先确认：

```powershell
Get-NetTCPConnection -LocalPort 8082 -State Listen -ErrorAction SilentlyContinue |
  ForEach-Object { Get-Process -Id $_.OwningProcess | Select-Object Id, ProcessName }
```

若被占用，把端口改成 8090，并**同步三处**（缺一处就连不上 WS）：

1. `msg_gateway/config.yaml` → `gateway.addr: "localhost:8090"`
2. `web/test_frontend/vite.config.ts` → `/ws` 的 `target: 'ws://localhost:8090'`
3. 若你手动指定了前端 WS 地址，也需一并修改

---

## 二、准备环境变量

Compose 从 `deploy/.env` 读取变量；示例文件里的取值与各服务 `config.yaml` 的默认值对齐，**先复制再启动**可避免 Meilisearch / ClickHouse 密码不一致：

```powershell
Copy-Item deploy\.env.example deploy\.env -Force
```

需要按需修改的两项：

| 变量 | 说明 |
| --- | --- |
| `EMBEDDING_API_KEY` | 知识库向量化密钥（火山方舟 / OpenAI 兼容端点）。**留空时文档解析会失败**，Skill 只能验到「文件已入库」 |
| `MEILISEARCH_API_KEY` | 默认 `Airiseina_answer_key`，与 `knowledge_service/config.yaml` 中的 `meilisearch.api_key` 必须一致 |

`knowledge_service` 还有一项运行期配置，建议用环境变量注入（Viper 会把 `.` 映射为 `_`）：

```powershell
$env:EMBEDDING_API_KEY = "sk-你的向量化密钥"
```

---

## 三、启动依赖容器

### 方式 A：只起验收必需的依赖（推荐，最快）

```powershell
docker compose -f deploy/docker-compose.yml up -d `
  mysql postgres etcd garnet kafka-1 kafka-2 `
  qdrant meilisearch neo4j clickhouse `
  seaweedfs-master seaweedfs-volume seaweedfs-filer
```

### 方式 B：使用仓库构建脚本（含 MCP 服务与 RAGAS 评估，较重）

```powershell
go run build.go infra-up
```

等待全部容器健康（首次拉镜像需要几分钟）：

```powershell
docker ps --format "{{.Names}}`t{{.Status}}" | Select-String "answer_"
```

至少要看到 `answer_mysql`、`answer_postgres`、`answer_etcd`、`answer_garnet`、`answer_kafka-1`、`answer_qdrant`、`answer_meilisearch`、`answer_seaweedfs_filer` 处于 `Up`。

---

## 四、启动 Go 服务

### 方式 A：多终端逐个启动（便于看日志，推荐首次验收）

在每个新终端里执行一条（顺序建议先基础设施类服务，再网关）：

```powershell
go run build.go run-user-service
go run build.go run-group-service
go run build.go run-chat-service
go run build.go run-knowledge-service
go run build.go run-bot-service
go run build.go run-work-service
go run build.go run-api-gateway
go run build.go run-msg-gateway
```

### 方式 B：后台批量启动（日志落到 `.scratch/logs`）

```powershell
$logs = ".scratch\logs"; New-Item -ItemType Directory -Force $logs | Out-Null
$services = @("user_service","group_service","chat_service","knowledge_service","bot_service","work_service")
foreach ($s in $services) {
  Start-Process -FilePath "go" -ArgumentList "run","main.go" `
    -WorkingDirectory "kitex_service\$s" `
    -RedirectStandardOutput "$logs\$s.log" -RedirectStandardError "$logs\$s.err.log" `
    -WindowStyle Hidden
}
Start-Process -FilePath "go" -ArgumentList "run","cmd/main.go" -WorkingDirectory "api_gateway" `
  -RedirectStandardOutput "$logs\api_gateway.log" -RedirectStandardError "$logs\api_gateway.err.log" -WindowStyle Hidden
Start-Process -FilePath "go" -ArgumentList "run","cmd/main.go" -WorkingDirectory "msg_gateway" `
  -RedirectStandardOutput "$logs\msg_gateway.log" -RedirectStandardError "$logs\msg_gateway.err.log" -WindowStyle Hidden

# 逐个查看错误日志（有输出说明该服务没起来）
Get-ChildItem "$logs\*.err.log" | ForEach-Object { "--- $($_.Name)"; Get-Content $_.FullName -Tail 20 }
```

启动期两个已知现象（都属预期，不必处理）：

- `bot_service` 首次建库时会创建系统 Bot，并顺带初始化「kiana角色Skill」知识库；若 `knowledge_service` 还没就绪，会重试 10 次、每次间隔 3 秒后才放弃。
- `chat_service` 连不上 ClickHouse 时只告警并关闭冷库，不影响消息收发。

### 补充说明：关于「整体容器化」运行

容器化的 Go 服务镜像由各服务 `Dockerfile` 复制源码构建，**不会包含工作区里未提交的改动**。因此：

- 想验证本轮新增能力 → 用上面的本地 `go run`（改动即时生效），或先 `docker compose -f deploy/docker-compose.yml build api-gateway bot-service knowledge-service work-service msg-gateway` 再 `up -d`。
- 本轮验收不需要 `open_service` / `admin_service` / `billing_service`：`api_gateway` 启动时仅创建对应的 Kitex 客户端，服务不在线不影响其它接口；调用开放平台、管理后台、用量统计接口时才需要它们（仓库暂未提供 `build.go` 的启动目标，需手动 `go run` 到对应服务目录）。


---

## 五、探活检查

```powershell
# 1) 网关在监听
Test-NetConnection 127.0.0.1 -Port 1234 -InformationLevel Quiet   # True
Test-NetConnection 127.0.0.1 -Port 8082 -InformationLevel Quiet   # True（换成你的 msg_gateway 端口）

# 2) 服务已注册到 etcd（预期能看到 botservice / knowledgeservice / workservice 等 8 个服务名）
docker exec answer_etcd etcdctl get --prefix "" --keys-only

# 3) 依赖健康
Invoke-WebRequest http://127.0.0.1:7700/health -UseBasicParsing | Select-Object -ExpandProperty Content  # {"status":"available"}
Invoke-WebRequest http://127.0.0.1:6333/healthz -UseBasicParsing | Select-Object -ExpandProperty Content  # healthz check passed
docker ps --format "{{.Names}}`t{{.Status}}" | Select-String "answer_seaweedfs_filer"                          # Up (healthy)
```

---

## 六、验收 A：Skill 上传与绑定（纯 API，不需要 LLM Key）

### 1. 注册并登录

```powershell
$base = "http://localhost:1234"
Invoke-RestMethod -Method Post -Uri "$base/register" -ContentType "application/json" `
  -Body '{"account":"skilltest","name":"技能测试","password":"12345678"}'

$login = Invoke-RestMethod -Method Post -Uri "$base/login" -ContentType "application/json" `
  -Body '{"account":"skilltest","password":"12345678"}'
$token = $login.data.token
$tenantId = $login.data.tenant_id
$headers = @{ Authorization = "Bearer $token" }
"token 长度=$($token.Length) tenant_id=$tenantId"
```

### 2. 造一个 Skill 包

```powershell
New-Item -ItemType Directory -Force .\tmp\demo-skill\references | Out-Null
@"
---
name: 会议纪要
description: 把会议记录整理成结构化要点
version: 1.0.0
---
# 会议纪要 Skill

先给结论，再列行动项。
"@ | Set-Content -Encoding utf8 .\tmp\demo-skill\SKILL.md
"- 行动项必须包含负责人与截止时间" | Set-Content -Encoding utf8 .\tmp\demo-skill\references\checklist.md
Compress-Archive -Path .\tmp\demo-skill\* -DestinationPath .\tmp\demo-skill.zip -Force
```

### 3. 上传

```powershell
$up = Invoke-RestMethod -Method Post -Uri "$base/api/skill/upload" -Headers $headers `
  -Form @{ file = Get-Item .\tmp\demo-skill.zip }
$up | ConvertTo-Json -Depth 5
$skillId = $up.data.skill_id
```

预期：`code=0`，`file_count=2`，`warnings=[]`。若 `warnings` 里出现 `assets/xxx: 仅支持 md/markdown/txt 资料文件`，说明非资料文件被按设计跳过。

> 若 PowerShell 版本不支持 `-Form`，改用 curl：`curl.exe -s -X POST "$base/api/skill/upload" -H "Authorization: Bearer $token" -F "file=@tmp\demo-skill.zip"`

### 4. 列表与详情

```powershell
(Invoke-RestMethod -Uri "$base/api/skill/list" -Headers $headers).data.skills | Format-Table skill_id,name,version,status,file_count,size_bytes,kb_id
$detail = Invoke-RestMethod -Uri "$base/api/skill/detail?skill_id=$skillId" -Headers $headers
$detail.data.files | Format-Table rel_path,size_bytes
$kbId = $detail.data.skill.kb_id
```

预期：`status=1`（可用），`files` 含 `SKILL.md` 与 `references/checklist.md`。

### 5. 创建 Bot 并绑定

```powershell
$bot = Invoke-RestMethod -Method Post -Uri "$base/api/bot/create" -Headers $headers -ContentType "application/json" `
  -Body '{"name":"纪要助手","system_prompt":"你是会议纪要助手","model":"doubao-seed-1-8-251228","base_url":"https://ark.cn-beijing.volces.com/api/v3","api_key":"你的对话密钥"}'
$botId = $bot.data.bot_id

Invoke-RestMethod -Method Post -Uri "$base/api/skill/bind" -Headers $headers -ContentType "application/json" `
  -Body (@{ bot_id = $botId; skill_id = $skillId; enabled = $true } | ConvertTo-Json)
```

### 6. 双向校验绑定是否真的落到知识库链路

```powershell
# a) Skill 维度
(Invoke-RestMethod -Uri "$base/api/skill/bot_skills?bot_id=$botId" -Headers $headers).data.skills | Format-Table

# b) 知识库维度（关键：证明写入 bot_knowledge，运行时会被 search_knowledge 检索）
(Invoke-RestMethod -Uri "$base/api/knowledge/bot_bases?bot_id=$botId" -Headers $headers).data | ConvertTo-Json -Depth 4
```

第二条应返回名为「会议纪要 Skill」的知识库；这是「Skill 绑定生效」的硬证据。

---

## 七、验收 B：前端页面

```powershell
cd web\test_frontend
npm install       # 首次
npm run dev
```

浏览器打开 <http://localhost:3000> → 登录同一账号 → 左侧进入「AI 助手」→ 顶部页签 **Skill**：

| 检查点 | 预期 |
| --- | --- |
| 上传按钮 | 选择 `tmp\demo-skill.zip` 后弹出「Skill 已导入」 |
| 列表卡片 | 显示名称、`可用` 徽标、`v1.0.0`、`2 个文件`、体积 |
| 选中卡片 | 下方出现「文件清单」，列出 `SKILL.md` / `references/checklist.md` |
| 绑定下拉 | 选中「纪要助手」后，卡片出现 `已绑定` 徽标；点 🔗/🔌 可绑定 / 解绑 |
| 解绑 | 再点一次后徽标消失，`bot_skills` 列表同步为空 |

---

## 八、验收 C：检索链路（需要 Embedding Key）

Skill 的可用性最终取决于文档是否被切成 chunk 并写入向量库。

```powershell
# 查看该 Skill 知识库下的文档解析状态
(Invoke-RestMethod -Uri "$base/api/knowledge/document/list?kb_id=$kbId" -Headers $headers).data.documents |
  Format-Table file_name,status,chunk_count
```

| 看到的状态 | 含义 | 处理 |
| --- | --- | --- |
| `status=parsed`、`chunk_count>0` | 向量与 BM25 索引均就绪，运行时必被检索到 | 无需处理 |
| `status=pending` | 解析消息尚未被消费 | 确认 `knowledge_service` 已启动、Kafka 正常 |
| `status=failed` | 多为 Embedding 调用失败 | 配置 `EMBEDDING_API_KEY` 后重试解析（见下） |

```powershell
# 失败后补配密钥并重试（doc_id 取自上一条命令的输出）
$docId = "上面的 doc_id"
Invoke-RestMethod -Method Post -Uri "$base/api/knowledge/document/retry" -Headers $headers `
  -ContentType "application/json" -Body (@{ doc_id = $docId } | ConvertTo-Json)
```

状态为 `parsed` 后，在「纪要助手」里发送与资料相关的问题（如「会议纪要的格式要求是什么」），观察 `work_service` 日志出现知识检索调用，回复中会引用 Skill 内容。

---

## 九、验收 D：多端在线（本轮新增）

同浏览器开**两个标签页**登录同一账号（共享 localStorage 中的 token，但每个标签各自建立一条 WebSocket），即等价于两台设备。

```powershell
# 观察 msg_gateway 日志
Get-Content .scratch\logs\msg_gateway.log -Wait -Tail 20
```

| 操作 | 预期日志 | 说明 |
| --- | --- | --- |
| 打开第 1 个标签 | `用户<id>上线, 当前在线用户: 1` | 按「用户」计数，不按连接 |
| 打开第 2 个标签 | `用户<id>上线, 当前在线用户: 1` | 多端共存，不再踢掉旧连接 |
| 关闭第 1 个标签 | `用户<id>关闭一个连接，仍有1个连接在线` | 另一设备保持在线 |
| 关闭第 2 个标签 | `用户<id>下线, 当前在线用户: 0` | 全部断开才摘除在线状态 |

进一步验证实时链路：给该账号发一条消息，两个标签页应同时收到；在 A 标签标记已读后，B 标签会收到 `read_receipt` 帧（DevTools → Network → WS → Messages 可见）。

同一账号最多保留 5 条并发连接，超出时最早的一条会被关闭（日志：`并发连接超过5个，关闭最早连接`）。

---

## 十、验收 E：Bot 任务并发配置

```powershell
# 1) 在 kitex_service/work_service/config.yaml 的 ai 段追加：
#    bot_task:
#      max_concurrent: 3
#      per_bot_queue: 20

# 2) 重启 work_service，观察启动日志
Get-Content .scratch\logs\work_service.log -Tail 20 | Select-String "Bot任务并发上限"
```

预期输出 `Bot任务并发上限=3, 单Bot队列长度=20`；不配置时默认 `10 / 50`。

---

## 十一、清理

```powershell
# 停止前端与 Go 服务
Get-Process go -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process node -ErrorAction SilentlyContinue | Stop-Process -Force

# 停止依赖容器（保留数据）
docker compose -f deploy/docker-compose.yml down

# 连数据卷一起删除（下次启动是全新库，会重建表并重跑系统 Bot 初始化）
docker compose -f deploy/docker-compose.yml down -v
```

---

## 十二、排障速查

| 现象 | 根因 | 处理 |
| --- | --- | --- |
| `docker compose` 报 `yaml: ... did not find expected key` | 早期提交里 `knowledge-service` 段的缩进少了 3 个空格，整份 compose 无法解析 | 已在本次修复；若从旧分支拉取请同步该改动，或用 `docker compose -f deploy/docker-compose.yml config --services` 自检 |
| `user_service` 启动即退出，日志 `Error 3140 (22032): Invalid JSON text: "The document is empty." ... column 'tenant.settings'` | `tenant.settings` 是 MySQL JSON 列，不接受空字符串，而早期版本把 `string` 零值直接写进去（平台租户初始化必踩） | 已修：`Tenant` 增加 `BeforeSave` 把空值归一化为 `{}`。同步代码后**直接重启即可**，不需要清库（`EnsurePlatformTenant` 会在平台租户不存在时自动补建） |
| `api_gateway` / `bot_service` 启动即退出，日志含「SeaweedFS健康检查失败」 | `storage.Init` 对 filer 做硬校验 | 先起 `seaweedfs-filer`，等 `8888` 可访问 |
| 前端 WS 一直重连 | `vite.config.ts` 的 `/ws` 与 `msg_gateway/config.yaml` 的 `gateway.addr` 不一致，或端口被 QQ 占用 | 两处改成同一端口，必要时换 8090 |
| 上传报「Skill 只支持 zip 包或 markdown 文件」 | 后缀不在 `.zip/.md/.markdown/.txt` | 重新打包 |
| 上传报「Skill 包解压后不能超过 20 MB」 | 解压总量超限 | 精简资料文件（上限：包 10MB / 解压 20MB / 单文件 2MB / ≤50 个） |
| 绑定报「只有Bot创建者才能绑定Skill」 | Bot 不是你本人创建的，或选了系统 Bot | 用自己建的 Bot |
| 绑定报「把Skill知识库绑定到Bot失败」 | `knowledge_service` 不可达，或知识库已被外部删除 | 查 `knowledge_service` 日志，重新上传 Skill |
| 文档长期 `failed` | `EMBEDDING_API_KEY` 未配置、模型名或额度有问题 | 配置后重试解析 |
| 前端登录后立刻 401 | `jwt.Key` 在 `api_gateway` 与 `msg_gateway` 不一致 | 两处保持相同值 |
| 服务起不来但日志无异常 | 端口被占用（4320~4326 / 1234 / 8082） | `Get-NetTCPConnection -LocalPort <port>` 查占用进程 |
| 表结构与代码不一致 | 改过模型但旧库残留 | `docker compose down -v` 重建，或手工迁移 |
| 命令直接报 `SetNamedSecurityInfoW` 之类的文件权限错误 | 工作区权限阻止执行 | 该现象属环境问题，非项目缺陷；按提示放宽工作区权限后重试 |
