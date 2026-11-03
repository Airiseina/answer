# API 接口文档

> Base URL: `http://<host>:<port>`
> 认证方式：JWT Bearer Token（除注册、登录和文件代理外，所有 `/api` 路由均需携带）

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
| name | string | 是 | 用户名 |
| password | string | 是 | 密码 |

**请求示例**:
```json
{
  "account": "testuser",
  "name": "测试用户",
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

### 1.3 更新头像

- **URL**: `/api/update_avatar`
- **Method**: `POST`

更新当前用户的头像。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| avatar_url | string | 是 | 头像 URL |

**请求示例**:
```json
{
  "avatar_url": "http://localhost/files/avatars/1.png"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": "更新成功"
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
| receiver_account | string | 是 | 目标用户账号 |
| message | string | 否 | 好友请求附言 |

**请求示例**:
```json
{
  "receiver_account": "zhangsan",
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
| sender_account | string | 是 | 请求发起者账号 |
| accept | bool | 是 | `true` 接受，`false` 拒绝 |

**请求示例**:
```json
{
  "sender_account": "zhangsan",
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
| friend_account | string | 是 | 要删除的好友账号 |

**请求示例**:
```json
{
  "friend_account": "zhangsan"
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

获取当前用户的所有好友信息，包含好友的账号、备注和分组。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "friend_account": "zhangsan",
      "remark": "小明",
      "group_id": "1",
      "name": "张三"
    },
    {
      "friend_account": "lisi",
      "remark": "",
      "group_id": "0",
      "name": "李四"
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| friend_account | string | 好友账号 |
| remark | string | 好友备注，未设置时为空字符串 |
| group_id | string | 所属分组 ID（字符串），`"0"` 表示默认分组 |
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
      "sender_account": "zhangsan",
      "receiver_account": "lisi",
      "message": "你好，我想添加你为好友",
      "status": 0
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| sender_account | string | 请求发起者账号 |
| receiver_account | string | 请求接收者账号 |
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
| friend_account | string | 是 | 好友账号 |
| remark | string | 是 | 新备注名，传空字符串可清除备注 |

**请求示例**:
```json
{
  "friend_account": "zhangsan",
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
    "group_id": "1"
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
| friend_account | string | 是 | 好友账号 |
| group_id | int64 | 是 | 目标分组 ID，`0` 为默认分组 |

**请求示例**:
```json
{
  "friend_account": "zhangsan",
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
> 群组相关接口统一使用 **群号（group_number）** 标识群组，而非群组内部 ID。

### 4.1 创建群组

- **URL**: `/api/create_group`
- **Method**: `POST`

创建群组并可选地拉入初始成员。创建群组时会自动创建对应的群聊会话。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 群组名称 |
| initial_accounts | string[] | 否 | 初始成员账号列表 |

**请求示例**:
```json
{
  "name": "项目讨论组",
  "initial_accounts": ["zhangsan", "lisi", "wangwu"]
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "group_number": "7234567890123456",
    "conversation_id": "100"
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
| group_number | string | 群号（字符串格式） |
| conversation_id | string | 群聊会话 ID，创建群组时自动生成 |

---

### 4.2 邀请成员

- **URL**: `/api/invite_members`
- **Method**: `POST`

邀请用户加入群组。需要群主或管理员权限。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_number | string | 是 | 群号（数字字符串） |
| accounts | string[] | 是 | 被邀请用户账号列表 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "accounts": ["zhaoliu", "sunqi"]
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
| group_number | string | 是 | 群号（数字字符串） |
| accounts | string[] | 是 | 被踢出用户账号列表 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "accounts": ["zhaoliu"]
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
| group_number | string | 是 | 群号（Query 参数，数字字符串） |

**请求示例**: `/api/get_group_info?group_number=7234567890123456`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "name": "项目讨论组",
    "create_time": 1700000000,
    "group_number": "7234567890123456",
    "owner_account": "zhangsan",
    "owner_name": "张三",
    "notice": "欢迎加入",
    "notices": [
      {
        "id": 1,
        "content": "欢迎加入",
        "operator_id": 1,
        "operator_name": "zhangsan",
        "create_time": 1700000000
      }
    ],
    "members": [
      {
        "account": "zhangsan",
        "name": "张三",
        "role": 3,
        "is_muted": false,
        "join_time": 1700000000
      },
      {
        "account": "lisi",
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
| name | string | 群组名称 |
| create_time | int64 | 创建时间（Unix 时间戳） |
| group_number | string | 群号（字符串格式） |
| owner_account | string | 群主账号 |
| owner_name | string | 群主用户名 |
| notice | string | 最新一条群公告内容 |
| notices | array | 群公告列表 |
| members | array | 成员列表 |

**notices 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int64 | 公告 ID |
| content | string | 公告内容 |
| operator_id | int64 | 操作者用户 ID |
| operator_name | string | 操作者账号 |
| create_time | int64 | 创建时间（Unix 时间戳） |

**members 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| account | string | 用户账号 |
| name | string | 用户名 |
| role | int64 | 角色：`1` 普通成员，`2` 管理员，`3` 群主 |
| is_muted | bool | 是否被禁言 |
| join_time | int64 | 加入时间（Unix 时间戳） |

---

### 4.5 转让群主

- **URL**: `/api/change_owner`
- **Method**: `POST`

将群主身份转让给其他群成员。仅群主可操作。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| group_number | string | 是 | 群号（数字字符串） |
| new_account | string | 是 | 新群主账号 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "new_account": "lisi"
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
| group_number | string | 是 | 群号（数字字符串） |
| notice | string | 是 | 新公告内容 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
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
| group_number | string | 是 | 群号（数字字符串） |
| muted_account | string | 是 | 被操作用户账号 |
| is_muted | bool | 是 | `true` 禁言，`false` 解禁 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "muted_account": "wangwu",
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
| group_number | string | 是 | 群号（数字字符串） |
| target_account | string | 是 | 目标用户账号 |
| role | int64 | 是 | 角色值：`0` 取消管理员（设为普通成员），`1` 设置管理员 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "target_account": "lisi",
  "role": 1
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
      "name": "项目讨论组",
      "group_number": "7234567890123456"
    },
    {
      "name": "技术交流群",
      "group_number": "7234567890123457"
    }
  ]
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
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
    "name": "项目讨论组",
    "owner_name": "张三",
    "group_number": "7234567890123456"
  }
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
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
  "data": "已存在待处理的申请"
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
| group_number | string | 是 | 群号（数字字符串） |
| account | string | 是 | 申请人账号 |
| accept | bool | 是 | `true` 接受，`false` 拒绝 |

**请求示例**:
```json
{
  "group_number": "7234567890123456",
  "account": "wangwu",
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
| group_number | string | 是 | 群号（Query 参数，数字字符串） |

**请求示例**: `/api/get_join_requests?group_number=7234567890123456`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": [
    {
      "account": "wangwu",
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
| account | string | 申请人账号 |
| name | string | 申请人用户名 |
| message | string | 申请附言 |
| status | int64 | 申请状态：`0` 待处理，`1` 已接受，`2` 已拒绝 |

---

## 5. 聊天消息

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 5.1 获取历史消息

- **URL**: `/api/chat/messages`
- **Method**: `GET`

分页获取指定会话的历史消息，支持向上翻页。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| conversation_id | string | 是 | 会话 ID（Query 参数） |
| before_msg_id | string | 否 | 起始消息 ID，默认 `"0"`（Query 参数） |
| limit | int | 否 | 每页数量，默认 20，最大 100（Query 参数） |

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
        "msg_id": "1",
        "client_seq": 1001,
        "sender_account": "zhangsan",
        "sender_name": "张三",
        "conversation_id": "100",
        "content": "你好",
        "timestamp": 1700000000,
        "status": 0,
        "is_edited": false
      }
    ]
  }
}
```

**messages 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| msg_id | string | 服务端消息 ID |
| client_seq | int64 | 客户端序列号 |
| sender_account | string | 发送者账号 |
| sender_name | string | 发送者用户名 |
| conversation_id | string | 会话 ID |
| content | string | 消息内容 |
| timestamp | int64 | 发送时间（Unix 时间戳） |
| status | int16 | 消息状态：`0` 正常，`1` 已撤回 |
| is_edited | bool | 是否已编辑 |

---

### 5.2 获取会话列表

- **URL**: `/api/chat/conversations`
- **Method**: `GET`

获取当前用户的所有会话。

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
        "conversation_id": "100",
        "type": 1,
        "name": "张三",
        "group_number": "0",
        "member_accounts": ["zhangsan", "lisi"],
        "max_seq": 42,
        "max_read_seq": 40,
        "unread_count": 2
      }
    ]
  }
}
```

**conversations 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| conversation_id | string | 会话 ID |
| type | int16 | 会话类型：`1` 单聊，`2` 群聊 |
| name | string | 单聊为对方用户名，群聊为群组名称 |
| group_number | string | 群号，单聊时为 `"0"` |
| member_accounts | string[] | 成员账号列表 |
| max_seq | int64 | 最新消息序号 |
| max_read_seq | int64 | 已读消息序号 |
| unread_count | int64 | 未读消息数 |

---

### 5.3 标记已读

- **URL**: `/api/chat/mark_read/:conversation_id`
- **Method**: `POST`

标记指定会话为已读。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| conversation_id | string | 是 | 会话 ID（路径参数） |

**请求示例**: `/api/chat/mark_read/100`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "max_read_seq": 42
  }
}
```

