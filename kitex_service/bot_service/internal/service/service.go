package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/bot_service/rpc"
	"github.com/Airiseina/answer/pkg/snowflake"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/cloudwego/kitex/pkg/klog"
)

// 安全提示词注入相关：用于对用户创建的Bot进行prompt审核和安全规则追加
var (
	userBotSafetyPrompt string // 用户Bot安全提示词模板，启动时加载
	safetyPromptOnce    bool   // 是否已尝试加载
)

// loadUserBotSafetyPrompt 加载用户Bot安全提示词模板
func loadUserBotSafetyPrompt() string {
	if safetyPromptOnce {
		return userBotSafetyPrompt
	}
	safetyPromptOnce = true
	v := config.V
	promptFile := v.GetString("ai.user_bot.safety_prompt_file")
	if promptFile == "" {
		klog.Warn("未配置ai.user_bot.safety_prompt_file，用户Bot安全提示词为空")
		return ""
	}
	data, err := os.ReadFile(promptFile)
	if err != nil {
		klog.Warnf("读取用户Bot安全提示词文件[%s]失败: %v", promptFile, err)
		return ""
	}
	userBotSafetyPrompt = strings.TrimSpace(string(data))
	klog.Infof("用户Bot安全提示词加载成功，长度=%d", len(userBotSafetyPrompt))
	return userBotSafetyPrompt
}

