# API 接口文档

> Base URL: `http://<host>:<port>`
> 认证方式：JWT Bearer Token（除注册和登录外，所有 `/api` 路由均需携带）

---

## 通用响应格式

```json
{
  "code": 0,
  "msg": "提示信息",
  "data": {}
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 业务状态码，`0` 成功，`1` 失败 |
| msg | string | 提示信息 |
| data | any | 响应数据，失败时为错误详情字符串 |

---

## 认证机制

### JWT Token

- 登录成功后返回 `token`
- 后续请求需在 Header 中携带：`Authorization: <token>`
- Token 有效期：24 小时
- Token 刷新期：7 天

---

## 1. 用户认证

### 1.1 注册

- **URL**: `/register`
- **Method**: `POST`
- **认证**: 不需要

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| account | string | 是 | 账号 |
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |

**请求示例**:
```json
{
  "account": "testuser",
  "username": "测试用户",
  "password": "123456"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "注册成功"
}
```

失败（用户已存在）：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "用户已存在或完善你的信息"
}
```

---

### 1.2 登录

- **URL**: `/login`
- **Method**: `POST`
- **认证**: 不需要

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| account | string | 是 | 账号 |
| password | string | 是 | 密码 |

**请求示例**:
```json
{
  "account": "testuser",
  "password": "123456"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "id": 1,
    "account": "testuser"
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "账号或密码错误"
}
```

---

## 2. 好友管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 2.1 添加好友

- **URL**: `/api/add_friend`
- **Method**: `POST`

向目标用户发送好友请求。系统会校验：不能添加自己、目标用户必须存在、双方不能已是好友、不能重复发送请求。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| receiver | int64 | 是 | 目标用户 ID |
| message | string | 否 | 好友请求附言 |

**请求示例**:
```json
{
  "receiver": 2,
  "message": "你好，我想添加你为好友"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "好友请求已发送"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "用户不存在或已是好友或已发送请求"
}
```

---

### 2.2 处理好友请求

- **URL**: `/api/handle_friend_req`
- **Method**: `POST`

接受或拒绝收到的好友请求。接受后双方自动建立好友关系。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| sender | int64 | 是 | 请求发起者用户 ID |
| accept | bool | 是 | `true` 接受，`false` 拒绝 |

**请求示例**:
```json
{
  "sender": 1,
  "accept": true
}
```

**响应示例**:

成功（接受）：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "好友请求已通过"
}
```

成功（拒绝）：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "好友请求已拒绝"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "请求不存在或已处理"
}
```

---

### 2.3 删除好友

- **URL**: `/api/delete_friend`
- **Method**: `POST`

单方面删除好友，系统会同时清理双方的好友关系记录。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| friend_id | int64 | 是 | 要删除的好友用户 ID |

**请求示例**:
```json
{
  "friend_id": 2
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "删除成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "好友关系不存在"
}
```

---

### 2.4 获取好友列表

- **URL**: `/api/get_friend_list`
- **Method**: `GET`