---

### 5.4 获取在线状态

- **URL**: `/api/chat/online_status`
- **Method**: `POST`

批量查询用户在线状态，最多 100 个。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| accounts | string[] | 是 | 用户账号列表，最多 100 个 |

**请求示例**:
```json
{
  "accounts": ["zhangsan", "lisi", "wangwu"]
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "statuses": [
      {
        "account": "zhangsan",
        "online": true
      },
      {
        "account": "lisi",
        "online": false
      }
    ]
  }
}
```

---

### 5.5 撤回消息

- **URL**: `/api/chat/recall/:msg_id`
- **Method**: `POST`

撤回指定消息。仅发送者可撤回，且有时间限制。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| msg_id | string | 是 | 消息 ID（路径参数） |
| conversation_id | string | 是 | 会话 ID（Body 参数） |

**请求示例**: `/api/chat/recall/123456789`

```json
{
  "conversation_id": "100"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "recalled": true
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "撤回失败",
  "data": "无法撤回该消息，可能已超时或无权限"
}
```

---

### 5.6 编辑消息

- **URL**: `/api/chat/edit/:msg_id`
- **Method**: `POST`

编辑已发送的消息内容。仅发送者可编辑。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| msg_id | string | 是 | 消息 ID（路径参数） |
| conversation_id | string | 是 | 会话 ID（Body 参数） |
| new_content | string | 是 | 新消息内容 |

**请求示例**: `/api/chat/edit/123456789`

```json
{
  "conversation_id": "100",
  "new_content": "修改后的内容"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "edited": true
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "编辑失败",
  "data": "无法编辑该消息，可能已撤回或无权限"
}
```

---

### 5.7 获取编辑历史

- **URL**: `/api/chat/edit_history/:msg_id`
- **Method**: `POST`

获取指定消息的编辑历史记录。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| msg_id | string | 是 | 消息 ID（路径参数） |
| conversation_id | string | 是 | 会话 ID（Body 参数） |

**请求示例**: `/api/chat/edit_history/123456789`

```json
{
  "conversation_id": "100"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "histories": [
      {
        "id": "1",
        "msg_id": "123456789",
        "version": 1,
        "old_content": "原始内容",
        "editor_account": "zhangsan",
        "edited_at": 1700000100
      }
    ]
  }
}
```

**histories 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 编辑记录 ID |
| msg_id | string | 消息 ID |
| version | int32 | 编辑版本号 |
| old_content | string | 编辑前的内容 |
| editor_account | string | 编辑者账号 |
| edited_at | int64 | 编辑时间（Unix 时间戳） |

---

### 5.8 同步消息

- **URL**: `/api/chat/sync`
- **Method**: `POST`

批量同步多个会话的新消息，最多 100 个会话。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| conv_seqs | array | 是 | 会话-序号对列表，每项含 `conversation_id`（string）和 `last_seq`（int64） |
| limit_per_conv | int16 | 否 | 每个会话拉取消息数，默认 50，最大 200 |

**请求示例**:
```json
{
  "conv_seqs": [
    { "conversation_id": "100", "last_seq": 40 },
    { "conversation_id": "200", "last_seq": 10 }
  ],
  "limit_per_conv": 50
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "conv_messages": [
      {
        "conversation_id": "100",
        "messages": [
          {
            "msg_id": "41",
            "client_seq": 1001,
            "sender_account": "zhangsan",
            "sender_name": "张三",
            "conversation_id": "100",
            "content": "新消息",
            "timestamp": 1700000100,
            "seq": 41,
            "status": 0,
            "is_edited": false
          }
        ]
      }
    ]
  }
}
```

---

### 5.9 获取会话成员信息

- **URL**: `/api/chat/conversation_members`
- **Method**: `POST`

批量获取会话成员的详细信息，最多 500 个。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| accounts | string[] | 是 | 用户账号列表，最多 500 个 |

**请求示例**:
```json
{
  "accounts": ["zhangsan", "lisi"]
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "members": [
      {
        "account": "zhangsan",
        "name": "张三",
        "avatar_url": "http://localhost/files/avatars/1.png"
      }
    ]
  }
}
```

**members 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| account | string | 用户账号 |
| name | string | 用户名 |
| avatar_url | string | 头像 URL |

---

## 6. AI 辅助功能

> 以下接口均需 JWT 认证，路径前缀为 `/api`
> AI 功能受速率限制，频繁调用会被限流。

### 6.1 总结会话

- **URL**: `/api/chat/summarize`
- **Method**: `POST`

生成指定会话的消息摘要。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| conversation_id | string | 是 | 会话 ID |

**请求示例**:
```json
{
  "conversation_id": "100"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "summary": "本次对话主要讨论了项目进度安排..."
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "总结失败",
  "data": "无法生成该会话的总结"
}
```

---

### 6.2 生成回复建议

- **URL**: `/api/chat/suggest_replies`
- **Method**: `POST`

根据会话上下文生成回复候选。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| conversation_id | string | 是 | 会话 ID |

**请求示例**:
```json
{
  "conversation_id": "100"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "replies": ["好的，收到！", "我稍后回复你", "这个方案可以"]
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "生成失败",
  "data": "无法生成回复候选"
}
```

---

### 6.3 翻译消息

- **URL**: `/api/chat/translate`
- **Method**: `POST`

将消息内容翻译为目标语言。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| content | string | 是 | 要翻译的消息内容 |
| target_lang | string | 是 | 目标语言（如 `"en"`、`"zh"`、`"ja"`） |

**请求示例**:
```json
{
  "content": "你好，世界",
  "target_lang": "en"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "translated_content": "Hello, world"
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "翻译失败",
  "data": "无法翻译该消息"
}
```

---

## 7. Bot 管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`
> Bot 是 AI 助手实体，可绑定 MCP Server 和知识库来扩展能力。

### 7.1 创建 Bot

- **URL**: `/api/bot/create`
- **Method**: `POST`

创建一个新的 Bot。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | Bot 名称 |
| system_prompt | string | 是 | 系统提示词 |
| api_key | string | 否 | LLM API Key |
| model | string | 是 | 模型名称 |
| base_url | string | 否 | LLM API Base URL |

**请求示例**:
```json
{
  "name": "助手小A",
  "system_prompt": "你是一个有帮助的AI助手",
  "api_key": "sk-xxx",
  "model": "gpt-4o",
  "base_url": "https://api.openai.com/v1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "bot_id": "123456789"
  }
}
```

---

