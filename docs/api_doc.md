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
