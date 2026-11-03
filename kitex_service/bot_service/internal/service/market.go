package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/bot_service/rpc"
	"github.com/Airiseina/answer/pkg/audit"
	"github.com/Airiseina/answer/pkg/snowflake"
	"github.com/Airiseina/answer/pkg/tenant"

	"github.com/cloudwego/kitex/pkg/klog"
)

const (
	marketDefaultPageSize = 20
	marketMaxPageSize     = 100
	marketMaxNameLen      = 64
	marketMaxDescLen      = 512
	marketMaxTagsLen      = 255
	marketMaxNoteLen      = 255
)

// MarketService 负责 Bot 市场的发布、审核、浏览、安装、评分与下架。
//
// 市场全局可见：已发布条目跨租户可读；发布管理与安装记录严格按租户隔离。
type MarketService struct {
	dao        dal.MarketBotDao
	installDao dal.MarketInstallDao
	ratingDao  dal.MarketRatingDao
	botDao     dal.BotDao
	botService *BotService
	audit      audit.Recorder
	snowNode   *snowflake.Node
}

func NewMarketService(dao dal.MarketBotDao, installDao dal.MarketInstallDao, ratingDao dal.MarketRatingDao,
	botDao dal.BotDao, botService *BotService, rec audit.Recorder) *MarketService {
	if rec == nil {
		rec = audit.NoopRecorder{}
	}
	return &MarketService{
		dao:        dao,
		installDao: installDao,
		ratingDao:  ratingDao,
		botDao:     botDao,
		botService: botService,
		audit:      rec,
		snowNode:   snowflake.NewNode(snowflake.ShardBot),
	}
}

// MarketBotDTO 是市场条目的对外视图。
type MarketBotDTO struct {
	MarketBotId   int64
	PublisherId   int64
	PublisherName string
	Name          string
	Description   string
	AvatarUrl     string
	SystemPrompt  string
	Model         string
	BaseURL       string
	Category      string
	Tags          string
	Status        int16
	ReviewNote    string
	InstallCount  int64
	Rating        float64
	RatingCount   int32
	Version       int32
	CreatedAt     int64
	UpdatedAt     int64
}

// Publish 用户提交自己编写的 Bot 到市场，落库后进入待审核状态。
func (svc *MarketService) Publish(ctx context.Context, tenantId, publisherId int64, name, systemPrompt,
	description, avatarURL, model_, baseURL, category, tags string) (int64, int16, error) {
	if !tenant.IsValid(tenantId) {
		return 0, 0, fmt.Errorf("缺少有效的租户上下文")
	}
	if strings.TrimSpace(name) == "" {
		return 0, 0, fmt.Errorf("Bot名称不能为空")
	}
	if strings.TrimSpace(systemPrompt) == "" {
		return 0, 0, fmt.Errorf("系统提示词不能为空")
	}
	// 上架前先做提示词预审：危险内容直接拦截，注入标记统一清洗。
	filtered, err := filterUserPrompt(systemPrompt)
	if err != nil {
		return 0, 0, err
	}
	item := model.MarketBot{
		ID:           svc.snowNode.Generate(),
		TenantID:     tenantId,
		PublisherID:  publisherId,
		Name:         truncateRunes(strings.TrimSpace(name), marketMaxNameLen),
		Description:  truncateRunes(description, marketMaxDescLen),
		AvatarURL:    avatarURL,
		SystemPrompt: sanitizeUserPrompt(filtered),
		Model:        model_,
		BaseURL:      baseURL,
		Category:     category,
		Tags:         truncateRunes(tags, marketMaxTagsLen),
		Status:       model.MarketStatusPending,
		// 版本从 0 起算，首次审核通过后置 1；后续每次重新过审递增，便于安装者识别更新。
		Version: 0,
	}
	if err := svc.dao.CreateMarketBot(item); err != nil {
		svc.record(ctx, audit.ActorUser, publisherId, tenantId, "publish", item.ID, "提交市场发布申请", audit.ResultFailure)
		return 0, 0, err
	}
	svc.record(ctx, audit.ActorUser, publisherId, tenantId, "publish", item.ID, "提交市场发布申请", audit.ResultSuccess)
	return item.ID, item.Status, nil
}

