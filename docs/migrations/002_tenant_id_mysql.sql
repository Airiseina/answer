-- 002: 业务表 tenant_id 加列 + 回填 + 唯一键租户化（MySQL 8.0 业务库）
--
-- 前置：001_tenant_mapping_mysql.sql 已执行，personal_tenant 映射表可用。
--
-- 幂等性说明（重要）：
--   MySQL 8.0 **不支持** `ADD COLUMN IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS`，
--   那是 MariaDB 的扩展语法。本脚本统一改成
--   「information_schema 判断 + PREPARE 动态 SQL」实现幂等，可重复执行；
--   表还没建（服务未启动过）时加列会被跳过而不是报错。
--
-- 说明：GORM 的 AutoMigrate 也会补列与索引；本脚本用于存量库的显式、可核验迁移，
--      索引名与模型 tag 保持一致，AutoMigrate 不会重复创建。

-- ========== 0) 加列（幂等） ==========
-- 覆盖：users / friend_requests / friends / friend_groups（user_service）
--      groups / group_members / group_join_requests / group_notices（group_service）
--      bot_table / bot_mcp_server（bot_service）
--      knowledge_base / kb_document / bot_knowledge（knowledge_service）

SET @tbl = 'users', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friend_requests', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friends', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friend_groups', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'groups', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_members', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_join_requests', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_notices', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'bot_table', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'bot_mcp_server', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'knowledge_base', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'kb_document', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'bot_knowledge', @col = 'tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = @tbl AND column_name = @col) > 0,
    'DO 1', CONCAT('ALTER TABLE `', @tbl, '` ADD COLUMN `', @col, '` BIGINT NOT NULL DEFAULT 0'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ========== 1) 回填：按归属列映射到个人租户 ==========

-- 1.0 用户自身：机器人归平台租户，普通用户归个人租户（与 001 的收尾一致，重复执行安全）
UPDATE `users` u
    JOIN `personal_tenant` pt ON pt.`user_id` = u.`id`
SET u.`tenant_id` = pt.`tenant_id`
WHERE u.`tenant_id` = 0 AND u.`is_bot` = 0;
UPDATE `users` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.1 Bot：创建者归属；系统 Bot（is_system=1 或 creator_id=0）归平台租户
UPDATE `bot_table` b
    JOIN `personal_tenant` pt ON pt.`user_id` = b.`creator_id`
SET b.`tenant_id` = pt.`tenant_id`
WHERE b.`tenant_id` = 0 AND b.`creator_id` <> 0;
UPDATE `bot_table` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.2 Bot 的 MCP 服务器：跟随所属 Bot
UPDATE `bot_mcp_server` m
    JOIN `bot_table` b ON b.`id` = m.`bot_id`
SET m.`tenant_id` = b.`tenant_id`
WHERE m.`tenant_id` = 0;
UPDATE `bot_mcp_server` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.3 知识库：创建者归属
UPDATE `knowledge_base` k
    JOIN `personal_tenant` pt ON pt.`user_id` = k.`owner_id`
SET k.`tenant_id` = pt.`tenant_id`
WHERE k.`tenant_id` = 0 AND k.`owner_id` <> 0;
UPDATE `knowledge_base` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.4 知识库文档：跟随所属知识库
UPDATE `kb_document` d
    JOIN `knowledge_base` k ON k.`id` = d.`kb_id`
SET d.`tenant_id` = k.`tenant_id`
WHERE d.`tenant_id` = 0;
UPDATE `kb_document` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.5 Bot-知识库绑定：跟随所属 Bot
UPDATE `bot_knowledge` bk
    JOIN `bot_table` b ON b.`id` = bk.`bot_id`
SET bk.`tenant_id` = b.`tenant_id`
WHERE bk.`tenant_id` = 0;
UPDATE `bot_knowledge` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.6 群组：群主归属
UPDATE `groups` g
    JOIN `personal_tenant` pt ON pt.`user_id` = g.`owner_id`
