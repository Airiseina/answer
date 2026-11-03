# Answer 开放平台接入指南

开放平台允许第三方开发者以「应用」身份**自助部署并托管自己的机器人**，无需修改平台代码。
你只需要一个 AppID + AppSecret，即可通过 HTTP 接口创建机器人、收发消息、订阅事件。

机器人使用**你自己的模型 API Key**（`api_key` + `model` + `base_url`），
模型费用由你自己承担；通过开放平台接入的机器人**不计入平台的 token 用量统计**。

- 可浏览接口文档：`GET /open/docs`（Redoc 页面）
- 机器可读规范：`GET /open/openapi.yaml`（OpenAPI 3.1，可导入 Postman / Apifox / Apifox）
- 规范源文件：[docs/openapi/open_platform.yaml](openapi/open_platform.yaml)

---

## 1. 快速开始

### 1.1 创建应用（站内操作）

在客户端「开放平台」入口，或直接调用站内接口创建应用：

```bash
curl -X POST https://<网关地址>/api/open/apps/create \
  -H "Authorization: <你的用户 JWT>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的客服机器人",
    "scopes": "bot:read,bot:write,message:send,message:read,conversation:read,webhook:manage",
    "qps_limit": 20
  }'
```

响应中会**一次性**返回 `app_id` 与 `app_secret`：

```json
{ "code": 0, "msg": "操作成功", "data": {
  "app_id": "app_5f1c2d3e4b",
  "app_secret": "9c1f...（仅显示一次）",
  "notice": "AppSecret 仅显示一次，请立即妥善保存"
} }
```

> 妥善保存 `app_secret`。它只在创建与轮换时返回一次，平台只保存密文，
> 无法再次查看。丢失时请调用 `/api/open/apps/reset_secret` 轮换（旧密钥立即失效）。

### 1.2 作用域（scopes）

| Scope | 覆盖接口 |
| --- | --- |
| `bot:read` | 查询机器人列表 / 详情 |
| `bot:write` | 创建 / 更新 / 删除机器人，拉机器人入会话 |
| `message:send` | 以机器人身份发送消息 |
| `message:read` | 拉取历史消息、拉取事件、确认事件 |
| `conversation:read` | 查询机器人参与的会话 |
| `webhook:manage` | 回调注册 / 查询 / 详情 / 删除 |

调用未授权的接口会返回 `code=1, msg=权限不足`。

---

## 2. 签名鉴权

所有 `/open/v1/*` 接口都要携带 4 个请求头：

| 请求头 | 说明 |
| --- | --- |
| `X-App-Id` | 应用 AppID |
| `X-Timestamp` | Unix **秒级**时间戳，与服务端偏差需在 **±300 秒**内 |
| `X-Nonce` | 随机串；同一应用 **300 秒**内不可重复（防重放） |
| `X-Signature` | HMAC-SHA256 十六进制签名 |

### 2.1 签名算法

把 6 段内容用 `\n` 连接，得到待签串 `stringToSign`：

```
appID \n timestamp \n nonce \n METHOD \n path \n sha256Hex(body)
```

- `METHOD`：大写 HTTP 方法，如 `GET` / `POST` / `PATCH` / `DELETE`。
- `path`：**不含查询串**、不含域名的接口路径，如 `/open/v1/bots`。
  注意 `GET /open/v1/messages?bot_id=1` 的参与签名的 `path` 仍然是 `/open/v1/messages`。
- `sha256Hex(body)`：请求体的 SHA-256 十六进制摘要；无请求体时为空串 `""`。

签名 = `hex(hmac_sha256(appSecret, stringToSign))`，放入 `X-Signature`（小写十六进制）。

### 2.2 各语言示例

**cURL / Shell**

```bash
APP_ID="app_5f1c2d3e4b"
APP_SECRET="你的 AppSecret"
BODY='{"name":"demo","api_key":"sk-xxx","model":"gpt-4o-mini"}'
TS=$(date +%s)
NONCE=$(openssl rand -hex 8)
PATH_ONLY="/open/v1/bots"
METHOD="POST"
BODY_SHA=$(printf '%s' "$BODY" | openssl dgst -sha256 -hex | awk '{print $2}')
TO_SIGN=$(printf '%s\n%s\n%s\n%s\n%s\n%s' "$APP_ID" "$TS" "$NONCE" "$METHOD" "$PATH_ONLY" "$BODY_SHA")
SIGNATURE=$(printf '%s' "$TO_SIGN" | openssl dgst -sha256 -hmac "$APP_SECRET" -hex | awk '{print $2}')

curl -X POST "https://<网关地址>${PATH_ONLY}" \
  -H "Content-Type: application/json" \
  -H "X-App-Id: $APP_ID" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"
```

**Go**

