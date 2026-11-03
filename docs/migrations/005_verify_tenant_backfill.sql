-- 005: 多租户回填总核验（MySQL 业务库）
--
-- 本脚本不修改数据，只做断言。任何一条返回非 0 即表示迁移未完成，需中止发布。

-- 断言 1：不存在无租户用户
SELECT 'users_without_tenant' AS `check_name`, COUNT(*) AS `violations`
FROM `users`
WHERE `tenant_id` = 0 AND `deleted_at` IS NULL;

-- 断言 2：每个用户在其个人租户内恰有一条 owner 成员记录
SELECT 'personal_owner_mismatch' AS `check_name`, COUNT(*) AS `violations`
FROM `users` u
         LEFT JOIN `tenant_member` m
                   ON m.`user_id` = u.`id` AND m.`role` = 2 AND m.`status` <> 2
WHERE u.`is_bot` = 0
  AND u.`deleted_at` IS NULL
  AND (m.`user_id` IS NULL OR m.`tenant_id` <> u.`tenant_id`);

-- 断言 3：业务表不存在 tenant_id = 0 的存量记录
SELECT 'bot_table_unassigned' AS `check_name`, COUNT(*) AS `violations` FROM `bot_table` WHERE `tenant_id` = 0
UNION ALL SELECT 'bot_mcp_server_unassigned', COUNT(*) FROM `bot_mcp_server` WHERE `tenant_id` = 0
UNION ALL SELECT 'knowledge_base_unassigned', COUNT(*) FROM `knowledge_base` WHERE `tenant_id` = 0
UNION ALL SELECT 'kb_document_unassigned', COUNT(*) FROM `kb_document` WHERE `tenant_id` = 0
UNION ALL SELECT 'bot_knowledge_unassigned', COUNT(*) FROM `bot_knowledge` WHERE `tenant_id` = 0
UNION ALL SELECT 'groups_unassigned', COUNT(*) FROM `groups` WHERE `tenant_id` = 0
UNION ALL SELECT 'group_members_unassigned', COUNT(*) FROM `group_members` WHERE `tenant_id` = 0
UNION ALL SELECT 'group_join_requests_unassigned', COUNT(*) FROM `group_join_requests` WHERE `tenant_id` = 0
UNION ALL SELECT 'group_notices_unassigned', COUNT(*) FROM `group_notices` WHERE `tenant_id` = 0
UNION ALL SELECT 'friend_requests_unassigned', COUNT(*) FROM `friend_requests` WHERE `tenant_id` = 0
UNION ALL SELECT 'friends_unassigned', COUNT(*) FROM `friends` WHERE `tenant_id` = 0
UNION ALL SELECT 'friend_groups_unassigned', COUNT(*) FROM `friend_groups` WHERE `tenant_id` = 0;

-- 断言 4：子表租户与父表一致（不能出现跨租户悬挂）
SELECT 'bot_mcp_tenant_mismatch' AS `check_name`, COUNT(*) AS `violations`
FROM `bot_mcp_server` m JOIN `bot_table` b ON b.`id` = m.`bot_id` WHERE m.`tenant_id` <> b.`tenant_id`
UNION ALL
SELECT 'kb_document_tenant_mismatch', COUNT(*)
FROM `kb_document` d JOIN `knowledge_base` k ON k.`id` = d.`kb_id` WHERE d.`tenant_id` <> k.`tenant_id`
UNION ALL
SELECT 'group_member_tenant_mismatch', COUNT(*)
FROM `group_members` gm JOIN `groups` g ON g.`id` = gm.`group_id` WHERE gm.`tenant_id` <> g.`tenant_id`;

-- 断言 5：系统 Bot 归平台租户
SELECT 'system_bot_not_platform' AS `check_name`, COUNT(*) AS `violations`
FROM `bot_table` WHERE `is_system` = 1 AND `tenant_id` <> 1;

-- 断言 6：平台租户存在且唯一
SELECT 'platform_tenant_missing' AS `check_name`, IF(COUNT(*) = 1, 0, 1) AS `violations`
FROM `tenant` WHERE `id` = 1;

-- 断言 7：唯一索引已切换为租户复合唯一（返回缺失的索引名，空集为通过）
SELECT t.`table_name`, t.`required_index`
FROM (SELECT 'friends' AS `table_name`, 'idx_tenant_user_friend' AS `required_index`
      UNION ALL SELECT 'friend_requests', 'idx_tenant_sender_receiver'
      UNION ALL SELECT 'friend_groups', 'idx_tenant_user_group'
      UNION ALL SELECT 'group_members', 'idx_tenant_group_user'
      UNION ALL SELECT 'group_join_requests', 'idx_tenant_group_user_join'
      UNION ALL SELECT 'groups', 'idx_tenant_group_number') t
WHERE NOT EXISTS (SELECT 1
                  FROM information_schema.statistics s
                  WHERE s.`table_schema` = DATABASE()
                    AND s.`table_name` = t.`table_name`
                    AND s.`index_name` = t.`required_index`);