### 7.2 更新 Bot

- **URL**: `/api/bot/update`
- **Method**: `POST`

更新 Bot 信息。仅传入需要修改的字段，未传入的字段保持不变。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| name | string | 否 | Bot 名称 |
| avatar_url | string | 否 | 头像 URL |
| system_prompt | string | 否 | 系统提示词 |
| api_key | string | 否 | LLM API Key |
| model | string | 否 | 模型名称 |
| base_url | string | 否 | LLM API Base URL |

**请求示例**:
```json
{
  "bot_id": "123456789",
  "name": "新名称",
  "model": "gpt-4o-mini"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 7.3 删除 Bot

- **URL**: `/api/bot/delete`
- **Method**: `POST`

删除指定 Bot。系统 Bot 不可删除。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |

**请求示例**:
```json
{
  "bot_id": "123456789"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 7.4 获取 Bot 列表

- **URL**: `/api/bot/list`
- **Method**: `GET`

获取当前用户创建的所有 Bot。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "bots": [
      {
        "bot_id": "123456789",
        "creator_id": "1",
        "name": "助手小A",
        "avatar_url": "http://localhost/files/avatars/bot1.png",
        "system_prompt": "你是一个有帮助的AI助手",
        "model": "gpt-4o",
        "is_system": false,
        "created_at": "1700000000",
        "base_url": "https://api.openai.com/v1"
      }
    ]
  }
}
```

**bots 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| bot_id | string | Bot ID |
| creator_id | string | 创建者用户 ID |
| name | string | Bot 名称 |
| avatar_url | string | 头像 URL |
| system_prompt | string | 系统提示词 |
| model | string | 模型名称 |
| is_system | bool | 是否为系统 Bot |
| created_at | string | 创建时间（Unix 时间戳字符串） |
| base_url | string | LLM API Base URL（可选） |

---

### 7.5 将 Bot 拉入会话

- **URL**: `/api/bot/add_to_conversation`
- **Method**: `POST`

将 Bot 添加到指定会话中。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| conversation_id | string | 否 | 会话 ID |
| conversation_type | int16 | 是 | 会话类型：`1` 单聊，`2` 群聊 |

**请求示例**:
```json
{
  "bot_id": "123456789",
  "conversation_id": "100",
  "conversation_type": 2
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true,
    "conversation_id": "100"
  }
}
```

---

### 7.6 获取系统 Bot

- **URL**: `/api/bot/system`
- **Method**: `GET`

获取系统预置的 Bot 信息。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true,
    "bot_id": "1"
  }
}
```

---

## 8. MCP Server 管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`
> MCP（Model Context Protocol）Server 用于扩展 Bot 的工具调用能力。系统 Bot 不支持添加 MCP Server。

### 8.1 创建 MCP Server

- **URL**: `/api/bot/mcp/create`
- **Method**: `POST`

为指定 Bot 添加一个 MCP Server。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| name | string | 是 | MCP Server 名称 |
| url | string | 是 | MCP Server 地址 |
| description | string | 否 | 描述 |
| transport | string | 否 | 传输方式（如 `"sse"`、`"stdio"`） |
| auth_type | string | 否 | 认证类型 |
| auth_token | string | 否 | 认证令牌 |

**请求示例**:
```json
{
  "bot_id": "123456789",
  "name": "天气查询",
  "url": "http://mcp-weather:8080/sse",
  "description": "查询天气信息",
  "transport": "sse"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "id": "1",
    "success": true
  }
}
```

---

### 8.2 获取 Bot 的 MCP Server 列表

- **URL**: `/api/bot/mcp/list`
- **Method**: `GET`

获取指定 Bot 绑定的所有 MCP Server。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID（Query 参数） |

**请求示例**: `/api/bot/mcp/list?bot_id=123456789`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "servers": [
      {
        "id": "1",
        "bot_id": "123456789",
        "name": "天气查询",
        "description": "查询天气信息",
        "transport": "sse",
        "url": "http://mcp-weather:8080/sse",
        "auth_type": "",
        "enabled": true,
        "created_at": "1700000000"
      }
    ]
  }
}
```

**servers 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | MCP Server ID |
| bot_id | string | 所属 Bot ID |
| name | string | 名称 |
| description | string | 描述 |
| transport | string | 传输方式 |
| url | string | 地址 |
| auth_type | string | 认证类型 |
| enabled | bool | 是否启用 |
| created_at | string | 创建时间（Unix 时间戳字符串） |

---

### 8.3 更新 MCP Server

- **URL**: `/api/bot/mcp/update`
- **Method**: `POST`

更新 MCP Server 配置。仅传入需要修改的字段。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | MCP Server ID |
| name | string | 否 | 名称 |
| description | string | 否 | 描述 |
| transport | string | 否 | 传输方式 |
| url | string | 否 | 地址 |
| auth_type | string | 否 | 认证类型 |
| auth_token | string | 否 | 认证令牌 |
| enabled | bool | 否 | 是否启用 |

**请求示例**:
```json
{
  "id": "1",
  "name": "天气查询V2",
  "enabled": true
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 8.4 删除 MCP Server

- **URL**: `/api/bot/mcp/delete`
- **Method**: `POST`

删除指定的 MCP Server。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | MCP Server ID |

**请求示例**:
```json
{
  "id": "1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

## 9. 知识库管理

> 以下接口均需 JWT 认证，路径前缀为 `/api`
> 知识库用于 RAG（检索增强生成），Bot 绑定知识库后可基于知识库内容回答问题。

### 9.1 创建知识库

- **URL**: `/api/knowledge/create`
- **Method**: `POST`

创建一个新的知识库。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 知识库名称 |
| description | string | 否 | 描述 |

**请求示例**:
```json
{
  "name": "项目文档",
  "description": "包含项目相关文档"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "kb_id": "1"
  }
}
```

---

### 9.2 获取知识库详情

- **URL**: `/api/knowledge/get`
- **Method**: `GET`

获取指定知识库的详细信息。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| kb_id | string | 是 | 知识库 ID（Query 参数） |

**请求示例**: `/api/knowledge/get?kb_id=1`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "kb_id": "1",
    "owner_id": "1",
    "name": "项目文档",
    "description": "包含项目相关文档",
    "doc_count": 5,
    "chunk_count": 120,
    "created_at": "1700000000"
  }
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| kb_id | string | 知识库 ID |
| owner_id | string | 所有者用户 ID |
| name | string | 知识库名称 |
| description | string | 描述 |
| doc_count | int32 | 文档数量 |
| chunk_count | int32 | 分块数量 |
| created_at | string | 创建时间（Unix 时间戳字符串） |

---

### 9.3 获取用户知识库列表

- **URL**: `/api/knowledge/list`
- **Method**: `GET`

获取当前用户的所有知识库。

**请求参数**: 无

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "knowledge_bases": [
      {
        "kb_id": "1",
        "owner_id": "1",
        "name": "项目文档",
        "description": "包含项目相关文档",
        "doc_count": 5,
        "chunk_count": 120,
        "created_at": "1700000000"
      }
    ]
  }
}
```

---

### 9.4 更新知识库

- **URL**: `/api/knowledge/update`
- **Method**: `POST`

更新知识库信息。仅传入需要修改的字段。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| kb_id | string | 是 | 知识库 ID |
| name | string | 否 | 知识库名称 |
| description | string | 否 | 描述 |