```go
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func sign(appID, secret, method, path, body string, ts string, nonce string) string {
	sum := sha256.Sum256([]byte(body))
	toSign := strings.Join([]string{
		appID, ts, nonce, strings.ToUpper(method), path, hex.EncodeToString(sum[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(toSign))
	return hex.EncodeToString(mac.Sum(nil))
}

func main() {
	const (
		baseURL = "https://<网关地址>"
		appID   = "app_5f1c2d3e4b"
		secret  = "你的 AppSecret"
	)
	path := "/open/v1/bots"
	body := `{"name":"demo","api_key":"sk-xxx","model":"gpt-4o-mini"}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nb := make([]byte, 8)
	_, _ = rand.Read(nb)
	nonce := hex.EncodeToString(nb)

	req, _ := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", appID)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", sign(appID, secret, http.MethodPost, path, body, ts, nonce))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	fmt.Println(string(out))
}
```

**TypeScript（Node 18+）**

```ts
import { createHmac, createHash, randomBytes } from "node:crypto";

const BASE_URL = "https://<网关地址>";
const APP_ID = "app_5f1c2d3e4b";
const APP_SECRET = "你的 AppSecret";

function sign(method: string, path: string, body: string): Record<string, string> {
  const ts = Math.floor(Date.now() / 1000).toString();
  const nonce = randomBytes(8).toString("hex");
  const bodySha = createHash("sha256").update(body).digest("hex");
  const toSign = [APP_ID, ts, nonce, method.toUpperCase(), path, bodySha].join("\n");
  const signature = createHmac("sha256", APP_SECRET).update(toSign).digest("hex");
  return {
    "X-App-Id": APP_ID,
    "X-Timestamp": ts,
    "X-Nonce": nonce,
    "X-Signature": signature,
  };
}

async function createBot() {
  const path = "/open/v1/bots";
  const body = JSON.stringify({ name: "demo", api_key: "sk-xxx", model: "gpt-4o-mini" });
  const res = await fetch(BASE_URL + path, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...sign("POST", path, body) },
    body,
  });
  console.log(await res.json());
}

createBot();
```

**Python（requests）**

```python
import hashlib
import hmac
import json
import os
import time

import requests

BASE_URL = "https://<网关地址>"
APP_ID = "app_5f1c2d3e4b"
APP_SECRET = "你的 AppSecret"


def sign(method: str, path: str, body: str) -> dict:
    ts = str(int(time.time()))
    nonce = os.urandom(8).hex()
    body_sha = hashlib.sha256(body.encode()).hexdigest()
    to_sign = "\n".join([APP_ID, ts, nonce, method.upper(), path, body_sha])
    signature = hmac.new(APP_SECRET.encode(), to_sign.encode(), hashlib.sha256).hexdigest()
    return {
        "X-App-Id": APP_ID,
        "X-Timestamp": ts,
        "X-Nonce": nonce,
        "X-Signature": signature,
    }


path = "/open/v1/bots"
body = json.dumps({"name": "demo", "api_key": "sk-xxx", "model": "gpt-4o-mini"})
headers = {"Content-Type": "application/json", **sign("POST", path, body)}
print(requests.post(BASE_URL + path, data=body, headers=headers).json())
```

> 时间戳过期、nonce 重复、签名不匹配都会导致 `code=1`，`msg` 分别为
> `签名已过期` / `重复请求` / `签名校验失败`。请确保服务器时间同步（NTP）。

---

## 3. 机器人管理

创建机器人（对应 `POST /open/v1/bots`）时，`name`、`api_key`、`model` 必填，
`system_prompt` 与 `base_url` 可选：

```json
{
  "name": "客服小助手",
  "system_prompt": "你是耐心专业的客服，回答简洁。",
  "api_key": "sk-your-own-key",
  "model": "gpt-4o-mini",
  "base_url": "https://api.openai.com/v1"
}
```

典型流程：

1. `POST /open/v1/bots` 创建机器人，拿到 `bot_id`。
2. `POST /open/v1/bots/{bot_id}/conversations` 把机器人拉入私聊/群聊。
3. 用户给机器人发消息 → 平台调度模型生成回复 → 产生 `message.reply` 事件。
4. `POST /open/v1/messages/send` 可主动以机器人身份发消息。

> 机器人数量上限默认 50（`open.max_bots_per_app`），可通过配置调整。

---

## 4. 事件：轮询与回调

事件先落库再投递，两种方式可同时使用，互不影响。

### 4.1 事件类型

| 事件类型 | 触发时机 |
| --- | --- |
| `message.received` | 有人（用户或群成员）给机器人发消息 |
| `message.reply` | 机器人生成了一条回复 |

`payload` 为 JSON 字符串，包含消息内容等上下文。

### 4.2 轮询（推荐作为兜底）

```bash
# 首次传 since_id=0，之后用上一批最大的 event_id
curl -H "X-App-Id: $APP_ID" -H "X-Timestamp: $TS" \
     -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" \
     "https://<网关地址>/open/v1/events?since_id=0&limit=50"
