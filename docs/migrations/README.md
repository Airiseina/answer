# 数据库迁移约定

本目录固化多租户及相关扩展的迁移规范，保证增量、可回滚、可核验。

## 1. 基本原则

1. **只增不改**：新增列一律「可空 + 带默认值」，不修改或删除既有列，不改变既有列语义。
2. **两步上线**：先加列（可空），再回填（`UPDATE`），最后按需收紧约束；不在同一次发布里既加列又强制 `NOT NULL`。
3. **可核验**：每张回填过的表都要有校验 SQL，迁移完成后产出报告（见 `reports/`）。
4. **幂等**：脚本可重复执行不报错（`ADD COLUMN IF NOT EXISTS` 或先查 `information_schema`）。
5. **与 AutoMigrate 对齐**：GORM 模型是运行时真相来源；手写 DDL 只用于（a）存量表加列与回填、（b）无法由 AutoMigrate 表达的约束调整（如复合唯一键）。

## 2. 命名与目录

```
docs/migrations/
  README.md                 # 本文件
  templates/                # 可复制的脚本模板
  reports/                  # 每次迁移的核验报告
  001_tenant_mapping_mysql.sql          # 存量用户 -> 个人租户 映射（MySQL）
  002_tenant_id_mysql.sql               # 业务表加列 + 回填 + 唯一键调整（MySQL）
  003_tenant_id_postgres_chat.sql       # 会话 / 消息表加列 + 回填（PostgreSQL）
  004_tenant_id_clickhouse_cold.sql     # 冷库消息回填（ClickHouse）
  005_verify_tenant_backfill.sql        # 总核验断言（只读）
```

- 脚本命名：`<序号>_<动作>_<对象>.sql`，例如 `001_add_tenant_id_to_bot_table.sql`。
- 报告命名：`<序号>_<对象>_report.md`，例如 `001_tenant_backfill_report.md`。

## 3. 多租户迁移流程（Phase 1）

1. 建表 `tenant` / `tenant_member` / `tenant_department`（由 `user_service` 的 AutoMigrate 负责）。
2. 为每个已有 `user` 建个人租户：`tenant(id, owner_id, is_personal=1)`，并写 `tenant_member(user, tenant, role=owner)`。
3. 逐表 `ADD COLUMN tenant_id BIGINT NOT NULL DEFAULT 0` 并建索引。
4. 逐表回填：按 `user_id`/`creator_id`/`owner_id` 指向的用户，映射到其个人租户；系统 Bot（`creator_id=0`）归入固定平台租户 `PlatformTenantID = 1`。
5. 唯一键调整：业务键从全局唯一改为 `(tenant_id, key)` 复合唯一。
6. 产出报告：每张表回填前后的行数、未归属（`tenant_id = 0`）行数。

## 4. 验收断言

回填完成后，执行 `005_verify_tenant_backfill.sql`，下列断言必须成立（结果写入报告）：

- 所有已迁移表的 `tenant_id = 0` 计数为 **0**。
- `tenant_member` 中每个用户在其个人租户内都有且仅有一条 `role = owner` 的成员记录。
- 系统 Bot 的 `tenant_id = 1`。
- 子表（`bot_mcp_server` / `kb_document` / `group_members` 等）的 `tenant_id` 与父表一致。
- 唯一索引已切换为 `(tenant_id, key)` 复合唯一。

## 4.1 表名对照（易错点）

| 业务对象 | 实际表名 | 说明 |
| --- | --- | --- |
| 用户 | `users` | GORM 默认复数化 |
| 好友 / 好友分组 | `friends` / `friend_groups` | 同上 |
| 租户 / 成员 / 部门 | `tenant` / `tenant_member` / `tenant_department` | 模型内固定 `TableName()` |
| Bot / MCP | `bot_table` / `bot_mcp_server` | 模型内固定 `TableName()` |
| 群 / 群成员 | `groups` / `group_members` | GORM 默认复数化（`group` 是 MySQL 保留字） |
| 会话 / 消息 | `conversation_table` / `message_table` | 模型内固定 `TableName()`（PostgreSQL） |

## 5. 回滚

每个迁移脚本在同目录以注释形式给出对照的回滚语句；回滚只删除新增列/索引，不动数据。

## 6. 执行顺序

1. 备份待迁移库。
2. `001_tenant_mapping_mysql.sql`：建立「存量用户 → 个人租户」映射与成员关系。
3. `002_tenant_id_mysql.sql`：业务表加列 + 回填 + 唯一键调整为复合唯一。
4. `003_tenant_id_postgres_chat.sql`：PostgreSQL 会话/消息表加列 + 回填。
5. `004_tenant_id_clickhouse_cold.sql`：ClickHouse 冷库消息回填。
6. `005_verify_tenant_backfill.sql`：只读核验，任一条 `violations` 非 0 即中止发布。
7. 把各步输出整理为报告，落到 `reports/<序号>_<对象>_report.md`（模板见 `reports/TEMPLATE.md`）。

> 开放平台相关表（`open_app` / `open_webhook` / `open_event` / `open_webhook_delivery`）
> 为新增表，由 `open_service` 启动时的 AutoMigrate 创建，无需单独迁移脚本。
> 管理后台（`admin_user`）同理由 `admin_service` AutoMigrate 创建。