**请求示例**:
```json
{
  "kb_id": "1",
  "name": "项目文档V2"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 9.5 删除知识库

- **URL**: `/api/knowledge/delete`
- **Method**: `POST`

删除指定知识库及其所有文档。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| kb_id | string | 是 | 知识库 ID |

**请求示例**:
```json
{
  "kb_id": "1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 9.6 添加文档

- **URL**: `/api/knowledge/document/add`
- **Method**: `POST`

向知识库添加一个文档。文档将异步解析并分块入库。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| kb_id | string | 是 | 知识库 ID |
| file_name | string | 是 | 文件名 |
| file_url | string | 是 | 文件 URL（通过上传接口获取） |
| file_type | string | 是 | 文件类型（如 `"pdf"`、`"txt"`、`"md"`） |
| file_size | int64 | 否 | 文件大小（字节） |

**请求示例**:
```json
{
  "kb_id": "1",
  "file_name": "项目说明.pdf",
  "file_url": "http://localhost/files/docs/project.pdf",
  "file_type": "pdf",
  "file_size": 1048576
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "doc_id": "1"
  }
}
```

---

### 9.7 获取文档列表

- **URL**: `/api/knowledge/document/list`
- **Method**: `GET`

获取指定知识库的文档列表。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| kb_id | string | 是 | 知识库 ID（Query 参数） |

**请求示例**: `/api/knowledge/document/list?kb_id=1`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "documents": [
      {
        "doc_id": "1",
        "kb_id": "1",
        "file_name": "项目说明.pdf",
        "file_url": "http://localhost/files/docs/project.pdf",
        "file_type": "pdf",
        "file_size": 1048576,
        "status": "completed",
        "chunk_count": 25,
        "created_at": "1700000000"
      }
    ]
  }
}
```

**documents 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| doc_id | string | 文档 ID |
| kb_id | string | 知识库 ID |
| file_name | string | 文件名 |
| file_url | string | 文件 URL |
| file_type | string | 文件类型 |
| file_size | int64 | 文件大小（字节） |
| status | string | 解析状态：`"pending"` 待处理、`"processing"` 处理中、`"completed"` 已完成、`"failed"` 失败 |
| chunk_count | int32 | 分块数量 |
| error_message | string | 错误信息（仅失败时存在） |
| created_at | string | 创建时间（Unix 时间戳字符串） |

---

### 9.8 删除文档

- **URL**: `/api/knowledge/document/delete`
- **Method**: `POST`

删除指定文档。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| doc_id | string | 是 | 文档 ID |

**请求示例**:
```json
{
  "doc_id": "1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 9.9 重试文档解析

- **URL**: `/api/knowledge/document/retry`
- **Method**: `POST`

重新解析失败的文档。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| doc_id | string | 是 | 文档 ID |

**请求示例**:
```json
{
  "doc_id": "1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 9.10 绑定知识库到 Bot

- **URL**: `/api/knowledge/bind`
- **Method**: `POST`

将知识库绑定到指定 Bot。系统 Bot 不支持绑定知识库。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| kb_id | string | 是 | 知识库 ID |

**请求示例**:
```json
{
  "bot_id": "123456789",
  "kb_id": "1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 9.11 解绑知识库

- **URL**: `/api/knowledge/unbind`
- **Method**: `POST`

将知识库从 Bot 解绑。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| kb_id | string | 是 | 知识库 ID |

**请求示例**:
```json
{
  "bot_id": "123456789",
  "kb_id": "1"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 9.12 获取 Bot 绑定的知识库列表

- **URL**: `/api/knowledge/bot_bases`
- **Method**: `GET`

获取指定 Bot 绑定的所有知识库。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID（Query 参数） |

**请求示例**: `/api/knowledge/bot_bases?bot_id=123456789`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "knowledge_bases": [
      {
        "kb_id": "1",
        "owner_id": "1",
        "name": "项目文档",
        "description": "包含项目相关文档",
        "doc_count": 5,
        "chunk_count": 120,
        "created_at": "1700000000"
      }
    ]
  }
}
```

---

## 10. 文件上传与代理

> 以下接口均需 JWT 认证，路径前缀为 `/api`

### 10.1 上传文件

- **URL**: `/api/files`
- **Method**: `POST`

上传文件到对象存储。支持 `multipart/form-data` 格式，文件大小限制 50MB。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| file | file | 是 | 上传的文件（multipart form） |

**请求示例**（curl）:
```bash
curl -X POST http://localhost/api/files \
  -H "Authorization: <token>" \
  -F "file=@/path/to/file.pdf"
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "url": "http://localhost/files/docs/1_file.pdf",
    "file_name": "file.pdf",
    "file_size": 1048576,
    "media_type": "document"
  }
}
```

**data 字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| url | string | 文件访问 URL |
| file_name | string | 文件名 |
| file_size | int64 | 文件大小（字节） |
| media_type | string | 媒体类型：`"image"`、`"video"`、`"audio"`、`"document"` |

---

### 10.2 文件代理访问

- **URL**: `/files/*filepath`
- **Method**: `GET`
- **认证**: 不需要

代理访问存储在 SeaweedFS 中的文件。响应头包含 `Cache-Control: public, max-age=86400`。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| filepath | string | 是 | 文件路径（路径参数） |

**请求示例**: `/files/docs/1_file.pdf`

**响应**: 直接返回文件内容，Content-Type 根据文件类型自动设置。

---

## 11. Bot 市场

> 用户端接口均需 JWT 认证，路径前缀为 `/api`；管理端接口前缀为 `/api/admin`，使用独立的管理后台登录态。
> 市场只分发 Bot 的**定义**（名称、头像、描述、提示词、推荐模型、分类标签），
> **不包含知识库与 MCP**；安装者需自备 `api_key` / `model` / `base_url`。

### 11.0 审核状态机

市场条目按以下状态流转，`status` 字段取值如下：

| status | 含义 | 说明 |
|--------|------|------|
| 0 | 草稿 | 保存但未提交审核 |
| 1 | 待审核 | 用户发布或修改提示词后进入该状态 |
| 2 | 已发布 | 审核通过，市场全局可见 |
| 3 | 已驳回 | 审核未通过，`review_note` 记录原因 |
| 4 | 已下架 | 发布者主动下架 |

流转规则：

- 发布（`POST /api/market/bots`）后进入 **1 待审核**，`version` 从 0 起算。
- 审核通过后进入 **2 已发布**，`version` 递增（首次上架得到 v1）。
- 修改条目时，若改动 `system_prompt` 则重新回到 **1 待审核**（提示词是核心分发内容，必须重新过审）。
- 驳回进入 **3 已驳回**；发布者下架进入 **4 已下架**。
- 「我的发布」可通过 `status` 查询；传负值（如 `-1`）表示不过滤，一次性返回全部状态。

### 11.1 浏览市场 Bot 列表

- **URL**: `/api/market/bots`
- **Method**: `GET`

分页浏览已发布（status=2）的市场条目，全局可见，按安装量倒序返回。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| keyword | string | 否 | 关键词，匹配名称/描述/标签 |
| category | string | 否 | 分类过滤 |
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20，最大 100 |

**请求示例**: `/api/market/bots?keyword=周报&page=1&page_size=20`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "bots": [
      {
        "market_bot_id": "123456789",
        "publisher_id": "7",
        "publisher_name": "张三",
        "name": "周报助手",
        "description": "把一周的工作整理成周报",
        "avatar_url": "http://localhost/files/avatars/1.png",
        "system_prompt": "你是一个周报助手……",
        "model": "gpt-4o-mini",
        "category": "效率",
        "tags": "周报,办公",
        "status": 2,
        "review_note": "",
        "install_count": 128,
        "rating": 4.6,
        "rating_count": 32,
        "version": 2,
        "created_at": "1700000000000",
        "updated_at": "1700000000000"
      }
    ],
    "total": 1
  }
}
```

---

### 11.2 查看市场 Bot 详情

- **URL**: `/api/market/bots/:market_bot_id`
- **Method**: `GET`

查看条目详情。未发布（待审核/驳回/下架）内容仅发布者本人可见，其他人返回「条目不存在」。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID（路径参数） |

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "bot": {
      "market_bot_id": "123456789",
      "publisher_id": "7",
      "publisher_name": "张三",
      "name": "周报助手",
      "description": "把一周的工作整理成周报",
      "avatar_url": "",
      "system_prompt": "你是一个周报助手……",
      "model": "gpt-4o-mini",
      "category": "效率",
      "tags": "周报,办公",
      "status": 2,
      "review_note": "",
      "install_count": 128,
      "rating": 4.6,
      "rating_count": 32,
      "version": 2,
      "created_at": "1700000000000",
      "updated_at": "1700000000000"
    }
  }
}
```