// 危险内容过滤正则：检测用户prompt中可能试图覆盖安全规则的模式
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(忽略|忽略以上|ignore\s+(all\s+)?previous|ignore\s+above)\s*(指令|规则|prompt|instruction)`),
	regexp.MustCompile(`(?i)(你是|你现在是|act\s+as|pretend\s+to\s+be|you\s+are\s+now)\s*(一个\s*)?(开发者|管理员|admin|developer|system)`),
	regexp.MustCompile(`(?i)(system\s*prompt|系统提示词|原始指令|original\s+instruction).{0,20}(输出|显示|重复|reveal|show|repeat|output)`),
	regexp.MustCompile(`(?i)(jailbreak|越狱|解除限制|remove\s+restrictions|bypass)`),
	regexp.MustCompile(`(?i)(DAN\s+mode|developer\s+mode|开发者模式|god\s+mode)`),
}

// filterUserPrompt 对用户提供的系统提示词进行安全审核和过滤
// 返回过滤后的安全prompt，如果检测到严重违规内容则返回错误
func filterUserPrompt(prompt string) (string, error) {
	if prompt == "" {
		return "", nil
	}
	// 检测危险模式
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(prompt) {
			klog.Warnf("用户Bot prompt包含危险模式[%s]，已拦截", pattern.String())
			return "", fmt.Errorf("系统提示词包含不允许的内容，请修改后重试")
		}
	}
	return prompt, nil
}

// sanitizeUserPrompt 对用户prompt进行清洗，移除可能干扰安全规则的内容
func sanitizeUserPrompt(prompt string) string {
	// 移除可能被用来注入分隔的标记
	replacements := []struct {
		pattern *regexp.Regexp
		repl    string
	}{
		{regexp.MustCompile(`(?i)===+\s*system\s*===+`), "---"},
		{regexp.MustCompile(`(?i)\[system\]`), ""},
		{regexp.MustCompile(`(?i)<system>`), ""},
		{regexp.MustCompile(`(?i)</system>`), ""},
	}
	result := prompt
	for _, r := range replacements {
		result = r.pattern.ReplaceAllString(result, r.repl)
	}
	return result
}

// stripSafetyPrompt 从prompt中剥离已拼接的安全提示词（兼容旧数据）
func stripSafetyPrompt(prompt string) string {
	safety := loadUserBotSafetyPrompt()
	if safety == "" || prompt == "" {
		return prompt
	}
	// 安全词以 "\n\n" + safety 拼接在末尾
	suffix := "\n\n" + safety
	if strings.HasSuffix(prompt, suffix) {
		return strings.TrimSuffix(prompt, suffix)
	}
	// 兜底：直接匹配安全词
	if strings.HasSuffix(prompt, safety) {
		return strings.TrimSuffix(prompt, safety)
	}
	return prompt
}

type BotService struct {
	dao      dal.BotDao
	snowNode *snowflake.Node
}

func NewBotService(dao dal.BotDao) *BotService {
	return &BotService{
		dao:      dao,
		snowNode: snowflake.NewNode(5),
	}
}

func (svc *BotService) CreateBot(ctx context.Context, creatorId int64, name, systemPrompt, apiKey, model_, baseURL string) (int64, error) {
	// 对用户提供的系统提示词进行安全审核
	filtered, err := filterUserPrompt(systemPrompt)
	if err != nil {
		return 0, err
	}
	// 清洗用户prompt中的注入标记（只存用户原始prompt，安全词在运行时动态拼接）
	cleaned := sanitizeUserPrompt(filtered)

	botId := svc.snowNode.Generate()
	userID, rpcErr := rpc.CreateBotUser(ctx, name, "")
	if rpcErr != nil {
		return 0, fmt.Errorf("创建Bot用户记录失败: %w", rpcErr)
	}
	bot := model.Bot{
		ID:           botId,
		UserID:       userID,
		CreatorID:    creatorId,
		SystemPrompt: cleaned,
		ApiKey:       apiKey,
		Model:        model_,
		BaseURL:      baseURL,
		IsSystem:     false,
	}
	err = svc.dao.CreateBot(bot)
	if err != nil {
		return 0, err
	}
	return botId, nil
}

type BotInfoDTO struct {
	BotId        int64
	UserId       int64
	CreatorId    int64
	Name         string
	AvatarUrl    string
	ApiKey       string
	SystemPrompt string
	Model        string
	BaseURL      string
	IsSystem     bool
	CreatedAt    int64
}

func (svc *BotService) enrichBotInfo(ctx context.Context, info model.Bot) BotInfoDTO {
	dto := BotInfoDTO{
		BotId:        info.ID,
		UserId:       info.UserID,
		CreatorId:    info.CreatorID,
		ApiKey:       info.ApiKey,
		SystemPrompt: stripSafetyPrompt(info.SystemPrompt), // 剥离安全词，只返回用户原始prompt
		Model:        info.Model,
		BaseURL:      info.BaseURL,
		IsSystem:     info.IsSystem,
		CreatedAt:    info.CreatedAt.UnixMilli(),
	}
	if info.UserID > 0 {
		names, err := rpc.GetUserNames(ctx, []int64{info.UserID})
		if err == nil && len(names) > 0 {
			dto.Name = names[0].Name
			dto.AvatarUrl = names[0].AvatarURL
		}
	}
	return dto
}

func (svc *BotService) GetBot(botId int64) (BotInfoDTO, error) {
	info, err := svc.dao.GetBot(botId)
	if err != nil {
		return BotInfoDTO{}, err
	}
	return svc.enrichBotInfo(context.Background(), info), nil
}

func (svc *BotService) GetSystemBot() (int64, error) {
	info, err := svc.dao.GetSystemBot()
	if err != nil {
		return 0, err
	}
	return info.ID, nil
}

func (svc *BotService) GetUserBots(creatorId int64) ([]BotInfoDTO, error) {
	infos, err := svc.dao.GetUserBots(creatorId)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	var dtos []BotInfoDTO
	for _, info := range infos {
		dtos = append(dtos, svc.enrichBotInfo(ctx, info))
	}
	return dtos, nil
}

func (svc *BotService) UpdateBot(botId, operatorId int64, updates map[string]interface{}) (bool, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		return false, err
	}
	if bot.ID == 0 {
		return false, nil
	}
	if bot.IsSystem {
		return false, nil
	}
	if bot.CreatorID != operatorId {
		return false, nil
	}
	botUpdates := make(map[string]interface{})
	for k, v := range updates {
		if k == "name" || k == "avatar_url" {
			continue
		}
		// 对用户更新的system_prompt进行安全审核（只存用户原始prompt，安全词在运行时动态拼接）
		if k == "system_prompt" {
			newPrompt, ok := v.(string)
			if !ok {
				continue
			}
			filtered, filterErr := filterUserPrompt(newPrompt)
			if filterErr != nil {
				return false, filterErr
			}
			botUpdates[k] = sanitizeUserPrompt(filtered)
			continue
		}
		botUpdates[k] = v
	}
	if len(botUpdates) > 0 {
		err = svc.dao.UpdateBot(botId, botUpdates)
		if err != nil {
			return false, err
		}
	}
	if newName, ok := updates["name"].(string); ok && bot.UserID > 0 {
		if syncErr := rpc.UpdateBotUserName(context.Background(), bot.UserID, newName); syncErr != nil {
			klog.Warnf("Bot[%d]名称已更新但同步user_service失败: %v", botId, syncErr)
		}
	}
	if newAvatar, ok := updates["avatar_url"].(string); ok && bot.UserID > 0 {
		if syncErr := rpc.UpdateBotUserAvatar(context.Background(), bot.UserID, newAvatar); syncErr != nil {
			klog.Warnf("Bot[%d]头像已更新但同步user_service失败: %v", botId, syncErr)
		}
	}
	return true, nil
}

func (svc *BotService) DeleteBot(botId, operatorId int64) (bool, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		return false, err
	}
	if bot.ID == 0 {
		return false, nil
	}
	if bot.IsSystem {
		return false, nil
	}
	if bot.CreatorID != operatorId {
		return false, nil
	}
	err = svc.dao.DeleteBot(botId)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (svc *BotService) IsBot(userId int64) (bool, int64, error) {
	bot, err := svc.dao.GetBotByUserId(userId)
	if err != nil {
		return false, 0, err
	}
	if bot.ID == 0 {
		return false, 0, nil
	}
	return true, bot.ID, nil
}

const (
	ConvTypePrivate int16 = 1
	ConvTypeGroup   int16 = 2
)

func (svc *BotService) AddBotToConversation(ctx context.Context, operatorId, botId, conversationId int64, convType int16) (int64, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		return 0, fmt.Errorf("查询Bot失败: %w", err)
	}
	if bot.ID == 0 {
		return 0, fmt.Errorf("bot不存在")
	}
	if !bot.IsSystem && bot.CreatorID != operatorId {
		return 0, fmt.Errorf("只有Bot创建者才能将Bot拉入会话")
	}
	if bot.UserID == 0 {
		return 0, fmt.Errorf("bot用户记录异常，缺少user_id")
	}
	switch convType {
	case ConvTypeGroup:
		err = rpc.AddConversationMembers(ctx, conversationId, []int64{bot.UserID})
		if err != nil {
			return 0, fmt.Errorf("将Bot加入群聊会话失败: %w", err)
		}
		return conversationId, nil
	case ConvTypePrivate:
		if conversationId == 0 {
			convID, err := rpc.GetOrCreatePrivateConversation(ctx, operatorId, bot.UserID)
			if err != nil {
				return 0, fmt.Errorf("创建Bot单聊会话失败: %w", err)
			}
			return convID, nil
		}
		err = rpc.AddConversationMembers(ctx, conversationId, []int64{bot.UserID})
		if err != nil {
			return 0, fmt.Errorf("将Bot加入私聊会话失败: %w", err)
		}
		return conversationId, nil
	default:
		return 0, fmt.Errorf("不支持的会话类型: %d", convType)
	}
}

func (svc *BotService) InitSystemBot(ctx context.Context) (int64, error) {
	bot, err := svc.dao.GetSystemBot()
	if err != nil {
		return 0, err
	}
	if bot.ID != 0 {
		klog.Infof("系统Bot已存在, ID: %d，同步更新prompt和头像", bot.ID)
		svc.syncSystemBotFromFiles(ctx, bot)
		return bot.ID, nil
	}
	botId := svc.snowNode.Generate()
	v := config.V
	name := v.GetString("ai.system.bot_name")
	systemPrompt := v.GetString("ai.system.bot_prompt")
	promptFile := v.GetString("ai.system.bot_prompt_file")
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			klog.Warnf("读取系统Bot prompt文件失败: %v, 使用默认prompt", err)
		} else {
			systemPrompt = string(data)
		}
	}
	systemBot := model.Bot{
		ID:           botId,
		CreatorID:    0,
		SystemPrompt: systemPrompt,
		ApiKey:       strings.TrimSpace(v.GetString("ai.system.bot_api_key")),
		Model:        strings.TrimSpace(v.GetString("ai.system.bot_model")),
		BaseURL:      strings.TrimSpace(v.GetString("ai.system.bot_base_url")),
		IsSystem:     true,
	}
	err = svc.dao.CreateBot(systemBot)
	if err != nil {
		return 0, fmt.Errorf("初始化系统Bot失败: %w", err)
	}
	avatarURL := svc.uploadSystemBotAvatar(ctx, botId)
	userID, rpcErr := rpc.CreateBotUser(ctx, name, avatarURL)
	if rpcErr != nil {
		delErr := svc.dao.DeleteBot(botId)
		if delErr != nil {
			klog.Errorf("系统Bot[%d]创建用户记录失败后回滚删除也失败: %v", botId, delErr)
		}
		return 0, fmt.Errorf("系统Bot创建用户记录失败: %w", rpcErr)
	}
	err = svc.dao.UpdateBot(botId, map[string]interface{}{"user_id": userID})
	if err != nil {
		klog.Errorf("系统Bot[%d]更新user_id失败: %v", botId, err)
	}
	klog.Infof("系统Bot创建成功, ID: %d, UserID: %d", botId, userID)

	svc.initSkillKnowledgeBase(ctx, botId)

	return botId, nil
}

// syncSystemBotFromFiles 从文件同步系统Bot的prompt和头像
// 每次启动时调用，如果文件内容与数据库不同则更新
func (svc *BotService) syncSystemBotFromFiles(ctx context.Context, bot model.Bot) {
	v := config.V
	updates := make(map[string]interface{})

	// 同步prompt
	promptFile := v.GetString("ai.system.bot_prompt_file")
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			klog.Warnf("系统Bot[%d]读取prompt文件[%s]失败: %v", bot.ID, promptFile, err)
		} else {
			newPrompt := string(data)
			if newPrompt != bot.SystemPrompt {
				updates["system_prompt"] = newPrompt
				klog.Infof("系统Bot[%d] prompt已变更，将更新", bot.ID)
			}
		}
	}

	// 同步头像：计算新头像的objectName，与当前头像URL对比
	newAvatarURL := svc.computeSystemBotAvatarURL()
	if newAvatarURL != "" && bot.UserID != 0 {
		// 获取当前用户头像URL进行对比
		users, err := rpc.GetUserNames(ctx, []int64{bot.UserID})
		if err != nil || len(users) == 0 {
			klog.Warnf("系统Bot[%d]获取当前头像失败: %v", bot.ID, err)
		} else if users[0].AvatarURL != newAvatarURL {
			// 头像有变化，上传新头像并更新
			svc.uploadSystemBotAvatar(ctx, bot.ID)
			if err := rpc.UpdateBotUserAvatar(ctx, bot.UserID, newAvatarURL); err != nil {
				klog.Warnf("系统Bot[%d]更新头像失败: %v", bot.ID, err)
			} else {
				klog.Infof("系统Bot[%d]头像已更新: %s", bot.ID, newAvatarURL)
			}
		}
	}

	// 更新数据库
	if len(updates) > 0 {
		if err := svc.dao.UpdateBot(bot.ID, updates); err != nil {
			klog.Errorf("系统Bot[%d]同步更新失败: %v", bot.ID, err)
		} else {
			var keys []string
			for k := range updates {
				keys = append(keys, k)
			}
			klog.Infof("系统Bot[%d]同步更新成功，更新字段: %v", bot.ID, keys)
		}
	}
}

// computeSystemBotAvatarURL 计算系统Bot头像的URL（不上传，仅计算路径）
func (svc *BotService) computeSystemBotAvatarURL() string {
	entries, err := os.ReadDir("avatar")
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
			continue
		}
		filePath := filepath.Join("avatar", entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil || len(data) == 0 {
			continue
		}
		contentHash := storage.ComputeContentHash(data)
		objectName := storage.GenerateObjectName(contentHash, ext)
		return storage.PublicURL + storage.BasePath + "/" + objectName
	}
	return ""
}

func (svc *BotService) uploadSystemBotAvatar(ctx context.Context, botID int64) string {
	entries, err := os.ReadDir("avatar")
	if err != nil {
		klog.Warnf("系统Bot[%d]读取avatar目录失败: %v", botID, err)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
			continue
		}
		filePath := filepath.Join("avatar", entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			klog.Warnf("系统Bot[%d]读取头像文件[%s]失败: %v", botID, filePath, err)
			continue
		}
		if len(data) == 0 {
			continue
		}
		contentHash := storage.ComputeContentHash(data)
		objectName := storage.GenerateObjectName(contentHash, ext)
		contentType := "image/png"
		if ext != ".png" {
			contentType = "image/" + ext[1:]
		}
		if err := storage.Client.PutObject(ctx, objectName, bytes.NewReader(data), contentType); err != nil {
			klog.Errorf("系统Bot[%d]上传头像[%s]失败: %v", botID, filePath, err)
			return ""
		}
		url := storage.PublicURL + storage.BasePath + "/" + objectName
		klog.Infof("系统Bot[%d]头像上传成功: %s", botID, url)
		return url
	}
	klog.Warnf("系统Bot[%d]未在avatar目录中找到图片文件", botID)
	return ""
}

var skillFiles = []string{
	"SKILL.md",
	"profile.md",
	"personality.md",
	"interaction.md",
	"relations.md",
	"conflicts.md",
	"background_story.md",
	"memory.md",
	"references/tone-guide.md",
	"references/tone-engine.md",
	"references/scene-dialogues.md",
	"references/vocal-mannerisms.md",
}

func (svc *BotService) initSkillKnowledgeBase(ctx context.Context, botID int64) {
	v := config.V
	skillDir := v.GetString("ai.system.bot_skill_dir")
	if skillDir == "" {
		klog.Warn("未配置ai.system.bot_skill_dir，跳过Skill知识库初始化")
		return
	}

	// 重试创建知识库，等待knowledge_service就绪
	var kbID int64
	var err error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		kbID, err = rpc.CreateKnowledgeBase(ctx, 0, "kiana角色Skill", "琪亚娜·卡斯兰娜角色扮演数据")
		if err == nil {
			break
		}
		klog.Warnf("系统Bot[%d]创建Skill知识库失败(第%d次): %v，3秒后重试...", botID, i+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		klog.Errorf("系统Bot[%d]创建Skill知识库失败(已重试%d次): %v", botID, maxRetries, err)
		return
	}
	klog.Infof("系统Bot[%d]创建Skill知识库成功, KB ID: %d", botID, kbID)

	if err := rpc.BindSystemKnowledgeBase(ctx, botID, kbID); err != nil {
		klog.Errorf("系统Bot[%d]绑定Skill知识库[%d]失败: %v", botID, kbID, err)
		return
	}
	klog.Infof("系统Bot[%d]绑定Skill知识库[%d]成功", botID, kbID)

	for _, f := range skillFiles {
		filePath := filepath.Join(skillDir, f)
		svc.uploadSkillFile(ctx, kbID, filePath, f)
	}
	klog.Infof("系统Bot[%d] Skill知识库[%d]初始化完成", botID, kbID)
}

func (svc *BotService) uploadSkillFile(ctx context.Context, kbID int64, filePath, relativePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		klog.Warnf("读取Skill文件[%s]失败: %v", filePath, err)
		return
	}
	if len(data) == 0 {
		klog.Warnf("Skill文件[%s]为空，跳过", filePath)
		return
	}

	contentHash := storage.ComputeContentHash(data)
	ext := filepath.Ext(relativePath)
	objectName := storage.GenerateObjectName(contentHash, ext)

	if err := storage.Client.PutObject(ctx, objectName, bytes.NewReader(data), "text/markdown"); err != nil {
		klog.Errorf("上传Skill文件[%s]到SeaweedFS失败: %v", relativePath, err)
		return
	}

	fileURL := storage.PublicURL + storage.BasePath + "/" + objectName
	fileName := strings.TrimSuffix(filepath.Base(relativePath), ext)

	_, err = rpc.AddSystemDocument(ctx, kbID, fileName, fileURL, "md", int64(len(data)))
	if err != nil {
		klog.Errorf("添加Skill文件[%s]到知识库[%d]失败: %v", relativePath, kbID, err)
		return
	}
	klog.Infof("Skill文件[%s]上传成功, 知识库[%d]", relativePath, kbID)
}