```

处理完成后确认事件，避免重复消费：

```json
POST /open/v1/events/ack
{ "event_ids": [1001, 1002, 1003] }
```

### 4.3 回调

注册回调地址：

```json
POST /open/v1/webhooks
{ "bot_id": 1700000000000000001, "url": "https://your.app/answer/callback",
  "events": "message.received,message.reply" }
```

响应会返回 `secret`（回调验签密钥，仅一次）与 `verify_challenge`。平台随后会向该地址
发起一次校验 POST，请求体含 `verify_challenge`，你的服务需**原样回显**该字段即视为通过。

投递失败按指数退避重试（默认 1s / 5s / 30s / 120s，最多 5 次），超过次数进入死信。

#### 回调请求格式

回调也是一个签名 POST，请求头与调用开放平台一致
（`X-App-Id` / `X-Timestamp` / `X-Nonce` / `X-Signature`），请求体：

```json
{
  "event_id": 1001,
  "event_type": "message.received",
  "app_id": "app_5f1c2d3e4b",
  "bot_id": 1700000000000000001,
  "created_at": 1730000000000,
  "payload": { "...": "事件内容" }
}
```

#### 回调验签

用注册时返回的 `secret` 按 [2.1 签名算法](#21-签名算法) 计算，注意回调的 `path` 固定为 `/`：

```python
def verify_callback(secret: str, headers, raw_body: bytes) -> bool:
    ts = headers["X-Timestamp"]
    nonce = headers["X-Nonce"]
    body_sha = hashlib.sha256(raw_body).hexdigest()
    to_sign = "\n".join([headers["X-App-Id"], ts, nonce, "POST", "/", body_sha])
    expect = hmac.new(secret.encode(), to_sign.encode(), hashlib.sha256).hexdigest()
    return hmac.compare_digest(expect, headers["X-Signature"])
```

请在回调处理里**先去重（按 event_id）再处理**，并尽快返回 2xx，避免被重试。

---

## 5. 响应与错误

所有接口返回 HTTP 200，业务结果在 body：

```json
{ "code": 0, "msg": "操作成功", "data": { } }
```

`code=0` 成功，`code=1` 失败。失败时 `msg` 为错误分类，`data` 为具体原因：

| msg | 含义 | 处理建议 |
| --- | --- | --- |
| `鉴权失败` | 缺少签名请求头 | 补齐 4 个请求头 |
| `签名校验失败` | 签名不匹配 | 检查 AppSecret 与待签串拼装 |
| `签名已过期` | 时间戳超出 ±300 秒 | 校准服务器时间后重试 |
| `重复请求` | nonce 已被使用 | 每次请求生成新的 nonce |
| `应用不可用` | 应用不存在或已停用 | 检查 AppID / 应用状态 |
| `权限不足` | 缺少对应 scope | 补 scope 或换用有权限的应用 |
| `请求过于频繁` | 超出 QPS | 降低频率或申请更高 `qps_limit` |
| `参数缺失` / `参数错误` | 入参不合法 | 按文档校验入参 |
| `系统繁忙` | 服务端异常 | 稍后重试（建议指数退避） |

---

## 6. 限流

- 维度：**按 AppID**。
- 默认：**20 QPS**（突发容量同样为 20）。创建应用时可用 `qps_limit` 覆盖。
- 超限响应：`code=1, msg=请求过于频繁`。

建议客户端对 `系统繁忙` 与 `请求过于频繁` 做指数退避重试，并对写接口保证幂等。

---

## 7. 默认取值

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `open.timestamp_window` | 300 秒 | 时间戳有效窗口 |
| `open.nonce_ttl` | 300 秒 | nonce 去重窗口 |
| `open.default_qps` | 20 | 单 AppID 默认 QPS |
| `open.max_bots_per_app` | 50 | 单应用机器人上限 |
| `open.webhook_timeout` | 10 秒 | 单次回调超时 |
| `open.webhook_max_attempts` | 5 | 回调最大投递次数 |
| `open.webhook_retry_backoff` | 1/5/30/120 秒 | 回调重试退避 |
| `open.event_retention_days` | 30 天 | 事件保留天数 |

---

## 8. 数据与安全

- AppSecret 在库中保存两份：`app_secret_hash`（SHA-256 摘要，用于快速失效判定）
  与 `app_secret_cipher`（AES-256-GCM 密文，验签时还原明文）。明文只在创建/轮换时返回一次。
- 防重放：时间戳窗口 + nonce 去重（Garnet `SETNX`，TTL 与窗口一致）。
- 回调验签：使用每条回调独立的 `secret`，与 AppSecret 分离。
- 审计：应用创建/停用、Bot 发布审核、租户管理等关键操作写入审计日志。

---

## 9. 当前版本边界

- 开放平台**暂不提供** MCP Server 绑定与知识库绑定接口，将在后续迭代开放。
- 通过开放平台接入的机器人**不参与平台 token 用量统计与计费**，
  模型调用费用由应用方自行承担。