获取当前用户的所有好友信息，包含好友的用户名、备注和分组。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "friend_id": 2,
      "remark": "小明",
      "group_id": 1,
      "name": "张三"
    },
    {
      "friend_id": 3,
      "remark": "",
      "group_id": 0,
      "name": "李四"
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| friend_id | int64 | 好友用户 ID |
| remark | string | 好友备注，未设置时为空字符串 |
| group_id | int64 | 所属分组 ID，`0` 表示默认分组 |
| name | string | 好友用户名 |

---

### 2.5 获取好友请求列表

- **URL**: `/api/get_friend_requests`
- **Method**: `GET`

获取当前用户收到的所有好友请求。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "sender": 1,
      "receiver": 2,
      "message": "你好，我想添加你为好友",
      "status": 0
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| sender | int64 | 请求发起者用户 ID |
| receiver | int64 | 请求接收者用户 ID |
| message | string | 请求附言 |
| status | int64 | 请求状态：`0` 待处理，`1` 已接受，`2` 已拒绝 |

---

### 2.6 修改好友备注

- **URL**: `/api/update_friend_remark`
- **Method**: `POST`

修改指定好友的备注名。需双方已是好友关系。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| friend_id | int64 | 是 | 好友用户 ID |
| remark | string | 是 | 新备注名，传空字符串可清除备注 |

**请求示例**:
```json
{
  "friend_id": 2,
  "remark": "小明同学"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "修改成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "好友关系不存在"
}
```

---

### 2.7 搜索用户

- **URL**: `/api/search_user`
- **Method**: `GET`

根据账号搜索用户。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| account | string | 是 | 用户账号（Query 参数） |

**请求示例**: `/api/search_user?account=testuser`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "id": 2,
    "account": "testuser",
    "name": "测试用户"
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "用户不存在",
  "data": "请检查账号"
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int64 | 用户 ID |
| account | string | 用户账号 |
| name | string | 用户名 |

---

## 3. 好友分组管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 3.1 创建好友分组

- **URL**: `/api/create_friend_group`
- **Method**: `POST`

创建一个新的好友分组。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 分组名称，不能为空 |

**请求示例**:
```json
{
  "name": "同学"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "group_id": 1
  }
}
```

失败（名称为空）：
```json
{
  "code": 1,
  "msg": "参数缺失",
  "data": "分组名称不能为空"
}
```

---

### 3.2 修改好友分组名称

- **URL**: `/api/update_friend_group`
- **Method**: `POST`

修改指定分组的名称。仅分组所属用户可操作。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 分组 ID |
| name | string | 是 | 新分组名称，不能为空 |

**请求示例**:
```json
{
  "group_id": 1,
  "name": "大学同学"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "修改成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "分组不存在或无权限"
}
```

---

### 3.3 删除好友分组

- **URL**: `/api/delete_friend_group`
- **Method**: `POST`

删除指定分组。删除后该分组下的好友会自动移回默认分组（group_id=0）。仅分组所属用户可操作。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 分组 ID |

**请求示例**:
```json
{
  "group_id": 1
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "删除成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "分组不存在或无权限"
}
```

---

### 3.4 移动好友到分组

- **URL**: `/api/move_friend_to_group`
- **Method**: `POST`

将好友移动到指定分组。`group_id` 传 `0` 表示移回默认分组。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| friend_id | int64 | 是 | 好友用户 ID |
| group_id | int64 | 是 | 目标分组 ID，`0` 为默认分组 |

**请求示例**:
```json
{
  "friend_id": 2,
  "group_id": 1
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "移动成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "好友关系或分组不存在"
}
```

---

### 3.5 获取好友分组列表

- **URL**: `/api/get_friend_groups`
- **Method**: `GET`

获取当前用户的所有好友分组。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "group_id": 1,
      "name": "同学"
    },
    {
      "group_id": 2,
      "name": "同事"
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | int64 | 分组 ID |
| name | string | 分组名称 |

---

## 4. 群组管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 4.1 创建群组

- **URL**: `/api/create_group`
- **Method**: `POST`

创建群组并可选地拉入初始成员。创建群组时会自动创建对应的群聊会话。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 群组名称 |
| initial_members | int64[] | 否 | 初始成员用户 ID 列表 |

**请求示例**:
```json
{
  "name": "项目讨论组",
  "initial_members": [2, 3, 4]
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "group_id": 1,
    "group_number": "7234567890123456",
    "conversation_id": 100
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "请重新选择拉群人"
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | int64 | 群组 ID |
| group_number | string | 群号（字符串格式） |
| conversation_id | int64 | 群聊会话 ID，创建群组时自动生成 |

---

### 4.2 邀请成员

- **URL**: `/api/invite_members`
- **Method**: `POST`

邀请用户加入群组。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| user_ids | int64[] | 是 | 被邀请用户 ID 列表 |

**请求示例**:
```json
{
  "group_id": 1,
  "user_ids": [5, 6]
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "邀请成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足或成员已在群聊"
}
```

---

### 4.3 踢出成员

- **URL**: `/api/kick_members`
- **Method**: `POST`

将成员踢出群组。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| user_ids | int64[] | 是 | 被踢出用户 ID 列表 |

**请求示例**:
```json
{
  "group_id": 1,
  "user_ids": [5]
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "踢出成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足或操作不合法"
}
```

---

### 4.4 获取群组信息

- **URL**: `/api/get_group_info`
- **Method**: `GET`

获取指定群组的详细信息，包含群组基本信息和成员列表。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID（Query 参数） |

**请求示例**: `/api/get_group_info?group_id=1`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "group_id": 1,
    "name": "项目讨论组",
    "owner_id": 1,
    "owner_name": "张三",
    "notice": "欢迎加入",
    "create_time": 1700000000,
    "group_number": "7234567890123456",
    "members": [
      {
        "user_id": 1,
        "name": "张三",
        "role": 3,
        "is_muted": false,
        "join_time": 1700000000
      },
      {
        "user_id": 2,
        "name": "李四",
        "role": 1,
        "is_muted": false,
        "join_time": 1700000001
      }
    ]
  }
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | int64 | 群组 ID |
| name | string | 群组名称 |
| owner_id | int64 | 群主用户 ID |
| owner_name | string | 群主用户名 |
| notice | string | 群公告 |
| create_time | int64 | 创建时间（Unix 时间戳） |
| group_number | string | 群号（字符串格式） |
| members | array | 成员列表 |

**members 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | int64 | 用户 ID |
| name | string | 用户名 |
| role | int64 | 角色：`1` 普通成员，`2` 管理员，`3` 群主 |
| is_muted | bool | 是否被禁言 |
| join_time | int64 | 加入时间（Unix 时间戳） |

失败：
```json
{
  "code": 1,
  "msg": "参数缺失或错误",
  "data": "无效的群组ID"
}
```

---

### 4.5 转让群主

- **URL**: `/api/change_owner`
- **Method**: `POST`

将群主身份转让给其他群成员。仅群主可操作。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| new_id | int64 | 是 | 新群主用户 ID |

**请求示例**:
```json
{
  "group_id": 1,
  "new_id": 2
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "转让成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足或新群主不在群聊内"
}
```

---

### 4.6 修改群公告

- **URL**: `/api/change_notice`
- **Method**: `POST`

修改群组公告。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| notice | string | 是 | 新公告内容 |

**请求示例**:
```json
{
  "group_id": 1,
  "notice": "本周五下午3点开会"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "群公告修改成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足"
}
```

---

### 4.7 禁言/解禁

- **URL**: `/api/muted`
- **Method**: `POST`

对群成员进行禁言或解禁操作。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| muted_id | int64 | 是 | 被操作用户 ID |
| is_muted | bool | 是 | `true` 禁言，`false` 解禁 |

**请求示例**:
```json
{
  "group_id": 1,
  "muted_id": 3,
  "is_muted": true
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "禁言状态修改成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足或操作不合法"
}
```

---

### 4.8 设置管理员

- **URL**: `/api/set_admin`
- **Method**: `POST`

设置或取消群管理员。仅群主可操作。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| target_id | int64 | 是 | 目标用户 ID |
| role | int64 | 是 | 角色值：`1` 取消管理员，`2` 设置管理员 |

**请求示例**:
```json
{
  "group_id": 1,
  "target_id": 2,
  "role": 2
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "管理员设置修改成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足或操作不合法"
}
```

---

### 4.9 获取用户群组列表

- **URL**: `/api/get_user_groups`
- **Method**: `GET`

获取当前用户加入的所有群组。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "group_id": 1,
      "name": "项目讨论组",
      "group_number": "7234567890123456"
    },
    {
      "group_id": 2,
      "name": "技术交流群",
      "group_number": "7234567890123457"
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | int64 | 群组 ID |
| name | string | 群组名称 |
| group_number | string | 群号（字符串格式） |

---

### 4.10 搜索群组

- **URL**: `/api/search_group`
- **Method**: `GET`

根据群号搜索群组。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_number | string | 是 | 群号（Query 参数，必须为数字） |

**请求示例**: `/api/search_group?group_number=7234567890123456`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "group_id": 1,
    "name": "项目讨论组",
    "owner_name": "张三",
    "group_number": "7234567890123456"
  }
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | int64 | 群组 ID |
| name | string | 群组名称 |
| owner_name | string | 群主用户名 |
| group_number | string | 群号（字符串格式） |

失败（群号不存在）：
```json
{
  "code": 1,
  "msg": "群号不存在",
  "data": "请检查群号"
}
```

失败（格式错误）：
```json
{
  "code": 1,
  "msg": "参数格式错误",
  "data": "群号必须为数字"
}
```

---

### 4.11 申请入群

- **URL**: `/api/join_group`
- **Method**: `POST`

通过群号申请加入群组。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_number | string | 是 | 群号（必须为数字字符串） |
| message | string | 否 | 入群申请附言 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "message": "你好，我想加入这个群"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "申请已发送"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "已存在待处理的申请或已是群成员"
}
```

---

### 4.12 处理入群申请

- **URL**: `/api/handle_join_req`
- **Method**: `POST`

接受或拒绝入群申请。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID |
| user_id | int64 | 是 | 申请人用户 ID |
| accept | bool | 是 | `true` 接受，`false` 拒绝 |

**请求示例**:
```json
{
  "group_id": 1,
  "user_id": 5,
  "accept": true
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "处理成功"
}
```

失败：
```json
{
  "code": 1,
  "msg": "操作失败",
  "data": "权限不足或申请已被处理"
}
```

---

### 4.13 获取入群申请列表

- **URL**: `/api/get_join_requests`
- **Method**: `GET`

获取指定群组的入群申请列表。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_id | int64 | 是 | 群组 ID（Query 参数） |

**请求示例**: `/api/get_join_requests?group_id=1`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "user_id": 5,
      "name": "王五",
      "message": "你好，我想加入这个群",
      "status": 0
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | int64 | 申请人用户 ID |
| name | string | 申请人用户名 |
| message | string | 申请附言 |
| status | int64 | 申请状态：`0` 待处理，`1` 已接受，`2` 已拒绝 |

失败：
```json
{
  "code": 1,
  "msg": "参数缺失或错误",
  "data": "无效的群组ID"
}
```

---

## 5. 聊天服务

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 5.1 获取消息列表

- **URL**: `/api/chat/messages`
- **Method**: `GET`

拉取指定会话的消息，支持游标分页。会校验用户是否为会话成员。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| conversation_id | int64 | 是 | 会话 ID（Query 参数） |
| before_msg_id | int64 | 否 | 游标，返回 msg_id < before_msg_id 的消息，默认 `0`（从最新开始） |
| limit | int | 否 | 返回条数，范围 [1, 100]，默认 `20` |

**请求示例**: `/api/chat/messages?conversation_id=100&before_msg_id=0&limit=20`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "messages": [
      {
        "msg_id": 1,
        "client_seq": 1001,
        "sender_id": 1,
        "conversation_id": 100,
        "content": "你好",
        "timestamp": 1700000000
      },
      {
        "msg_id": 2,
        "client_seq": 1002,
        "sender_id": 2,
        "conversation_id": 100,
        "content": "你好！",
        "timestamp": 1700000001
      }
    ]
  }
}
```

**messages 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| msg_id | int64 | 服务端消息 ID |
| client_seq | int64 | 客户端序列号 |
| sender_id | int64 | 发送者用户 ID |
| conversation_id | int64 | 会话 ID |
| content | string | 消息内容 |
| timestamp | int64 | 发送时间（Unix 时间戳） |

失败：
```json
{
  "code": 1,
  "msg": "查询失败",
  "data": "无权查看该会话消息或获取历史消息失败"
}
```

---

### 5.2 获取会话列表

- **URL**: `/api/chat/conversations`
- **Method**: `GET`

获取当前用户参与的所有会话，按最近活跃时间降序排列。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "conversations": [
      {
        "conversation_id": 100,
        "type": 1,
        "name": "",
        "group_id": 0,
        "member_ids": [1, 2]
      },
      {
        "conversation_id": 101,
        "type": 2,
        "name": "项目讨论组",
        "group_id": 1,
        "member_ids": [1, 2, 3, 4]
      }
    ]
  }
}
```