// Update 发布者修改自己的条目；提示词变更会重新进入待审核。
func (svc *MarketService) Update(ctx context.Context, tenantId, marketBotId, operatorId int64,
	updates map[string]interface{}) (bool, error) {
	item, err := svc.dao.GetMarketBotForPublisher(tenantId, marketBotId)
	if err != nil {
		return false, err
	}
	if item.ID == 0 {
		return false, nil
	}
	if item.PublisherID != operatorId {
		return false, nil
	}

	dbUpdates := make(map[string]interface{})
	promptChanged := false
	for k, v := range updates {
		switch k {
		case "name":
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				dbUpdates["name"] = truncateRunes(strings.TrimSpace(s), marketMaxNameLen)
			}
		case "description":
			if s, ok := v.(string); ok {
				dbUpdates["description"] = truncateRunes(s, marketMaxDescLen)
			}
		case "avatar_url":
			if s, ok := v.(string); ok {
				dbUpdates["avatar_url"] = s
			}
		case "model":
			if s, ok := v.(string); ok {
				dbUpdates["model"] = s
			}
		case "base_url":
			if s, ok := v.(string); ok {
				dbUpdates["base_url"] = s
			}
		case "category":
			if s, ok := v.(string); ok {
				dbUpdates["category"] = s
			}
		case "tags":
			if s, ok := v.(string); ok {
				dbUpdates["tags"] = truncateRunes(s, marketMaxTagsLen)
			}
		case "system_prompt":
			s, ok := v.(string)
			if !ok {
				continue
			}
			filtered, ferr := filterUserPrompt(s)
			if ferr != nil {
				return false, ferr
			}
			dbUpdates["system_prompt"] = sanitizeUserPrompt(filtered)
			promptChanged = true
		}
	}
	if len(dbUpdates) == 0 {
		return false, nil
	}
	if promptChanged {
		// 提示词是市场的核心分发内容，任何改动都必须重新过审。
		dbUpdates["status"] = model.MarketStatusPending
		dbUpdates["review_note"] = ""
		dbUpdates["reviewed_by"] = 0
		dbUpdates["reviewed_at"] = 0
	}
	if err := svc.dao.UpdateMarketBot(marketBotId, dbUpdates); err != nil {
		return false, err
	}
	action := "update"
	if promptChanged {
		action = "update_repending"
	}
	svc.record(ctx, audit.ActorUser, operatorId, tenantId, action, marketBotId, "修改市场条目", audit.ResultSuccess)
	return true, nil
}

// Get 读取市场条目；未发布内容仅发布者本人可见，其余人返回空。
func (svc *MarketService) Get(ctx context.Context, marketBotId, viewerId int64) (MarketBotDTO, error) {
	item, err := svc.dao.GetMarketBot(marketBotId)
	if err != nil {
		return MarketBotDTO{}, err
	}
	if item.ID == 0 {
		return MarketBotDTO{}, nil
	}
	if item.Status != model.MarketStatusPublished && item.PublisherID != viewerId {
		return MarketBotDTO{}, nil
	}
	dtos := svc.enrich(ctx, []model.MarketBot{item})
	return dtos[0], nil
}

