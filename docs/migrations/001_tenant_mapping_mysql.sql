-- 001: 存量用户 -> 个人租户 映射建立（MySQL 业务库）
--
-- 前置：
--   1. user_service 至少启动过一次，AutoMigrate 已创建 tenant / tenant_member / tenant_department。
--   2. 本脚本幂等，可重复执行。
--
-- 说明：个人租户 ID 使用保留号段 2000000000000 + user_id。
--   现有 snowflake 节点 epoch=1700000000000（2023-11-06），当前生成 ID 约 4e17；
--   即使「刚过 epoch 立刻发号」，最小 ID 也只有几千量级。
--   2e12 号段对应 epoch + 约 477 秒，早已越过，现有与未来 snowflake 都不会落入该区间，
--   因此号段不会与运行时生成的租户 ID 碰撞。

-- 0) users.tenant_id 列与索引（幂等）
-- 注意：MySQL 8.0 不支持 `ADD COLUMN IF NOT EXISTS`，下面用 information_schema + 动态 SQL 保证幂等。
SET @tbl = 'users', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'users', @idx = 'idx_users_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', 'CREATE INDEX `idx_users_tenant_id` ON `users` (`tenant_id`)');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2) 迁移期映射表：用户 -> 个人租户
CREATE TABLE IF NOT EXISTS `personal_tenant` (
    `user_id`   BIGINT NOT NULL,
    `tenant_id` BIGINT NOT NULL,
    PRIMARY KEY (`user_id`),
    KEY `idx_personal_tenant_tenant` (`tenant_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- 2) 平台租户兜底（正常运行时会由 user_service 的 EnsurePlatformTenant 创建）
INSERT INTO `tenant` (`id`, `created_at`, `updated_at`, `deleted_at`, `name`, `code`, `logo_url`,
                      `owner_id`, `is_personal`, `status`, `plan`, `settings`)
SELECT 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, '平台', 'platform', '', 0, 0, 0, 'system', NULL
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `tenant` WHERE `id` = 1);

-- 3) 为每个存量用户创建个人租户（跳过机器人账号）
INSERT INTO `tenant` (`id`, `created_at`, `updated_at`, `deleted_at`, `name`, `code`, `logo_url`,
                      `owner_id`, `is_personal`, `status`, `plan`, `settings`)
SELECT 2000000000000 + u.`id`,
       UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL,
       COALESCE(NULLIF(u.`name`, ''), u.`account`),
       CONCAT('p_', u.`id`), '', u.`id`, 1, 0, 'free', NULL
FROM `users` u
         LEFT JOIN `tenant` t ON t.`id` = 2000000000000 + u.`id`
WHERE u.`is_bot` = 0
  AND u.`deleted_at` IS NULL
  AND t.`id` IS NULL;

-- 4) 个人租户的 owner 成员记录（role=2 owner，status=0 正常）
INSERT INTO `tenant_member` (`tenant_id`, `user_id`, `role`, `dept_id`, `status`, `joined_at`)
SELECT 2000000000000 + u.`id`, u.`id`, 2, 0, 0, UNIX_TIMESTAMP()
FROM `users` u
         LEFT JOIN `tenant_member` m
                   ON m.`tenant_id` = 2000000000000 + u.`id` AND m.`user_id` = u.`id`
WHERE u.`is_bot` = 0
  AND u.`deleted_at` IS NULL
  AND m.`user_id` IS NULL;

-- 5) 写入映射表
INSERT INTO `personal_tenant` (`user_id`, `tenant_id`)
SELECT u.`id`, 2000000000000 + u.`id`
FROM `users` u
WHERE u.`is_bot` = 0
  AND u.`deleted_at` IS NULL
ON DUPLICATE KEY UPDATE `tenant_id` = VALUES(`tenant_id`);

-- 6) 回填 user 表自身的默认租户：机器人归平台租户，普通用户归个人租户
UPDATE `users` SET `tenant_id` = 1 WHERE `is_bot` = 1 AND `tenant_id` = 0;
UPDATE `users` SET `tenant_id` = 2000000000000 + `id` WHERE `is_bot` = 0 AND `tenant_id` = 0;

-- 核验：应为 0
SELECT COUNT(*) AS `users_without_tenant` FROM `users` WHERE `tenant_id` = 0 AND `deleted_at` IS NULL;

-- 回滚（仅删除新增数据，不动既有数据）
-- DELETE FROM `tenant_member` WHERE `tenant_id` >= 2000000000000;
-- DELETE FROM `tenant` WHERE `id` >= 2000000000000;
-- DROP TABLE IF EXISTS `personal_tenant`;