**conversations 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| conversation_id | int64 | 会话 ID |
| type | int | 会话类型：`1` 单聊，`2` 群聊 |
| name | string | 会话名称（单聊为空，群聊为群组名称） |
| group_id | int64 | 群聊关联的群组 ID，单聊时为 `0` |
| member_ids | int64[] | 会话成员用户 ID 列表 |

失败：
```json
{
  "code": 1,
  "msg": "查询失败",
  "data": "获取会话列表失败"
}
```

---

## 6. 文件管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 6.1 上传文件

- **URL**: `/api/files`
- **Method**: `POST`
- **Content-Type**: `multipart/form-data`

上传文件到对象存储。文件大小不能超过 50MB。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| file | file | 是 | 上传的文件，最大 50MB |

**请求示例**（Form Data）:
```
file: <选择文件>
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "url": "https://oss.example.com/files/abc123.jpg",
    "file_name": "photo.jpg",
    "file_size": 102400,
    "media_type": "image"
  }
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| url | string | 文件访问地址 |
| file_name | string | 文件名 |
| file_size | int64 | 文件大小（字节） |
| media_type | string | 媒体类型：`image`、`video`、`audio`、`file` |

失败（文件过大）：
```json
{
  "code": 1,
  "msg": "文件过大",
  "data": "文件大小不能超过50MB"
}
```

失败（未选择文件）：
```json
{
  "code": 1,
  "msg": "参数错误",
  "data": "请选择要上传的文件"
}
```

---

## 错误处理机制

### 业务状态码

| code | 含义 |
|------|------|
| 0 | 请求成功 |
| 1 | 请求失败 |

### 常见错误场景

| 场景 | msg | data |
|------|-----|------|
| 请求体 JSON 解析失败 | 参数缺失 | 请重新输入参数 |
| RPC 服务不可用 | 系统繁忙 | 请稍后重试 |
| 业务逻辑校验不通过 | 操作失败 | 具体原因描述 |
| JWT Token 缺失或过期 | 登录过期 | 请重新登录 |
| JWT Token 格式错误 | 登录过期 | 请重新登录 |

### HTTP 状态码

所有响应的 HTTP 状态码均为 `200`，业务层面的成功/失败通过响应体中的 `code` 字段区分。

### Apipost 调试配置建议

1. **环境变量**：设置 `base_url` 为服务地址，如 `http://localhost:8080`
2. **Token 配置**：先调用登录接口获取 token，在环境变量中设置 `token` 值
3. **Auth Header**：认证接口的 Header 设置为 `Authorization: {{token}}`
4. **Content-Type**：所有 POST 请求设置 `Content-Type: application/json`