// List 浏览已发布的市场条目（全局可见）。
func (svc *MarketService) List(ctx context.Context, keyword, category string, page, pageSize int32) ([]MarketBotDTO, int32, error) {
	offset, limit := normalizeMarketPage(page, pageSize)
	items, total, err := svc.dao.ListPublished(strings.TrimSpace(keyword), strings.TrimSpace(category), offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return svc.enrich(ctx, items), int32(total), nil
}

// ListMy 发布者查看自己的条目；status 传负值时返回全部（含草稿、待审、驳回、下架）。
func (svc *MarketService) ListMy(ctx context.Context, tenantId, publisherId int64, status int16,
	page, pageSize int32) ([]MarketBotDTO, int32, error) {
	if !tenant.IsValid(tenantId) {
		return nil, 0, fmt.Errorf("缺少有效的租户上下文")
	}
	offset, limit := normalizeMarketPage(page, pageSize)
	// 负值表示不过滤状态；DAO 仅在状态取值合法时才追加 status 条件。
	if status < 0 {
		status = -1
	}
	items, total, err := svc.dao.ListByPublisher(tenantId, publisherId, status, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return svc.enrich(ctx, items), int32(total), nil
}

// ListReviews 管理员审核列表，默认返回待审核条目。
func (svc *MarketService) ListReviews(ctx context.Context, status int16, page, pageSize int32) ([]MarketBotDTO, int32, error) {
	// 未显式传入合法状态（含 -1 哨兵）时，默认查看待审核队列。
	if !model.ValidMarketStatus(status) {
		status = model.MarketStatusPending
	}
	offset, limit := normalizeMarketPage(page, pageSize)
	items, total, err := svc.dao.ListByStatus(status, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return svc.enrich(ctx, items), int32(total), nil
}

// Install 安装市场 Bot：用安装者自带的 api_key/model/base_url 落一条 origin=market 的 Bot 实例。
//
// 市场只分发定义，知识库与 MCP 不随安装迁移；同一用户在相同租户内重复安装同一条目时幂等返回既有实例。
func (svc *MarketService) Install(ctx context.Context, tenantId, userId, marketBotId int64,
	name, apiKey, model_, baseURL string) (int64, int32, error) {
	if !tenant.IsValid(tenantId) {
		return 0, 0, fmt.Errorf("缺少有效的租户上下文")
	}
	if strings.TrimSpace(apiKey) == "" {
		return 0, 0, fmt.Errorf("请填写你自己的 API Key")
	}
	if strings.TrimSpace(model_) == "" {
		return 0, 0, fmt.Errorf("请选择安装 Bot 使用的模型")
	}
	item, err := svc.dao.GetMarketBot(marketBotId)
	if err != nil {
		return 0, 0, err
	}
	if item.ID == 0 {
		return 0, 0, fmt.Errorf("市场Bot不存在")
	}
	if item.Status != model.MarketStatusPublished {
		return 0, 0, fmt.Errorf("该Bot尚未上架，无法安装")
	}

	existing, err := svc.installDao.GetUserInstall(tenantId, userId, marketBotId)
	if err != nil {
		return 0, 0, err
	}
	if existing.ID != 0 {
		return existing.BotID, existing.Version, nil
	}

	installName := strings.TrimSpace(name)
	if installName == "" {
		installName = item.Name
	}
	botId, err := svc.botService.CreateBot(ctx, tenantId, userId, installName, item.SystemPrompt,
		apiKey, model_, baseURL, model.BotOriginMarket, "")
	if err != nil {
		return 0, 0, err
	}

	install := model.MarketBotInstall{
		ID:          svc.snowNode.Generate(),
		TenantID:    tenantId,
		MarketBotID: marketBotId,
		BotID:       botId,
		UserID:      userId,
		Version:     item.Version,
	}
	if err := svc.installDao.CreateMarketInstall(install); err != nil {
		// 安装记录写入失败时回收刚创建的 Bot 实例，避免出现无归属的孤儿 Bot。
		if delErr := svc.botDao.DeleteBot(tenantId, botId); delErr != nil {
			klog.CtxErrorf(ctx, "市场安装[%d]写记录失败后回滚Bot[%d]也失败: %v", marketBotId, botId, delErr)
		}
		return 0, 0, err
	}
	if err := svc.dao.IncreaseInstallCount(marketBotId); err != nil {
		klog.CtxWarnf(ctx, "市场Bot[%d]累加安装量失败: %v", marketBotId, err)
	}
	svc.record(ctx, audit.ActorUser, userId, tenantId, "install", marketBotId,
		fmt.Sprintf("安装Bot实例=%d 版本=%d", botId, item.Version), audit.ResultSuccess)
	return botId, item.Version, nil
}

// Rate 用户对已发布的市场 Bot 评分（1-5），重复评分视为改分。
func (svc *MarketService) Rate(ctx context.Context, tenantId, userId, marketBotId int64, score int32) (bool, error) {
	if score < 1 || score > 5 {
		return false, fmt.Errorf("评分必须在1-5之间")
	}
	item, err := svc.dao.GetMarketBot(marketBotId)
	if err != nil {
		return false, err
	}
	if item.ID == 0 || item.Status != model.MarketStatusPublished {
		return false, fmt.Errorf("市场Bot不存在或未发布")
	}
	rating := model.MarketBotRating{
		ID:          svc.snowNode.Generate(),
		TenantID:    tenantId,
		MarketBotID: marketBotId,
		UserID:      userId,
		Score:       int16(score),
	}
	if err := svc.ratingDao.UpsertRating(rating); err != nil {
		return false, err
	}
	return true, nil
}

// Offline 发布者主动下架自己的条目。
func (svc *MarketService) Offline(ctx context.Context, tenantId, marketBotId, operatorId int64) (bool, error) {
	item, err := svc.dao.GetMarketBotForPublisher(tenantId, marketBotId)
	if err != nil {
		return false, err
	}
	if item.ID == 0 || item.PublisherID != operatorId {
		return false, nil
	}
	if item.Status == model.MarketStatusOffline {
		return true, nil
	}
	if err := svc.dao.UpdateMarketBot(marketBotId, map[string]interface{}{
		"status": model.MarketStatusOffline,
	}); err != nil {
		return false, err
	}
	svc.record(ctx, audit.ActorUser, operatorId, tenantId, "offline", marketBotId, "发布者下架", audit.ResultSuccess)
	return true, nil
}

// Review 管理员审核：通过则发布，驳回则记录原因；通过时对已审核过的条目递增版本号。
func (svc *MarketService) Review(ctx context.Context, adminId, marketBotId int64, approve bool, note string) (bool, error) {
	item, err := svc.dao.GetMarketBot(marketBotId)
	if err != nil {
		return false, err
	}
	if item.ID == 0 {
		return false, nil
	}
	updates := map[string]interface{}{
		"reviewed_by": adminId,
		"reviewed_at": time.Now().Unix(),
		"review_note": truncateRunes(note, marketMaxNoteLen),
	}
	action := "reject"
	if approve {
		updates["status"] = model.MarketStatusPublished
		// 每次审核通过都递增版本：首次上架得到 v1，修改后重新过审得到 v2，依此类推。
		updates["version"] = item.Version + 1
		action = "approve"
	} else {
		updates["status"] = model.MarketStatusRejected
	}
	if err := svc.dao.UpdateMarketBot(marketBotId, updates); err != nil {
		return false, err
	}
	svc.record(ctx, audit.ActorAdmin, adminId, item.TenantID, action, marketBotId,
		fmt.Sprintf("审核备注=%s", note), audit.ResultSuccess)
	return true, nil
}

// enrich 批量补齐发布者名称，并换算评分均值。
func (svc *MarketService) enrich(ctx context.Context, items []model.MarketBot) []MarketBotDTO {
	ids := make([]int64, 0, len(items))
	seen := make(map[int64]bool, len(items))
	for _, it := range items {
		if it.PublisherID > 0 && !seen[it.PublisherID] {
			seen[it.PublisherID] = true
			ids = append(ids, it.PublisherID)
		}
	}
	nameMap := make(map[int64]string, len(ids))
	if len(ids) > 0 {
		names, err := rpc.GetUserNames(ctx, ids)
		if err != nil {
			klog.CtxWarnf(ctx, "查询市场发布者名称失败: %v", err)
		} else {
			for _, n := range names {
				nameMap[n.Id] = n.Name
			}
		}
	}
	dtos := make([]MarketBotDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toMarketDTO(it, nameMap[it.PublisherID]))
	}
	return dtos
}

func toMarketDTO(item model.MarketBot, publisherName string) MarketBotDTO {
	var rating float64
	if item.RatingCount > 0 {
		rating = float64(item.RatingSum) / float64(item.RatingCount)
	}
	return MarketBotDTO{
		MarketBotId:   item.ID,
		PublisherId:   item.PublisherID,
		PublisherName: publisherName,
		Name:          item.Name,
		Description:   item.Description,
		AvatarUrl:     item.AvatarURL,
		SystemPrompt:  item.SystemPrompt,
		Model:         item.Model,
		BaseURL:       item.BaseURL,
		Category:      item.Category,
		Tags:          item.Tags,
		Status:        item.Status,
		ReviewNote:    item.ReviewNote,
		InstallCount:  item.InstallCount,
		Rating:        rating,
		RatingCount:   int32(item.RatingCount),
		Version:       item.Version,
		CreatedAt:     item.CreatedAt.UnixMilli(),
		UpdatedAt:     item.UpdatedAt.UnixMilli(),
	}
}

// record 统一写入 Bot 发布相关审计日志，失败只告警不影响主流程。
func (svc *MarketService) record(ctx context.Context, actorType string, actorId, tenantId int64,
	action string, targetId int64, detail string, result int16) {
	event := audit.Event{
		EventType:  audit.EventBotPublish,
		ActorType:  actorType,
		ActorID:    actorId,
		TenantID:   tenantId,
		Action:     action,
		TargetType: "market_bot",
		TargetID:   strconv.FormatInt(targetId, 10),
		Detail:     detail,
		Result:     result,
	}
	if err := svc.audit.Record(ctx, event); err != nil {
		klog.CtxWarnf(ctx, "写入市场审计日志失败: %v", err)
	}
}

// normalizeMarketPage 归一化分页参数，返回 offset 与 limit。
func normalizeMarketPage(page, pageSize int32) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = marketDefaultPageSize
	}
	if pageSize > marketMaxPageSize {
		pageSize = marketMaxPageSize
	}
	return int(page-1) * int(pageSize), int(pageSize)
}

// truncateRunes 按字符（而非字节）截断，避免把多字节字符截成乱码。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