SET g.`tenant_id` = pt.`tenant_id`
WHERE g.`tenant_id` = 0 AND g.`owner_id` <> 0;
UPDATE `groups` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.7 群成员 / 入群申请 / 群公告：跟随所属群
UPDATE `group_members` gm
    JOIN `groups` g ON g.`id` = gm.`group_id`
SET gm.`tenant_id` = g.`tenant_id`
WHERE gm.`tenant_id` = 0;
UPDATE `group_members` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

UPDATE `group_join_requests` r
    JOIN `groups` g ON g.`id` = r.`group_id`
SET r.`tenant_id` = g.`tenant_id`
WHERE r.`tenant_id` = 0;
UPDATE `group_join_requests` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

UPDATE `group_notices` n
    JOIN `groups` g ON g.`id` = n.`group_id`
SET n.`tenant_id` = g.`tenant_id`
WHERE n.`tenant_id` = 0;
UPDATE `group_notices` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- 1.8 好友 / 好友请求 / 好友分组：跟随其归属用户
UPDATE `friend_requests` f
    JOIN `personal_tenant` pt ON pt.`user_id` = f.`sender`
SET f.`tenant_id` = pt.`tenant_id`
WHERE f.`tenant_id` = 0;
UPDATE `friend_requests` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

UPDATE `friends` f
    JOIN `personal_tenant` pt ON pt.`user_id` = f.`user_id`
SET f.`tenant_id` = pt.`tenant_id`
WHERE f.`tenant_id` = 0;
UPDATE `friends` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

UPDATE `friend_groups` g
    JOIN `personal_tenant` pt ON pt.`user_id` = g.`user_id`
SET g.`tenant_id` = pt.`tenant_id`
WHERE g.`tenant_id` = 0;
UPDATE `friend_groups` SET `tenant_id` = 1 WHERE `tenant_id` = 0;

-- ========== 2) 租户列索引（幂等） ==========

SET @tbl = 'users', @idx = 'idx_users_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friend_requests', @idx = 'idx_friend_requests_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friends', @idx = 'idx_friends_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friend_groups', @idx = 'idx_friend_groups_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'groups', @idx = 'idx_groups_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_members', @idx = 'idx_group_members_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_join_requests', @idx = 'idx_group_join_requests_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_notices', @idx = 'idx_group_notices_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'bot_table', @idx = 'idx_bot_table_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'bot_mcp_server', @idx = 'idx_bot_mcp_server_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'knowledge_base', @idx = 'idx_knowledge_base_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'kb_document', @idx = 'idx_kb_document_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'bot_knowledge', @idx = 'idx_bot_knowledge_tenant_id';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = @tbl) = 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE INDEX `', @idx, '` ON `', @tbl, '` (`tenant_id`)'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ========== 3) 唯一键调整为 (tenant_id, key) ==========
-- 迁移前：好友 / 好友分组 / 群成员的唯一约束是全局的，跨租户会互相占用。
-- 迁移后：唯一性下沉到租户维度。旧索引名存在时才删除，可重复执行。