失败（不存在或不可见）：
```json
{
  "code": 1,
  "msg": "条目不存在",
  "data": "该 Bot 不存在或不可见"
}
```

---

### 11.3 我的发布

- **URL**: `/api/market/my-published`
- **Method**: `GET`

查看自己发布的条目（含草稿/待审/驳回/下架）。数据按「租户 + 发布者」隔离，仅本人可见。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| status | int | 否 | 状态过滤；不传或传负值表示返回全部状态 |
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20，最大 100 |

**响应示例**: 与 11.1 相同（`data.bots` + `data.total`）。

---

### 11.4 发布 Bot 到市场

- **URL**: `/api/market/bots`
- **Method**: `POST`

把自己的 Bot 定义提交到市场，提交后进入**待审核**状态。提示词会先做安全预审，危险内容会被拦截。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | Bot 名称，最长 64 字符 |
| system_prompt | string | 是 | 系统提示词 |
| description | string | 否 | 描述，最长 512 字符 |
| avatar_url | string | 否 | 头像 URL |
| model | string | 否 | 推荐模型 |
| base_url | string | 否 | 推荐 Base URL |
| category | string | 否 | 分类，最长 64 字符 |
| tags | string | 否 | 标签，逗号分隔，最长 255 字符 |

**请求示例**:
```json
{
  "name": "周报助手",
  "system_prompt": "你是一个周报助手……",
  "description": "把一周的工作整理成周报",
  "model": "gpt-4o-mini",
  "category": "效率",
  "tags": "周报,办公"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "market_bot_id": "123456789",
    "status": 1
  }
}
```

---

### 11.5 更新我的发布

- **URL**: `/api/market/bots/update`
- **Method**: `POST`

修改自己发布的条目，仅传需要改动的字段。改动 `system_prompt` 会重新进入待审核。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID |
| name | string | 否 | Bot 名称 |
| description | string | 否 | 描述 |
| avatar_url | string | 否 | 头像 URL |
| system_prompt | string | 否 | 系统提示词（改动后重新过审） |
| model | string | 否 | 推荐模型 |
| base_url | string | 否 | 推荐 Base URL |
| category | string | 否 | 分类 |
| tags | string | 否 | 标签 |

