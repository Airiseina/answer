-- 模板：新增列 + 回填 + 核验（以 tenant_id 为例）。
-- 约定：先加可空列 → 回填 → 建索引 → 核验；可重复执行。
-- 事务语义：MySQL DDL 隐式提交，故脚本按「每一步独立可重跑」设计。

-- 1) 加列（可空 + 默认值，幂等）
ALTER TABLE `bot_table`
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;

-- 2) 回填：按归属列映射到各用户的个人租户
--    personal_tenant 为「用户 -> 个人租户」的映射（迁移开始前由 001 脚本构建）。
UPDATE `bot_table` b
JOIN personal_tenant pt ON pt.user_id = b.creator_id
SET b.tenant_id = pt.tenant_id
WHERE b.tenant_id = 0 AND b.creator_id <> 0;

-- 系统 Bot（creator_id = 0）归入平台租户
UPDATE `bot_table`
SET tenant_id = 1
WHERE tenant_id = 0 AND creator_id = 0;

-- 3) 索引
CREATE INDEX IF NOT EXISTS idx_bot_table_tenant_id ON `bot_table` (tenant_id);

-- 4) 核验：应为 0，否则中止发布
SELECT COUNT(*) AS unassigned FROM `bot_table` WHERE tenant_id = 0;

-- 回滚（仅删新增列/索引，不动数据）
-- DROP INDEX idx_bot_table_tenant_id ON `bot_table`;
-- ALTER TABLE `bot_table` DROP COLUMN tenant_id;