SET @tbl = 'friend_requests', @idx = 'idx_sender_receiver';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friends', @idx = 'idx_user_friend';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friend_groups', @idx = 'idx_user_group';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_members', @idx = 'idx_group_user';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_join_requests', @idx = 'idx_group_user_join';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 群号唯一键：旧库可能是显式名 group_number，也可能是 GORM 默认名 idx_groups_group_number
SET @tbl = 'groups', @idx = 'group_number';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'groups', @idx = 'idx_groups_group_number';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    CONCAT('ALTER TABLE `', @tbl, '` DROP INDEX `', @idx, '`'), 'DO 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 复合唯一索引（与 GORM 模型 tag 同名，AutoMigrate 不会重复创建）
SET @tbl = 'friend_requests', @idx = 'idx_tenant_sender_receiver', @cols = '`tenant_id`, `sender`, `receiver`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friends', @idx = 'idx_tenant_user_friend', @cols = '`tenant_id`, `user_id`, `friend_id`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'friend_groups', @idx = 'idx_tenant_user_group', @cols = '`tenant_id`, `user_id`, `name`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_members', @idx = 'idx_tenant_group_user', @cols = '`tenant_id`, `group_id`, `user_id`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'group_join_requests', @idx = 'idx_tenant_group_user_join', @cols = '`tenant_id`, `group_id`, `user_id`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @tbl = 'groups', @idx = 'idx_tenant_group_number', @cols = '`tenant_id`, `group_number`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- bot_knowledge 的 (tenant_id, bot_id) 复合唯一也一并对齐
SET @tbl = 'bot_knowledge', @idx = 'idx_bot_kb', @cols = '`tenant_id`, `bot_id`';
SET @sql = IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = @tbl AND index_name = @idx) > 0,
    'DO 1', CONCAT('CREATE UNIQUE INDEX `', @idx, '` ON `', @tbl, '` (', @cols, ')'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ========== 4) 核验：以下每张表都应为 0 ==========
SELECT 'users' AS tbl, COUNT(*) AS unassigned FROM `users` WHERE `tenant_id` = 0
UNION ALL SELECT 'friend_requests', COUNT(*) FROM `friend_requests` WHERE `tenant_id` = 0
UNION ALL SELECT 'friends', COUNT(*) FROM `friends` WHERE `tenant_id` = 0
UNION ALL SELECT 'friend_groups', COUNT(*) FROM `friend_groups` WHERE `tenant_id` = 0
UNION ALL SELECT 'bot_table', COUNT(*) FROM `bot_table` WHERE `tenant_id` = 0
UNION ALL SELECT 'bot_mcp_server', COUNT(*) FROM `bot_mcp_server` WHERE `tenant_id` = 0
UNION ALL SELECT 'knowledge_base', COUNT(*) FROM `knowledge_base` WHERE `tenant_id` = 0
UNION ALL SELECT 'kb_document', COUNT(*) FROM `kb_document` WHERE `tenant_id` = 0
UNION ALL SELECT 'bot_knowledge', COUNT(*) FROM `bot_knowledge` WHERE `tenant_id` = 0
UNION ALL SELECT 'groups', COUNT(*) FROM `groups` WHERE `tenant_id` = 0
UNION ALL SELECT 'group_members', COUNT(*) FROM `group_members` WHERE `tenant_id` = 0
UNION ALL SELECT 'group_join_requests', COUNT(*) FROM `group_join_requests` WHERE `tenant_id` = 0
UNION ALL SELECT 'group_notices', COUNT(*) FROM `group_notices` WHERE `tenant_id` = 0;

-- 系统 Bot 必须落在平台租户
SELECT COUNT(*) AS system_bot_not_in_platform FROM `bot_table` WHERE `is_system` = 1 AND `tenant_id` <> 1;

-- 复合唯一索引必须存在（期望 7 行）
SELECT table_name, index_name FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND index_name IN ('idx_tenant_sender_receiver', 'idx_tenant_user_friend', 'idx_tenant_user_group',
                     'idx_tenant_group_user', 'idx_tenant_group_user_join', 'idx_tenant_group_number', 'idx_bot_kb')
GROUP BY table_name, index_name;

-- ========== 5) 回滚 ==========
-- DROP INDEX `idx_tenant_sender_receiver` ON `friend_requests`;
-- DROP INDEX `idx_tenant_user_friend` ON `friends`;
-- DROP INDEX `idx_tenant_user_group` ON `friend_groups`;
-- DROP INDEX `idx_tenant_group_user` ON `group_members`;
-- DROP INDEX `idx_tenant_group_user_join` ON `group_join_requests`;
-- DROP INDEX `idx_tenant_group_number` ON `groups`;
-- DROP INDEX `idx_bot_kb` ON `bot_knowledge`;
-- 随后逐表 ALTER TABLE <t> DROP COLUMN `tenant_id`; （数据不动）
