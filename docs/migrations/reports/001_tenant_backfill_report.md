# 迁移报告：存量用户多租户化（Phase 1）

- 执行人：待填写
- 执行时间（Asia/Shanghai）：待填写
- 环境：dev / staging / prod（待填写）
- 关联脚本：
  - `docs/migrations/001_tenant_mapping_mysql.sql`
  - `docs/migrations/002_tenant_id_mysql.sql`
  - `docs/migrations/003_tenant_id_postgres_chat.sql`
  - `docs/migrations/004_tenant_id_clickhouse_cold.sql`
  - `docs/migrations/005_verify_tenant_backfill.sql`
- 关联阶段：Phase 1
- 状态：脚本已就绪，待在有存量数据的环境执行并回填实测数值

## 1. 变更摘要

| 表 / 存储 | 动作 | 新增列 | 索引 | 是否回填 |
| --- | --- | --- | --- | --- |
| tenant | 建表（AutoMigrate） | - | code 唯一 | 否 |
| tenant_member | 建表（AutoMigrate） | - | 复合主键 (tenant_id, user_id) | 是（001 写入 owner 记录） |
| tenant_department | 建表（AutoMigrate） | - | tenant_id | 否 |
| users | 加列 | tenant_id | idx | 是（001） |
| bot_table | 加列 | tenant_id | idx | 是（002） |
| bot_mcp_server | 加列 | tenant_id | idx | 是（002） |
| knowledge_base | 加列 | tenant_id | idx | 是（002） |
| kb_document | 加列 | tenant_id | idx | 是（002） |
| bot_knowledge | 加列 | tenant_id | idx | 是（002） |
| groups | 加列 + 唯一键调整 | tenant_id | idx + (tenant_id, group_number) 唯一 | 是（002） |
| group_members | 加列 + 唯一键调整 | tenant_id | idx + (tenant_id, group_id, user_id) 唯一 | 是（002） |
| group_join_requests | 加列 + 唯一键调整 | tenant_id | idx + 复合唯一 | 是（002） |
| group_notices | 加列 | tenant_id | idx | 是（002） |
| friend_requests | 加列 + 唯一键调整 | tenant_id | (tenant_id, sender, receiver) 唯一 | 是（002） |
| friends | 加列 + 唯一键调整 | tenant_id | (tenant_id, user_id, friend_id) 唯一 | 是（002） |
| friend_groups | 加列 + 唯一键调整 | tenant_id | (tenant_id, user_id, name) 唯一 | 是（002） |
| conversation_table | 加列 | tenant_id | idx | 是（003，Postgres） |
| conversation_member | 加列 | tenant_id | idx | 是（003） |
| inbox_message | 加列 | tenant_id | idx | 是（003） |
| message_edit_history | 加列 | tenant_id | idx | 是（003） |
| message_table（含各月分区） | 加列 | tenant_id | (tenant_id, timestamp) | 是（003） |
| answer_cold.cold_message | 加列 + 回填 | tenant_id | - | 是（004，ClickHouse） |

## 2. 回填前后行数

| 表 | 迁移前总行数 | 迁移后总行数 | 未归属（tenant_id=0）|
| --- | --- | --- | --- |
| users | 待填写 | 待填写 | 待填写 |
| bot_table | 待填写 | 待填写 | 待填写 |
| knowledge_base | 待填写 | 待填写 | 待填写 |
| groups | 待填写 | 待填写 | 待填写 |
| conversation_table | 待填写 | 待填写 | 待填写 |
| message_table | 待填写 | 待填写 | 待填写 |

取值 SQL：

- 总行数：`SELECT COUNT(*) FROM bot_table;`
- 未归属：`SELECT COUNT(*) FROM bot_table WHERE tenant_id = 0;`

## 3. 断言结果

执行 `docs/migrations/005_verify_tenant_backfill.sql`，全部 `violations` 必须为 0。

- [ ] 不存在无租户用户（断言 1）
- [ ] 每个用户在其个人租户内恰有一条 owner 成员记录（断言 2）
- [ ] 业务表无 tenant_id = 0 的存量记录（断言 3）
- [ ] 子表租户与父表一致（断言 4）
- [ ] 系统 Bot 归平台租户 1（断言 5）
- [ ] 平台租户存在且唯一（断言 6）
- [ ] 唯一索引已切换为租户复合唯一（断言 7）
- [ ] Postgres 聊天库 5 张表 tenant_id 无 0 值（脚本 003 第 7 节）
- [ ] ClickHouse 冷表 tenant_id 无 0 值（脚本 004 第 4 节）

实测结果（执行时填写）：

| 断言 | violations | 结论 |
| --- | --- | --- |
| 1 | 待填写 | 通过 / 不通过 |
| 2 | 待填写 | 通过 / 不通过 |
| 3 | 待填写 | 通过 / 不通过 |
| 4 | 待填写 | 通过 / 不通过 |
| 5 | 待填写 | 通过 / 不通过 |
| 6 | 待填写 | 通过 / 不通过 |
| 7 | 待填写 | 通过 / 不通过 |

## 4. 抽样核验

| 表 | 抽样主键 | tenant_id | 期望租户 | 结论 |
| --- | --- | --- | --- | --- |
| bot_table | 待填写 | 待填写 | 创建者个人租户 | 待填写 |
| groups | 待填写 | 待填写 | 群主个人租户 | 待填写 |
| conversation_table | 待填写 | 待填写 | 会话成员所属租户 | 待填写 |
| message_table | 待填写 | 待填写 | 同会话租户 | 待填写 |

## 5. 回滚预案

- 回滚脚本：各迁移脚本尾部的注释块（仅删列 / 删索引，不动数据）。
- 影响评估：回滚后租户过滤失去数据支撑，必须同步回滚应用版本，否则查询将全部退化为跨租户可见。
- 建议：先在 staging 全量演练一次回填 + 回滚，再上生产。

## 6. 备注

- 个人租户 ID 使用保留号段 `2000000000000 + user_id`，与 snowflake 号段不重叠（论证见 `001_tenant_mapping_mysql.sql` 头部注释）。
- 聊天库与业务库分属不同实例，`user_tenant_map` 需通过导出 / 导入同步（见 `003` 头部说明）。
- 存量 Bot 若 `creator_id` 指向已注销用户，会落入平台租户（`tenant_id = 1`），属预期行为。
- 待跟进项：ClickHouse 的 `ALTER ... UPDATE` 为异步 mutation，回填后需 `SYSTEM WAIT FOR MUTATIONS` 再核验。
