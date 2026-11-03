-- 003: 会话 / 消息冷热表 tenant_id 加列 + 回填（PostgreSQL answer_chat）
--
-- 前置：
--   1. 001 / 002 已在 MySQL 业务库执行，personal_tenant 映射可用。
--   2. 需要把「用户 -> 个人租户」映射同步进聊天库（聊天库与业务库不同实例，无法跨库 JOIN）。
--
-- 同步方式（任选其一，推荐 a）：
--   a) 从 MySQL 导出 CSV，再在 Postgres 侧导入：
--      MySQL:  SELECT user_id, tenant_id FROM personal_tenant
--              INTO OUTFILE '/tmp/personal_tenant.csv'
--              FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n';
--      PG:     \copy user_tenant_map(user_id, tenant_id) FROM '/tmp/personal_tenant.csv' WITH (FORMAT csv)
--   b) 由运维脚本直接 INSERT（数据量小，几万行以内可接受）。
--
-- 注意：本脚本是 **PostgreSQL** 方言（标识符用双引号 / 不加引号，不能用 MySQL 的反引号）。
--       PostgreSQL 9.6+ 支持 ADD COLUMN IF NOT EXISTS 与 CREATE INDEX IF NOT EXISTS，可重复执行。

BEGIN;

-- 1) 映射表
CREATE TABLE IF NOT EXISTS "user_tenant_map"
(
    "user_id"   BIGINT PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL
);

-- 2) 加列
ALTER TABLE "conversation_table"   ADD COLUMN IF NOT EXISTS "tenant_id" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "conversation_member"  ADD COLUMN IF NOT EXISTS "tenant_id" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "inbox_message"        ADD COLUMN IF NOT EXISTS "tenant_id" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "message_edit_history" ADD COLUMN IF NOT EXISTS "tenant_id" BIGINT NOT NULL DEFAULT 0;
-- message_table 为按月 Range Partitioning 的父表，ADD COLUMN 会自动下发到各分区。
ALTER TABLE "message_table"        ADD COLUMN IF NOT EXISTS "tenant_id" BIGINT NOT NULL DEFAULT 0;

-- 3) 会话归属：取会话成员中最小租户（同一群/单聊在迁移前必定同属一个租户）
UPDATE "conversation_table" c
SET "tenant_id" = COALESCE((
    SELECT MIN(m."tenant_id")
    FROM "conversation_member" cm
             JOIN "user_tenant_map" m ON m."user_id" = cm."user_id"
    WHERE cm."conversation_id" = c."id"
), 1)
WHERE c."tenant_id" = 0;

-- 4) 成员 / 收件箱 / 消息跟随会话
UPDATE "conversation_member" cm
SET "tenant_id" = c."tenant_id"
FROM "conversation_table" c
WHERE cm."conversation_id" = c."id" AND cm."tenant_id" = 0;

UPDATE "inbox_message" im
SET "tenant_id" = c."tenant_id"
FROM "conversation_table" c
WHERE im."conversation_id" = c."id" AND im."tenant_id" = 0;

UPDATE "message_table" m
SET "tenant_id" = c."tenant_id"
FROM "conversation_table" c
WHERE m."conversation_id" = c."id" AND m."tenant_id" = 0;

-- 兜底：会话已删除但消息仍存在的孤立消息，归平台租户避免误判为跨租户数据
UPDATE "message_table" SET "tenant_id" = 1 WHERE "tenant_id" = 0;

-- 5) 编辑历史跟随原消息
UPDATE "message_edit_history" h
SET "tenant_id" = m."tenant_id"
FROM "message_table" m
WHERE h."msg_id" = m."msg_id" AND h."tenant_id" = 0;
UPDATE "message_edit_history" SET "tenant_id" = 1 WHERE "tenant_id" = 0;

-- 6) 索引（索引名与 GORM 模型 tag 对齐，避免 AutoMigrate 重复创建）
CREATE INDEX IF NOT EXISTS "idx_conversation_tenant" ON "conversation_table" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_conv_member_tenant"  ON "conversation_member" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_inbox_tenant"        ON "inbox_message" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_msg_edit_tenant"     ON "message_edit_history" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_tenant_timestamp"    ON "message_table" ("tenant_id", "timestamp");

-- 7) 核验：以下都应返回 0
SELECT 'conversation_table' AS tbl, COUNT(*) AS unassigned FROM "conversation_table" WHERE "tenant_id" = 0
UNION ALL SELECT 'conversation_member', COUNT(*) FROM "conversation_member" WHERE "tenant_id" = 0
UNION ALL SELECT 'inbox_message', COUNT(*) FROM "inbox_message" WHERE "tenant_id" = 0
UNION ALL SELECT 'message_edit_history', COUNT(*) FROM "message_edit_history" WHERE "tenant_id" = 0
UNION ALL SELECT 'message_table', COUNT(*) FROM "message_table" WHERE "tenant_id" = 0;

COMMIT;

-- 回滚
-- BEGIN;
-- DROP INDEX IF EXISTS "idx_tenant_timestamp";
-- DROP INDEX IF EXISTS "idx_msg_edit_tenant";
-- DROP INDEX IF EXISTS "idx_inbox_tenant";
-- DROP INDEX IF EXISTS "idx_conv_member_tenant";
-- DROP INDEX IF EXISTS "idx_conversation_tenant";
-- ALTER TABLE "message_table"        DROP COLUMN IF EXISTS "tenant_id";
-- ALTER TABLE "message_edit_history" DROP COLUMN IF EXISTS "tenant_id";
-- ALTER TABLE "inbox_message"        DROP COLUMN IF EXISTS "tenant_id";
-- ALTER TABLE "conversation_member"  DROP COLUMN IF EXISTS "tenant_id";
-- ALTER TABLE "conversation_table"   DROP COLUMN IF EXISTS "tenant_id";
-- DROP TABLE IF EXISTS "user_tenant_map";
-- COMMIT;