**请求示例**:
```json
{
  "market_bot_id": "123456789",
  "description": "新的描述"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 11.6 下架我的发布

- **URL**: `/api/market/bots/offline`
- **Method**: `POST`

发布者主动下架自己的条目，状态置为 4 已下架。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID |

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 11.7 安装市场 Bot

- **URL**: `/api/market/bots/install`
- **Method**: `POST`

用自己的 `api_key` 把市场 Bot 安装为自己的 Bot 实例（`origin=market`）。
同一用户在相同租户内重复安装同一条目是幂等的，会返回既有实例。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID |
| name | string | 否 | 安装后的 Bot 名称，不传则用市场条目名称 |
| api_key | string | 是 | 安装者自己的 LLM API Key |
| model | string | 是 | 安装时使用的模型 |
| base_url | string | 否 | 安装时使用的 Base URL |

**请求示例**:
```json
{
  "market_bot_id": "123456789",
  "name": "我的周报助手",
  "api_key": "sk-xxx",
  "model": "gpt-4o-mini"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "bot_id": "223344556677",
    "version": 2
  }
}
```

---

### 11.8 给市场 Bot 评分

- **URL**: `/api/market/bots/rate`
- **Method**: `POST`

对已发布的市场 Bot 评分（1-5）。同一用户重复评分视为改分，评分人数不重复累加。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID |
| score | int | 是 | 评分，取值 1-5 |

**请求示例**:
```json
{
  "market_bot_id": "123456789",
  "score": 5
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 11.9 条目字段说明

11.1 / 11.2 / 11.3 / 11.9 返回的市场条目字段如下：

| 字段 | 类型 | 说明 |
|------|------|------|
| market_bot_id | string | 市场 Bot ID（字符串化 int64） |
| publisher_id | string | 发布者用户 ID（字符串化 int64） |
| publisher_name | string | 发布者昵称 |
| name | string | Bot 名称 |
| description | string | 描述 |
| avatar_url | string | 头像 URL |
| system_prompt | string | 系统提示词 |
| model | string | 推荐模型 |
| base_url | string | 推荐 Base URL，空时不返回（`omitempty`） |
| category | string | 分类 |
| tags | string | 标签 |
| status | int | 审核状态，见 11.0 |
| review_note | string | 审核意见（驳回原因） |
| install_count | int64 | 安装量 |
| rating | float64 | 平均评分（无评分时为 0） |
| rating_count | int | 评分人数 |
| version | int | 版本号，审核通过后递增 |
| created_at | string | 创建时间（毫秒时间戳，字符串） |
| updated_at | string | 更新时间（毫秒时间戳，字符串） |

---

### 11.10 管理端：审核列表

- **URL**: `/api/admin/market/reviews`
- **Method**: `GET`
- **认证**: 管理后台 JWT（见 11.13）

查看市场审核队列，跨租户可见。未传 `status` 时默认查看待审核（status=1）。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| status | int | 否 | 状态过滤，默认 1 待审核 |
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20，最大 100 |

**响应示例**: 与 11.1 相同（`data.bots` + `data.total`）。

---

### 11.11 管理端：审核通过

- **URL**: `/api/admin/market/reviews/:market_bot_id/approve`
- **Method**: `POST`

审核通过，条目上架（status=2），`version` 递增。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID（路径参数） |
| note | string | 否 | 审核备注，最长 255 字符 |

**请求示例**:
```json
{
  "note": "内容合规"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 11.12 管理端：审核驳回

- **URL**: `/api/admin/market/reviews/:market_bot_id/reject`
- **Method**: `POST`

审核驳回，条目置为已驳回（status=3），`review_note` 记录原因。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| market_bot_id | string | 是 | 市场 Bot ID（路径参数） |
| note | string | 否 | 驳回原因，最长 255 字符 |

**请求示例**:
```json
{
  "note": "提示词包含违规内容"
}
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "success": true
  }
}
```

---

### 11.13 管理端登录态说明

管理后台使用独立账号体系，与普通用户登录态完全隔离：

| 项 | 说明 |
|----|------|
| 登录接口 | `POST /api/admin/login` |
| 请求体 | `{"account": "admin", "password": "******"}` |
| 响应 | `{"token": "...", "name": "初始管理员", "role": "super"}` |
| 携带方式 | Header `Authorization: <token>` |
| 有效期 | 12 小时 |

后续所有 `/api/admin/*` 接口均需携带该 `token`。

---

## 12. 日/周总结

> 系统 Agent 会在每天 **21:00（东八区）** 为用户生成「今日总结」，并在 **每周日 21:00** 额外生成「本周总结」。
> 总结聚合用户**全部会话**（私聊与群聊、跨租户）的消息，由系统 Bot 通过「用户 ⇄ 系统 Bot」私聊投递。
> 当天/当周没有消息时，系统仍会发送占位文案「今日无话可说」/「本周无话可说」。
> 所有总结接口均需登录态，且**仅本人可见**：服务端始终以登录态用户 ID 为准，不接受请求体传入的 `user_id`。

### 12.1 读取总结列表

- **URL**: `/api/summary/list`
- **Method**: `GET`

返回当前用户的全部总结（可按类型过滤），按统计窗口倒序排列。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| type | string | 否 | 过滤类型：`daily` / `weekly`；不传表示不过滤 |
| limit | int | 否 | 每页条数，默认 20，最大 100 |
| offset | int | 否 | 偏移量，默认 0 |

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "reports": [
      {
        "id": "1234567890",
        "user_id": "7",
        "type": "daily",
        "period_start": 1760000000000,
        "period_end": 1760086399999,
        "content": "**今日概览**\n今天主要……",
        "model": "gpt-4o-mini",
        "status": 2,
        "error": "",
        "created_at": 1760086400000
      }
    ]
  }
}
```

字段说明：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 总结报告 ID |
| user_id | string | 报告归属用户 ID |
| type | string | `daily` / `weekly` |
| period_start | int64 | 统计窗口起始（毫秒时间戳） |
| period_end | int64 | 统计窗口结束（毫秒时间戳） |
| content | string | 总结正文（Markdown） |
| model | string | 生成所用模型；占位文案为空 |
| status | int | 0 待生成 / 1 生成中 / 2 已完成 / 3 生成失败 |
| error | string | 失败原因（status=3 时非空） |
| created_at | int64 | 创建时间（毫秒时间戳） |

### 12.2 读取日总结列表

- **URL**: `/api/summary/daily`
- **Method**: `GET`

等价于 `GET /api/summary/list?type=daily`，支持 `limit` / `offset`。

### 12.3 读取周总结列表

- **URL**: `/api/summary/weekly`
- **Method**: `GET`

等价于 `GET /api/summary/list?type=weekly`，支持 `limit` / `offset`。

### 12.4 读取总结详情

- **URL**: `/api/summary/detail`
- **Method**: `GET`

读取本人某份总结的完整正文。报告中 `user_id` 与登录态不一致时按「无权查看」处理。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | 总结报告 ID |

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "report": {
      "id": "1234567890",
      "user_id": "7",
      "type": "daily",
      "period_start": 1760000000000,
      "period_end": 1760086399999,
      "content": "**今日概览**\n今天主要……",
      "model": "gpt-4o-mini",
      "status": 2,
      "error": "",
      "created_at": 1760086400000
    }
  }
}
```

失败（报告不存在或无权查看）：
```json
{
  "code": 1,
  "msg": "总结不存在",
  "data": "未找到该总结或无权查看"
}
```

### 12.5 手动生成总结

- **URL**: `/api/summary/generate`
- **Method**: `POST`

立即为当前用户生成一份总结。同一 `(user_id, type, 统计窗口起点)` 已生成成功时，**复用旧报告，不重复生成**。
同一用户 30 秒内最多调用一次（限流）。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| type | string | 否 | `daily` / `weekly`，默认 `daily` |

**请求示例**:
```json
{ "type": "weekly" }
```

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "report": { "id": "1234567891", "type": "weekly", "status": 2, "content": "……" }
  }
}
```

### 12.6 读取总结订阅

- **URL**: `/api/summary/schedule`
- **Method**: `GET`

读取当前用户的日/周总结订阅开关。未设置过时返回默认值（均为开启）。

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "daily_enabled": true,
    "weekly_enabled": true
  }
}
```

### 12.7 更新总结订阅

- **URL**: `/api/summary/schedule`
- **Method**: `POST`

写入当前用户的日/周总结订阅开关；关闭后定时调度不会为其生成对应类型的总结（手动生成不受影响）。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| daily_enabled | bool | 是 | 是否开启每日总结 |
| weekly_enabled | bool | 是 | 是否开启每周总结 |

**请求示例**:
```json
{ "daily_enabled": true, "weekly_enabled": false }
```

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "daily_enabled": true,
    "weekly_enabled": false
  }
}
```

---

## 13. Token 用量与成本核算

> 本章统计用户**自带 apikey** 调用大模型所产生的 Token 消耗与**预估成本**（记账口径，非售卖服务）。
> 用量看板**仅本人可见**：服务端始终以登录态用户 ID 为准，不接受请求体传入的 `user_id`。
> 归集规则：市场安装的 Bot 计入安装者；用户自建的 Bot 计入发送者；**通过 OpenAPI 接入的 Bot 不计入**。
> 参考单价由平台管理员在管理后台**手工维护**，按每 1K Token 计价；未配置单价的模型只记录 Token 用量，成本计 0 并在汇总中提示。
> 成本为**预估值，仅供参考**。

### 13.1 用量明细

- **URL**: `/api/billing/usage`
- **Method**: `GET`

返回当前登录用户的用量明细，可按日期、模型、Bot 过滤，按时间倒序分页。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| start_date | string | 否 | 起始自然日（东八区），格式 `yyyy-mm-dd` |
| end_date | string | 否 | 结束自然日（东八区），格式 `yyyy-mm-dd`，含当日 |
| model | string | 否 | 按模型名过滤 |
| bot_id | int | 否 | 按 Bot ID 过滤 |
| limit | int | 否 | 每页条数，默认 20 |
| offset | int | 否 | 偏移量，默认 0 |

**请求示例**: `/api/billing/usage?start_date=2026-10-01&end_date=2026-10-12&limit=20`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "items": [
      {
        "id": "1000000001",
        "biz_date": "2026-10-12",
        "bot_id": "123456789",
        "origin": 0,
        "conversation_id": "555",
        "msg_id": "987654321",
        "model": "gpt-4o-mini",
        "prompt_tokens": 1024,
        "completion_tokens": 512,
        "cost": 0.0031,
        "created_at": 1760256000000
      }
    ],
    "total": 1
  }
}
```

字段说明：

| 字段 | 类型 | 说明 |
|------|------|------|
| origin | int | Bot 来源：`0` 用户自建、`1` 市场安装、`2` OpenAPI（不计入统计） |
| cost | float | 该次调用的预估成本，按记录的单价快照计算；无单价时为 0 |

### 13.2 用量汇总

- **URL**: `/api/billing/usage/summary`
- **Method**: `GET`

返回当前登录用户在区间内的合计、按天趋势、按 Bot / 按模型占比，以及未配置单价的调用次数。

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| start_date | string | 否 | 起始自然日（东八区），格式 `yyyy-mm-dd` |
| end_date | string | 否 | 结束自然日（东八区），格式 `yyyy-mm-dd`，含当日 |

**请求示例**: `/api/billing/usage/summary?start_date=2026-10-06&end_date=2026-10-12`

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "prompt_tokens": 120000,
    "completion_tokens": 48000,
    "cost": 0.42,
    "request_count": 86,
    "missing_price_count": 2,
    "trend": [
      { "biz_date": "2026-10-12", "prompt_tokens": 12000, "completion_tokens": 4000, "cost": 0.05, "request_count": 9 }
    ],
    "by_model": [
      { "key": "gpt-4o-mini", "prompt_tokens": 120000, "completion_tokens": 48000, "cost": 0.42, "request_count": 86 }
    ],
    "by_bot": [
      { "key": "123456789", "prompt_tokens": 120000, "completion_tokens": 48000, "cost": 0.42, "request_count": 86 }
    ]
  }
}
```

字段说明：

| 字段 | 类型 | 说明 |
|------|------|------|
| missing_price_count | int | 期间内未匹配到参考单价的调用次数（这些调用成本计 0） |
| trend | array | 按自然日的用量趋势（东八区） |
| by_model / by_bot | array | 按模型 / 按 Bot 聚合，`key` 为模型名或 Bot ID |

---

## 14. 管理后台：参考单价

> 管理端接口前缀为 `/api/admin`，使用**独立的平台管理员登录态**，与用户 JWT 完全隔离。
> 参考单价用于估算用户自带 apikey 的 Token 成本，仅作记账口径，按每 1K Token 手工维护。

### 14.1 参考单价列表

- **URL**: `/api/admin/billing/prices`
- **Method**: `GET`

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| only_enabled | bool | 否 | 传 `1` / `true` 时仅返回启用的单价 |
| limit | int | 否 | 每页条数 |
| offset | int | 否 | 偏移量 |

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "prices": [
      {
        "id": "1",
        "provider": "openai",
        "model": "gpt-4o-mini",
        "input_per_1k": 0.0011,
        "output_per_1k": 0.0044,
        "currency": "CNY",
        "enabled": true,
        "created_at": 1759257600000,
        "updated_at": 1759257600000
      }
    ],
    "total": 1
  }
}
```

### 14.2 新增 / 更新参考单价

- **URL**: `/api/admin/billing/prices`
- **Method**: `POST`

同一模型号存在时更新，否则新建；操作均写入审计日志。

**请求参数**（JSON Body）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 否 | 更新时传已有单价 ID；不传表示新建 |
| provider | string | 否 | 提供方 |
| model | string | 是 | 模型名 |
| input_per_1k | float | 是 | 输入每 1K Token 单价 |
| output_per_1k | float | 是 | 输出每 1K Token 单价 |
| currency | string | 否 | 币种，默认 `CNY` |
| enabled | bool | 否 | 是否启用，默认启用 |

**请求示例**:
```json
{ "provider": "openai", "model": "gpt-4o-mini", "input_per_1k": 0.0011, "output_per_1k": 0.0044, "currency": "CNY", "enabled": true }
```

**响应示例**:
```json
{ "code": 0, "msg": "操作成功", "data": { "success": true } }
```

### 14.3 删除参考单价

- **URL**: `/api/admin/billing/prices/delete`
- **Method**: `POST`

**请求参数**（JSON Body）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | 参考单价 ID |

**请求示例**:
```json
{ "id": "1" }
```

**响应示例**:
```json
{ "code": 0, "msg": "操作成功", "data": { "success": true } }
```

---

## 15. 多 Agent 协作编排

> 接口前缀 `/api`，均需 JWT 认证；编排图按 **Bot 维度 + 当前租户** 隔离，一个 Bot 只保留一份编排图。
> 发布后该 Bot 的对话由 `work_service` 按编排图分层执行；未发布或执行异常时自动回退单 Agent，不影响正常回复。

### 15.0 状态机与执行规则

编排图 `status` 取值：

| status | 含义 | 说明 |
|--------|------|------|
| 0 | 草稿 | 保存后即进入该状态，可继续编辑，不对对话生效 |
| 1 | 已发布 | 该 Bot 的对话按图执行 |
| 2 | 已停用 | 停用后按回退逻辑走单 Agent |

- 每次保存都会把状态重置为 **0 草稿** 并递增 `version`，需要重新发布才对对话生效。
- 每次保存都会写入一条历史快照（全量画布 JSON），默认最多返回最近 20 条，上限 50 条。
- 运行时按拓扑分层串行推进：同一层节点按声明顺序执行，前驱节点的输出会作为上下文注入后继节点，`end` 节点负责汇聚结果。

### 15.1 编排图 schema

画布序列化为如下结构的 JSON 字符串，通过 `graph_json` 传入（字段名与后端 `pkg/agentgraph` 一致）：

```json
{
  "nodes": [
    { "id": "start", "type": "start", "name": "开始", "x": 60, "y": 60 },
    { "id": "agent_1", "type": "agent", "name": "资料检索", "system_prompt": "你负责检索并整理资料", "model": "", "tools": ["knowledge"], "max_steps": 3, "x": 290, "y": 60 },
    { "id": "end", "type": "end", "name": "结束", "system_prompt": "", "model": "", "x": 520, "y": 60 }
  ],
  "edges": [
    { "from": "start", "to": "agent_1" },
    { "from": "agent_1", "to": "end" }
  ]
}
```

**节点字段**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 节点唯一标识，非空且不重复 |
| type | string | 是 | 节点类型：`start` 触发 / `agent` 专家 / `end` 输出 |
| name | string | 否 | 节点名称，最长 64 字 |
| system_prompt | string | 否 | 节点提示词，最长 4000 字；`start` 节点忽略该字段 |
| model | string | 否 | 该节点使用的模型，留空继承 Bot 的模型 |
| tools | string[] | 否 | 工具白名单，仅 `agent` 节点生效 |
| max_steps | int | 否 | 该节点 ReAct 步数上限，0 表示用默认值，最大 6 |
| x / y | int | 否 | 画布坐标，仅用于前端回显，后端透传不参与校验 |

**工具取值**（`tools`）:

| 取值 | 含义 |
|------|------|
| 不传 / `[]` | 继承 Bot 的默认工具（绑定的知识库 + MCP） |
| `["none"]` | 禁用全部工具 |
| `["knowledge"]` | 允许检索 Bot 绑定的知识库 |
| `["memory"]` | 允许读写长期记忆 |

**连线字段**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| from | string | 是 | 起点节点 id，必须存在 |
| to | string | 是 | 终点节点 id，必须存在 |

### 15.2 校验规则

校验逻辑集中在 `pkg/agentgraph`，「保存 / 校验 / 发布 / 运行前」共用同一套规则：

| 规则 | 说明 |
|------|------|
| 规模上限 | 节点 ≤ 12，连线 ≤ 40 |
| 开始节点 | 必须**有且仅有 1 个** `start` 节点 |
| 结束节点 | **至少 1 个** `end` 节点，且 `end` 不能有出边 |
| 入边约束 | `start` 不能有入边；节点不能连接自身 |
| 无环 | 必须是有向无环图（DAG），存在环时报错 |
| 可达性 | 所有节点都必须能沿连线从 `start` 到达 |
| 工具白名单 | `tools` 只能取 `none` / `knowledge` / `memory` 或留空 |
| 内容上限 | `name` ≤ 64 字，`system_prompt` ≤ 4000 字，`max_steps` ∈ [0, 6] |

> 保存与发布的行为差异：**保存**允许「JSON 可解析但校验未通过」的草稿落库，并在响应里返回 `errors` 供画布提示；**发布**要求校验完全通过，否则状态不变并返回错误列表。

### 15.3 查询编排图

- **URL**: `/api/bot/agent-graph`
- **Method**: `GET`

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |

**响应示例**（已存在）:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "exists": true,
    "graph": {
      "graph_id": "123456789",
      "bot_id": "88",
      "name": "调研小组",
      "graph": { "nodes": [], "edges": [] },
      "status": 1,
      "version": 3,
      "created_at": "1759257600000",
      "updated_at": "1759257600000"
    }
  }
}
```

不存在时返回 `data.exists = false`，且不返回 `graph` 字段。

### 15.4 保存编排图（草稿）

- **URL**: `/api/bot/agent-graph/save`
- **Method**: `POST`

**请求参数**（JSON Body）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| graph_json | string | 是 | 编排图 JSON 字符串，见 15.1 |
| name | string | 否 | 编排图名称，缺省为「未命名编排」 |
| graph_id | string | 否 | 指定要更新的图 ID，一般不传（按 Bot 自动 upsert） |

**请求示例**:
```json
{ "bot_id": "88", "name": "调研小组", "graph_json": "{\"nodes\":[],\"edges\":[]}" }
```

**响应示例**:
```json
{ "code": 0, "msg": "操作成功", "data": { "success": true, "graph_id": "123456789", "version": 2, "status": 0, "errors": [] } }
```

| 字段 | 说明 |
|------|------|
| success | 是否完成落库；`false` 表示 `graph_json` 无法解析（未落库） |
| graph_id | 编排图 ID，`0` 表示未落库 |
| version | 保存后的版本号 |
| status | 保存后固定为 `0`（草稿） |
| errors | 校验未通过的错误列表，可随成功响应一起返回 |

### 15.5 校验编排图

- **URL**: `/api/bot/agent-graph/validate`
- **Method**: `POST`

只做结构与内容校验，不落库，供画布实时提示。

**请求参数**（JSON Body）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| graph_json | string | 是 | 待校验的编排图 JSON |

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "valid": false,
    "errors": ["编排图必须且只能有一个开始节点", "连线引用了不存在的终点: agent_9"]
  }
}
```

### 15.6 发布编排图

- **URL**: `/api/bot/agent-graph/publish`
- **Method**: `POST`

**请求参数**（JSON Body）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |

**响应示例**（发布成功）:
```json
{ "code": 0, "msg": "操作成功", "data": { "success": true } }
```

**响应示例**（校验未通过）:
```json
{ "code": 0, "msg": "操作成功", "data": { "success": false, "errors": ["编排图至少需要一个结束节点"] } }
```

### 15.7 历史版本

- **URL**: `/api/bot/agent-graph/history`
- **Method**: `GET`

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |
| limit | int | 否 | 返回条数，默认 20，最大 50 |

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "items": [
      {
        "id": "987654321",
        "graph_id": "123456789",
        "version": 2,
        "graph": { "nodes": [], "edges": [] },
        "operator_id": "7",
        "created_at": "1759257600000"
      }
    ]
  }
}
```

历史快照仅用于前端回滚：载入某个版本后需要再次调用保存接口才会生效（保存会把状态重置为草稿）。

### 15.8 运行与降级

- 只有状态为 **1 已发布** 的编排图才会在对话中生效；其余情况（无图、草稿、已停用、执行失败、超时）自动回退单 Agent，回复链路不中断。
- 整图执行超时默认 240 秒，可通过 `work_service` 的 `ai.agent_graph.timeout_seconds` 调整；`ai.agent_graph.enabled=false` 可整体关闭编排执行。
- 各节点消耗的 Token 会累加后随最终回复一起记账，口径与单 Agent 一致（OpenAPI 接入的 Bot 不计入用量统计）。

---

## 16. 个人 Skill

个人 Skill 让用户把自己的资料（SOP、话术、检查清单等）打包上传，绑定到自己的 Bot 后作为知识库资料被检索使用。

### 16.0 边界、状态机与存储

- **仅支持资料型 Skill**：zip 包（含 `SKILL.md` 与 `references/*.md`）或单个 markdown 文件；内容只进入独占知识库供检索，**不执行任何脚本、不提供自定义工具**。
- **隔离**：Skill 严格按「租户 + 上传者」归属，只有本人可上传、查看、绑定、删除；**不随市场分发**（与 Bot 市场的分发边界一致）。
- **每个 Skill 独占一个知识库**：上传时自动创建，删除 Skill 时一并回收该知识库及其文档向量。
- **状态机**：`0 待解析` → `1 可用`（全部文件入库成功）/ `2 解析失败`（没有任何文件入库成功）/ `3 已停用`。
- **限制**：单包 ≤ 10MB，解压后总量 ≤ 20MB，单文件 ≤ 2MB，文件数 ≤ 50；仅接受 `.md` / `.markdown` / `.txt`，其余文件跳过并在响应 `warnings` 里提示。
- **安全**：拒绝路径穿越条目（`../`、绝对路径）、过滤打包垃圾文件（`__MACOSX/`、`.DS_Store`）、逐个文件做提示词注入扫描（命中即整体拒绝）。
- **数据表**：`user_skill`（定义与状态）、`user_skill_file`（文件清单与对象地址）、`bot_skill`（Bot 绑定，`(tenant_id, bot_id, skill_id)` 复合主键）。

`SKILL.md` 的 frontmatter 用于声明 Skill 元信息（未声明时用包名兜底）：

```markdown
---
name: 会议纪要
description: 把会议记录整理成结构化要点
version: 1.0.0
---
```

---

### 16.1 上传 Skill

- **URL**: `/api/skill/upload`
- **Method**: `POST`
- **Content-Type**: `multipart/form-data`

上传 Skill 包，服务端解析后建立专属知识库并逐文件入库。同名 Skill 重复上传视为**覆盖更新**：复用原知识库，先清空旧文件记录再写入新快照。

**请求参数**（表单字段）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| file | file | 是 | zip 包或单个 markdown 文件，≤ 10MB |

**响应示例**:

成功：
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "skill_id": "1234567890123456789",
    "file_count": 3,
    "size_bytes": 18432,
    "warnings": ["assets/logo.png: 仅支持 md/markdown/txt 资料文件"]
  }
}
```

失败：
```json
{
  "code": 1,
  "msg": "上传Skill失败",
  "data": "Skill 包解压后不能超过 20 MB"
}
```

`warnings` 只在部分文件被跳过时出现，不影响整体成功；全部文件都被跳过时状态置为「解析失败」。

---

### 16.2 我的 Skill 列表

- **URL**: `/api/skill/list`
- **Method**: `GET`

返回当前用户在当前租户下的全部 Skill，按更新时间倒序。

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "skills": [
      {
        "skill_id": "1234567890123456789",
        "owner_id": "7",
        "name": "会议纪要",
        "description": "把会议记录整理成结构化要点",
        "version": "1.0.0",
        "status": 1,
        "kb_id": "2234567890123456789",
        "size_bytes": 18432,
        "file_count": 3,
        "created_at": "1700000000000",
        "updated_at": "1700000000000"
      }
    ]
  }
}
```

---

### 16.3 Skill 详情与文件清单

- **URL**: `/api/skill/detail`
- **Method**: `GET`

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| skill_id | string | 是 | Skill ID |

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "skill": {
      "skill_id": "1234567890123456789",
      "owner_id": "7",
      "name": "会议纪要",
      "description": "把会议记录整理成结构化要点",
      "version": "1.0.0",
      "status": 1,
      "kb_id": "2234567890123456789",
      "size_bytes": 18432,
      "file_count": 2,
      "created_at": "1700000000000",
      "updated_at": "1700000000000"
    },
    "files": [
      {
        "file_id": "3234567890123456789",
        "skill_id": "1234567890123456789",
        "rel_path": "SKILL.md",
        "object_url": "/files/chat/ab/cd123.md",
        "size_bytes": 512
      }
    ]
  }
}
```

---

### 16.4 删除 Skill

- **URL**: `/api/skill/delete`
- **Method**: `POST`

删除 Skill 时依次：解绑所有 Bot 上绑定的该 Skill 知识库 → 删除知识库（含文档与向量）→ 清理绑定与文件记录 → 删除 Skill 本体。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| skill_id | string | 是 | Skill ID |

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": { "success": true }
}
```

`success=false` 表示 Skill 不存在或不属于当前用户。

---

### 16.5 绑定 / 解绑 Skill

- **URL**: `/api/skill/bind`
- **Method**: `POST`

把 Skill 绑定到自己的 Bot（或从 Bot 上解绑）。绑定后会通过既有 `bot_knowledge` 链路把该 Skill 的知识库挂到 Bot 上，运行时被 `search_knowledge` 检索；解绑则同时解除该知识库绑定。

**请求参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | 目标 Bot ID |
| skill_id | string | 是 | Skill ID |
| enabled | bool | 是 | `true` 绑定，`false` 解绑 |

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": { "success": true }
}
```

约束：只有 Bot 创建者可以绑定；系统 Bot 不支持绑定个人 Skill；Skill 必须属于操作者本人。重复绑定幂等返回成功。

---

### 16.6 查询 Bot 已绑定的 Skill

- **URL**: `/api/skill/bot_skills`
- **Method**: `GET`

**请求参数**（Query）:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| bot_id | string | 是 | Bot ID |

**响应示例**:
```json
{
  "code": 0,
  "msg": "操作成功",
  "data": {
    "skills": [
      {
        "skill_id": "1234567890123456789",
        "name": "会议纪要",
        "description": "把会议记录整理成结构化要点",
        "version": "1.0.0",
        "enabled": true
      }
    ]
  }
}
```

仅 Bot 创建者可读；非创建者返回空列表。

---

### 16.7 运行与限制

- Skill 的知识库随绑定并入 Bot 的检索范围，与用户自建知识库共用同一套 Self-RAG 检索策略，无需额外配置。
- Skill 属于「用户自定义配置」资产：其检索所产生的 Token 计入 Bot 用量统计，口径与其他自建 Bot 一致。
- 上传与绑定/删除都会写审计日志（`audit_log.event_type = user_skill`），记录操作者、租户与动作结果。
- 资料型边界意味着即使包里混入脚本文件也不会被执行：非 `.md/.markdown/.txt` 一律跳过。
- 多端在线：同一账号的多个 WebSocket 连接会同时收到消息、输入状态与已读回执，单个设备断开不影响其他设备（单账号最多保留 5 个并发连接，超出淘汰最早连接）。

