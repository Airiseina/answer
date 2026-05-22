package dal

import (
	"chat_service/internal/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// convPairKey 生成单聊会话的用户对缓存 Key
// 无论 userA、userB 传入顺序如何，始终按较小值在前，保证同一对用户映射到同一个 Key
// 格式: conv:pair:{minUID}:{maxUID}
func hasDuplicateMembers(members []int64) bool {
	seen := make(map[int64]struct{}, len(members))
	for _, m := range members {
		if _, ok := seen[m]; ok {
			return true
		}
		seen[m] = struct{}{}
	}
	return false
}

func (dao *conversationDao) convPairKey(userA, userB int64) string {
	minUID, maxUID := userA, userB
	if userA > userB {
		minUID, maxUID = userB, userA
	}
	return fmt.Sprintf("conv:pair:%d:%d", minUID, maxUID)
}

// convMembersKey 生成会话成员列表的缓存 Key
// 格式: conv:members:{conversationID}
func (dao *conversationDao) convMembersKey(conversationID int64) string {
	return fmt.Sprintf("conv:members:%d", conversationID)
}

// CreateConversation 创建会话并在同一事务中插入所有成员记录
// 事务保证会话记录和成员记录的原子性：要么全部成功，要么全部回滚
// 注意：conv.ID 应由调用方（Service 层）预生成，DAL 层不负责 ID 生成
// 创建成功后会更新 Redis 缓存：
//   - 缓存会话成员列表（所有类型）
//   - 若为单聊会话，额外缓存用户对 → conversationID 的映射
func (dao *conversationDao) CreateConversation(ctx context.Context, conv *model.Conversation, memberIDs []int64) (int64, error) {
	err := dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 插入会话记录
		if err := tx.Create(conv).Error; err != nil {
			return fmt.Errorf("创建会话失败: %w", err)
		}
		// 批量插入成员记录，第一个成员为创建者
		now := time.Now().UnixMilli()
		for i, uid := range memberIDs {
			role := model.MemberRoleNormal
			if i == 0 {
				role = model.MemberRoleCreator
			}
			member := model.ConversationMember{
				ConversationID: conv.ID,
				UserID:         uid,
				Role:           role,
				JoinedAt:       now,
			}
			if err := tx.Create(&member).Error; err != nil {
				return fmt.Errorf("插入会话成员失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// 事务提交成功后，更新 Redis 缓存
	dao.cacheMembers(ctx, conv.ID, memberIDs)

	// 单聊会话额外缓存用户对映射，加速后续 GetOrCreatePrivateConversation 查询
	if conv.Type == model.ConvTypePrivate && len(memberIDs) == 2 {
		minUID, maxUID := memberIDs[0], memberIDs[1]
		if memberIDs[0] > memberIDs[1] {
			minUID, maxUID = memberIDs[1], memberIDs[0]
		}
		if err := dao.rdb.Set(ctx, dao.convPairKey(minUID, maxUID), conv.ID, 0).Err(); err != nil {
			return 0, err
		}
	}
	return conv.ID, nil
}

// GetOrCreatePrivateConversation 获取或隐式创建单聊会话
// 这是统一会话模型的核心方法，实现了单聊会话的"隐式创建"语义：
// 用户首次给对方发消息时自动创建会话，无需显式调用创建接口
//
// 查找流程（三级查找，从快到慢）：
//  1. Redis 缓存：查找 conv:pair:{min}:{max} → conversationID 映射
//  2. 数据库查询：通过子查询查找两人共同的 type=1 会话
//  3. 分布式锁 + Double-Check：获取锁后再次查库，确认不存在才创建
//
// 并发安全：使用 Redis SETNX 实现分布式锁，锁超时 5 秒
// 获取锁失败时，渐进退避重试读取缓存（3 次，50ms/100ms/150ms）
//
// 参数 idGen: 会话ID生成函数，由 Service 层提供，DAL 层不持有 ID 生成策略
func (dao *conversationDao) GetOrCreatePrivateConversation(ctx context.Context, userA, userB int64, idGen func() int64) (int64, error) {
	if userA == userB {
		return 0, fmt.Errorf("单聊会话的两个用户不能相同")
	}
	minUID, maxUID := userA, userB
	if userA > userB {
		minUID, maxUID = userB, userA
	}
	pairKey := dao.convPairKey(minUID, maxUID)

	// ===== 第一级：查 Redis 缓存 =====
	val, err := dao.rdb.Get(ctx, pairKey).Result()
	if err == nil {
		convID, parseErr := strconv.ParseInt(val, 10, 64)
		if parseErr == nil && convID > 0 {
			return convID, nil
		}
	}
	// 非"Key不存在"的错误（如 Redis 连接故障），直接返回
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("查询会话缓存失败: %w", err)
	}

	// ===== 第二级：查数据库 =====
	existingConvID, dbErr := dao.findPrivateConversation(ctx, minUID, maxUID)
	if dbErr != nil {
		return 0, fmt.Errorf("查询数据库会话失败: %w", dbErr)
	}
	if existingConvID > 0 {
		if cacheErr := dao.rdb.Set(ctx, pairKey, existingConvID, 0).Err(); cacheErr != nil {
			return 0, fmt.Errorf("回填会话缓存失败: %w", cacheErr)
		}
		dao.refreshMembersCache(ctx, existingConvID)
		return existingConvID, nil
	}

	// ===== 第三级：分布式锁保护下的创建 =====
	lockKey := fmt.Sprintf("lock:conv:create:%d:%d", minUID, maxUID)
	lockValue := fmt.Sprintf("%d:%d", time.Now().UnixNano(), minUID)
	locked, lockErr := dao.rdb.SetNX(ctx, lockKey, lockValue, 5*time.Second).Result()
	if lockErr != nil {
		return 0, fmt.Errorf("获取创建锁失败: %w", lockErr)
	}
	if !locked {
		// 未获锁：另一个请求正在创建，渐进退避重试读取缓存
		for i := 1; i <= 3; i++ {
			time.Sleep(time.Duration(50*i) * time.Millisecond)
			val, err := dao.rdb.Get(ctx, pairKey).Result()
			if err == nil {
				convID, parseErr := strconv.ParseInt(val, 10, 64)
				if parseErr == nil && convID > 0 {
					return convID, nil
				}
			}
		}
		return 0, fmt.Errorf("创建会话冲突，请重试")
	}
	// 使用 Lua 脚本安全释放锁：只有值匹配时才删除，避免误删他人的锁
	// 场景：本请求持锁超过 5s 后锁自动过期，另一个请求获取了新锁，此时本请求的 defer 不能删除新锁
	defer func() {
		luaScript := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
		dao.rdb.Eval(ctx, luaScript, []string{lockKey}, lockValue)
	}()
	// Double-Check：加锁后再次查库，防止锁竞争期间已被另一个请求创建
	existingConvID, dbErr = dao.findPrivateConversation(ctx, minUID, maxUID)
	if dbErr != nil {
		return 0, fmt.Errorf("双重检查查询会话失败: %w", dbErr)
	}
	if existingConvID > 0 {
		dao.rdb.Set(ctx, pairKey, existingConvID, 0)
		dao.refreshMembersCache(ctx, existingConvID)
		return existingConvID, nil
	}

	// 确认不存在，创建新会话
	// ID 由调用方通过 idGen 回调生成，DAL 层不持有 ID 生成策略
	conv := &model.Conversation{
		ID:        idGen(),
		Type:      model.ConvTypePrivate,
		Name:      "", // 单聊会话名称为空，由前端根据成员信息展示
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	return dao.CreateConversation(ctx, conv, []int64{minUID, maxUID})
}

// findPrivateConversation 在数据库中查找两人共同的单聊会话
// 通过子查询实现：先找两人共同参与的会话ID，再筛选 type=1 的单聊会话
// 返回值: 会话ID（0 表示不存在），数据库错误
func (dao *conversationDao) findPrivateConversation(ctx context.Context, minUID, maxUID int64) (int64, error) {
	var convID int64
	subQuery := dao.db.Table("conversation_member").
		Select("conversation_id").
		Where("user_id = ? AND conversation_id IN (SELECT conversation_id FROM conversation_member WHERE user_id = ?)", minUID, maxUID)
	err := dao.db.WithContext(ctx).
		Table("conversation_table").
		Where("id IN (?) AND type = ?", subQuery, model.ConvTypePrivate).
		Limit(1).
		Pluck("id", &convID).Error
	if err != nil {
		return 0, err
	}
	return convID, nil
}

// GetConversationMembers 查询会话的所有成员ID
// 采用缓存优先策略：Redis 命中则直接返回，未命中则回源数据库并回填缓存
// 缓存格式: conv:members:{conversationID} → JSON 编码的 []int64
// TTL 策略：单聊24小时过期，群聊1小时过期
func (dao *conversationDao) GetConversationMembers(ctx context.Context, conversationID int64) ([]int64, error) {
	membersKey := dao.convMembersKey(conversationID)
	// 尝试从 Redis 缓存读取
	val, err := dao.rdb.Get(ctx, membersKey).Result()
	if err == nil {
		var members []int64
		if jsonErr := json.Unmarshal([]byte(val), &members); jsonErr == nil {
			if !hasDuplicateMembers(members) {
				return members, nil
			}
			dao.refreshMembersCache(ctx, conversationID)
		}
	}
	// 缓存未命中，回源数据库
	var members []int64
	err = dao.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ?", conversationID).
		Pluck("user_id", &members).Error
	if err != nil {
		return nil, fmt.Errorf("查询会话成员失败: %w", err)
	}
	// 回填缓存
	dao.cacheMembers(ctx, conversationID, members)
	return members, nil
}

// GetUserConversations 查询用户参与的所有会话
// 先通过 conversation_member 表查找用户所属的会话ID列表，
// 再批量查询会话详情，按 updated_at 降序排列（最近活跃的会话排在前面）
func (dao *conversationDao) GetUserConversations(ctx context.Context, userID int64) ([]model.Conversation, error) {
	var convIDs []int64
	err := dao.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("user_id = ?", userID).
		Pluck("conversation_id", &convIDs).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户会话列表失败: %w", err)
	}
	if len(convIDs) == 0 {
		return nil, nil
	}

	var conversations []model.Conversation
	err = dao.db.WithContext(ctx).Where("id IN ?", convIDs).Order("updated_at DESC").Find(&conversations).Error
	if err != nil {
		return nil, fmt.Errorf("查询会话详情失败: %w", err)
	}
	return conversations, nil
}

// GetConversationInfo 查询单个会话的详细信息
func (dao *conversationDao) GetConversationInfo(ctx context.Context, conversationID int64) (*model.Conversation, error) {
	var conv model.Conversation
	err := dao.db.WithContext(ctx).Where("id = ?", conversationID).First(&conv).Error
	if err != nil {
		return nil, fmt.Errorf("查询会话信息失败: %w", err)
	}
	return &conv, nil
}

// cacheMembers 将会话成员列表写入 Redis 缓存
// 缓存 Key: conv:members:{conversationID}
// 缓存 Value: JSON 编码的 []int64
// TTL 策略：
//   - 单聊（2人）：24 小时过期，虽然成员固定不变，但设置 TTL 可防止异常数据永久驻留
//   - 群聊（>2人）：1 小时过期，因为成员可能变动（加人/踢人）
//
// 注意：缓存写入为尽力而为，失败不影响主流程
func (dao *conversationDao) cacheMembers(ctx context.Context, conversationID int64, members []int64) {
	data, err := json.Marshal(members)
	if err != nil {
		return
	}
	ttl := time.Hour
	if len(members) == 2 {
		ttl = 24 * time.Hour
	}
	_ = dao.rdb.Set(ctx, dao.convMembersKey(conversationID), string(data), ttl).Err()
}

func (dao *conversationDao) refreshMembersCache(ctx context.Context, conversationID int64) {
	var members []int64
	err := dao.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ?", conversationID).
		Pluck("user_id", &members).Error
	if err != nil {
		return
	}
	dao.cacheMembers(ctx, conversationID, members)
}

// invalidateMembersCache 删除会话成员缓存
// 在成员变更（添加/移除）时调用，确保下次查询时从数据库重建缓存
// 这是 Cache-Aside 模式的标准做法：写操作后删缓存，而非更新缓存
// 原因：并发场景下"更新缓存"可能导致旧数据覆盖新数据，"删除缓存"更安全
func (dao *conversationDao) invalidateMembersCache(ctx context.Context, conversationID int64) {
	_ = dao.rdb.Del(ctx, dao.convMembersKey(conversationID)).Err()
}

// AddMembers 向已有会话中批量添加成员
// 用于群聊场景：group_service 邀请成员入群后，通过 RPC 同步会话成员数据
//
// 为什么需要此方法：
//   - 统一会话模型下，群聊会话的成员必须同时存在于 conversation_member 表
//   - group_service 负责群组治理（权限校验、审批流程），chat_service 负责消息路由
//   - 两个服务的数据必须保持一致，否则消息推送会遗漏新成员
//
// 缓存策略：添加成员后删除缓存（而非更新），避免并发写导致数据不一致
func (dao *conversationDao) AddMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	if len(memberIDs) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	members := make([]model.ConversationMember, 0, len(memberIDs))
	for _, uid := range memberIDs {
		members = append(members, model.ConversationMember{
			ConversationID: conversationID,
			UserID:         uid,
			Role:           model.MemberRoleNormal,
			JoinedAt:       now,
		})
	}
	if err := dao.db.WithContext(ctx).Create(&members).Error; err != nil {
		return fmt.Errorf("添加会话成员失败: %w", err)
	}
	dao.invalidateMembersCache(ctx, conversationID)
	return nil
}

// RemoveMembers 从已有会话中批量移除成员
// 用于群聊场景：group_service 踢出成员后，通过 RPC 同步会话成员数据
//
// 为什么需要此方法：
//   - 被踢出的成员不应再收到该会话的消息推送
//   - 如果不同步移除 conversation_member 记录，被踢用户仍能通过 GetConversationMembers 被查到
//   - 这会导致消息推送给已退群的用户，造成数据不一致
//
// 缓存策略：移除成员后删除缓存，确保下次查询时从数据库重建最新成员列表
func (dao *conversationDao) RemoveMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	if len(memberIDs) == 0 {
		return nil
	}
	if err := dao.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id IN ?", conversationID, memberIDs).
		Delete(&model.ConversationMember{}).Error; err != nil {
		return fmt.Errorf("移除会话成员失败: %w", err)
	}
	dao.invalidateMembersCache(ctx, conversationID)
	return nil
}

// BatchGetConversationMembers 批量查询多个会话的成员列表
// 采用缓存优先策略：先通过 Redis Pipeline 批量读取缓存，缓存未命中的再回源数据库
// 相比逐个调用 GetConversationMembers，减少了 N-1 次 Redis 往返和可能的数据库查询
//
// 参数 conversationIDs: 会话ID列表
// 返回值: map[conversationID][]memberID，所有会话都会有条目（即使成员为空切片）
func (dao *conversationDao) BatchGetConversationMembers(ctx context.Context, conversationIDs []int64) (map[int64][]int64, error) {
	if len(conversationIDs) == 0 {
		return make(map[int64][]int64), nil
	}

	result := make(map[int64][]int64, len(conversationIDs))

	// 第一级：Redis Pipeline 批量读取缓存
	keys := make([]string, len(conversationIDs))
	for i, convID := range conversationIDs {
		keys[i] = dao.convMembersKey(convID)
	}
	vals, err := dao.rdb.MGet(ctx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		// Redis 故障时降级为全量数据库查询
		return dao.batchGetMembersFromDB(ctx, conversationIDs, result)
	}

	var missedIDs []int64
	for i, val := range vals {
		convID := conversationIDs[i]
		if val == nil {
			missedIDs = append(missedIDs, convID)
			continue
		}
		var members []int64
		if jsonErr := json.Unmarshal([]byte(val.(string)), &members); jsonErr == nil {
			result[convID] = members
		} else {
			missedIDs = append(missedIDs, convID)
		}
	}

	if len(missedIDs) == 0 {
		return result, nil
	}

	// 第二级：缓存未命中的回源数据库
	return dao.batchGetMembersFromDB(ctx, missedIDs, result)
}

// batchGetMembersFromDB 从数据库批量查询会话成员，并回填缓存
func (dao *conversationDao) batchGetMembersFromDB(ctx context.Context, conversationIDs []int64, result map[int64][]int64) (map[int64][]int64, error) {
	var rows []model.ConversationMember
	err := dao.db.WithContext(ctx).
		Where("conversation_id IN ?", conversationIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("批量查询会话成员失败: %w", err)
	}

	grouped := make(map[int64][]int64)
	for _, row := range rows {
		grouped[row.ConversationID] = append(grouped[row.ConversationID], row.UserID)
	}

	for _, convID := range conversationIDs {
		members := grouped[convID]
		if members == nil {
			members = []int64{}
		}
		result[convID] = members
		dao.cacheMembers(ctx, convID, members)
	}

	return result, nil
}

func (dao *conversationDao) DeleteConversation(ctx context.Context, conversationID int64) error {
	err := dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&model.ConversationMember{}).Error; err != nil {
			return fmt.Errorf("删除会话成员失败: %w", err)
		}
		if err := tx.Where("id = ?", conversationID).Delete(&model.Conversation{}).Error; err != nil {
			return fmt.Errorf("删除会话记录失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	dao.invalidateMembersCache(ctx, conversationID)
	return nil
}

// convMaxSeqKey 生成会话最大消息序号的 Redis Key
// 格式: conv:max_seq:{conversationID}
// 用于写扩散模型：每条消息落库后递增此 Key，作为该会话内消息的单调递增序号
func (dao *conversationDao) convMaxSeqKey(conversationID int64) string {
	return fmt.Sprintf("conv:max_seq:%d", conversationID)
}

// memberReadSeqKey 生成会话成员已读序号的 Redis 缓存 Key
// 格式: conv:member:read_seq:{conversationID}:{userID}
// 缓存用户在该会话中的已读位置，加速未读数计算
func (dao *conversationDao) memberReadSeqKey(conversationID int64, userID int64) string {
	return fmt.Sprintf("conv:member:read_seq:%d:%d", conversationID, userID)
}

// IncrConvMaxSeq 原子递增会话的最大消息序号
// 使用 Redis INCR 命令实现，天然保证原子性和单调递增
// 首次调用时 Key 不存在，INCR 会自动从 0 递增到 1
// 返回值: 递增后的 seq 值（即新消息的序号）
func (dao *conversationDao) IncrConvMaxSeq(ctx context.Context, conversationID int64) (int64, error) {
	seq, err := dao.rdb.Incr(ctx, dao.convMaxSeqKey(conversationID)).Result()
	if err != nil {
		return 0, fmt.Errorf("递增会话最大序号失败: %w", err)
	}
	return seq, nil
}

// GetConvMaxSeq 获取会话的当前最大消息序号
// Key 不存在时返回 0，表示该会话尚无消息
func (dao *conversationDao) GetConvMaxSeq(ctx context.Context, conversationID int64) (int64, error) {
	val, err := dao.rdb.Get(ctx, dao.convMaxSeqKey(conversationID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("查询会话最大序号失败: %w", err)
	}
	seq, parseErr := strconv.ParseInt(val, 10, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("解析会话最大序号失败: %w", parseErr)
	}
	return seq, nil
}

// BatchGetConvMaxSeq 批量获取多个会话的最大消息序号
// 使用 Redis MGet 一次性读取所有 Key，减少网络往返
// Key 不存在时对应值为 0
func (dao *conversationDao) BatchGetConvMaxSeq(ctx context.Context, conversationIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64, len(conversationIDs))
	if len(conversationIDs) == 0 {
		return result, nil
	}
	keys := make([]string, len(conversationIDs))
	for i, convID := range conversationIDs {
		keys[i] = dao.convMaxSeqKey(convID)
	}
	vals, err := dao.rdb.MGet(ctx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("批量查询会话最大序号失败: %w", err)
	}
	for i, val := range vals {
		convID := conversationIDs[i]
		if val == nil {
			result[convID] = 0
			continue
		}
		if str, ok := val.(string); ok {
			seq, parseErr := strconv.ParseInt(str, 10, 64)
			if parseErr != nil {
				result[convID] = 0
				continue
			}
			result[convID] = seq
		} else {
			result[convID] = 0
		}
	}
	return result, nil
}

// UpdateMemberReadSeq 更新会话成员的已读序号
// 双写策略：先更新 PostgreSQL（持久化真相），再更新 Redis（缓存加速）
// PG 更新使用 GORM 的 Conditions 更新，只更新 max_read_seq 字段
// Redis 缓存设置 24 小时过期，防止异常数据永久驻留
func (dao *conversationDao) UpdateMemberReadSeq(ctx context.Context, conversationID int64, userID int64, maxReadSeq int64) error {
	err := dao.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("max_read_seq", maxReadSeq).Error
	if err != nil {
		return fmt.Errorf("更新成员已读序号失败: %w", err)
	}
	if cacheErr := dao.rdb.Set(ctx, dao.memberReadSeqKey(conversationID, userID), maxReadSeq, 24*time.Hour).Err(); cacheErr != nil {
		return fmt.Errorf("缓存成员已读序号失败: %w", cacheErr)
	}
	return nil
}

// GetMemberReadSeq 获取会话成员的已读序号
// 采用缓存优先策略：Redis 命中则直接返回，未命中则回源 PostgreSQL 并回填缓存
func (dao *conversationDao) GetMemberReadSeq(ctx context.Context, conversationID int64, userID int64) (int64, error) {
	val, err := dao.rdb.Get(ctx, dao.memberReadSeqKey(conversationID, userID)).Result()
	if err == nil {
		seq, parseErr := strconv.ParseInt(val, 10, 64)
		if parseErr == nil {
			return seq, nil
		}
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("查询成员已读序号缓存失败: %w", err)
	}
	// 缓存未命中，回源数据库
	var maxReadSeq int64
	dbErr := dao.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Pluck("max_read_seq", &maxReadSeq).Error
	if dbErr != nil {
		return 0, fmt.Errorf("查询成员已读序号失败: %w", dbErr)
	}
	// 回填缓存
	_ = dao.rdb.Set(ctx, dao.memberReadSeqKey(conversationID, userID), maxReadSeq, 24*time.Hour).Err()
	return maxReadSeq, nil
}

// BatchGetMemberReadSeq 批量获取用户在多个会话中的已读序号
// 先通过 Redis MGet 批量读取缓存，缓存未命中的再回源数据库并回填
func (dao *conversationDao) BatchGetMemberReadSeq(ctx context.Context, userID int64, conversationIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64, len(conversationIDs))
	if len(conversationIDs) == 0 {
		return result, nil
	}
	// 第一级：Redis 批量读取
	keys := make([]string, len(conversationIDs))
	for i, convID := range conversationIDs {
		keys[i] = dao.memberReadSeqKey(convID, userID)
	}
	vals, err := dao.rdb.MGet(ctx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("批量查询成员已读序号缓存失败: %w", err)
	}
	var missedIDs []int64
	for i, val := range vals {
		convID := conversationIDs[i]
		if val == nil {
			missedIDs = append(missedIDs, convID)
			continue
		}
		if str, ok := val.(string); ok {
			seq, parseErr := strconv.ParseInt(str, 10, 64)
			if parseErr == nil {
				result[convID] = seq
				continue
			}
		}
		missedIDs = append(missedIDs, convID)
	}
	// 全部命中缓存
	if len(missedIDs) == 0 {
		return result, nil
	}
	// 第二级：缓存未命中的回源数据库
	var rows []model.ConversationMember
	dbErr := dao.db.WithContext(ctx).
		Select("conversation_id, max_read_seq").
		Where("user_id = ? AND conversation_id IN ?", userID, missedIDs).
		Find(&rows).Error
	if dbErr != nil {
		return nil, fmt.Errorf("批量查询成员已读序号失败: %w", dbErr)
	}
	for _, row := range rows {
		result[row.ConversationID] = row.MaxReadSeq
		// 回填缓存
		_ = dao.rdb.Set(ctx, dao.memberReadSeqKey(row.ConversationID, userID), row.MaxReadSeq, 24*time.Hour).Err()
	}
	// 未查到的会话默认已读序号为 0
	for _, convID := range missedIDs {
		if _, exists := result[convID]; !exists {
			result[convID] = 0
		}
	}
	return result, nil
}
