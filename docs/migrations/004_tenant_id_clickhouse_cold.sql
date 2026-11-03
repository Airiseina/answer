-- 004: ClickHouse 冷库消息表 tenant_id 回填（answer_cold.cold_message）
--
-- 前置：
--   1. chat_service 启动时已执行
--      ALTER TABLE answer_cold.cold_message ADD COLUMN IF NOT EXISTS tenant_id Int64 DEFAULT 0
--      （见 pkg/infra/clickhouse.go）。
--   2. 因冷库消息在归档时已带 conversation_id，可按「会话 -> 租户」映射回填。
--      与 003 一致，需要在本库内先有会话归属快照。

-- 1) 会话 -> 租户 快照（从 PostgreSQL 导出后导入，字段：conversation_id, tenant_id）
CREATE TABLE IF NOT EXISTS answer_cold.conversation_tenant_map
(
    conversation_id Int64,
    tenant_id       Int64
) ENGINE = MergeTree
      ORDER BY conversation_id;

-- 导入示例（Postgres 侧导出 CSV 后）：
--   clickhouse-client --query "INSERT INTO answer_cold.conversation_tenant_map FORMAT CSV" < conv_tenant.csv

-- 2) 回填（ClickHouse 的 UPDATE 为异步变更，执行后需等待 mutations 完成）
ALTER TABLE answer_cold.cold_message
    UPDATE tenant_id = (
        SELECT m.tenant_id
        FROM answer_cold.conversation_tenant_map AS m
        WHERE m.conversation_id = cold_message.conversation_id
        LIMIT 1
    )
    WHERE tenant_id = 0;

-- 3) 兜底：映射缺失的孤立消息归平台租户
ALTER TABLE answer_cold.cold_message UPDATE tenant_id = 1 WHERE tenant_id = 0;

-- 4) 核验：应为 0
SELECT count() AS unassigned FROM answer_cold.cold_message WHERE tenant_id = 0;

-- 5) 等待变更完成
-- SYSTEM WAIT FOR MUTATIONS;

-- 回滚
-- ALTER TABLE answer_cold.cold_message UPDATE tenant_id = 0 WHERE tenant_id <> 0;
