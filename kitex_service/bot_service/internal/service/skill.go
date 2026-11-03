package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/bot_service/rpc"
	"github.com/Airiseina/answer/pkg/audit"
	"github.com/Airiseina/answer/pkg/snowflake"
	"github.com/Airiseina/answer/pkg/storage"
	"github.com/Airiseina/answer/pkg/tenant"

	"github.com/cloudwego/kitex/pkg/klog"
)

// 个人 Skill 包的解析与安全限制。
//
// 本轮只接受「资料型 Skill」：zip 包（含 SKILL.md 与 references/*.md）或单个 markdown 文件，
// 内容全部落在该 Skill 独占的知识库里作为检索资料，运行期不执行任何脚本。
const (
	skillMaxPackageBytes = 10 << 20 // 单个 Skill 包压缩后上限
	skillMaxExpandBytes  = 20 << 20 // 解压后总量上限，防止 zip 炸弹
	skillMaxFileBytes    = 2 << 20  // 单个资料文件上限
	skillMaxFiles        = 50       // 单个 Skill 最多包含的文件数
	skillMaxNameLen      = 64
	skillMaxDescLen      = 512
	skillMaxVersionLen   = 32
	skillManifestName    = "SKILL.md"
)

// 允许的资料文件扩展名：仅纯文本资料，可被知识库解析为检索片段。
var skillAllowedExt = map[string]bool{
	".md":       true,
	".markdown": true,
	".txt":      true,
}

// skillFile 是 Skill 包内的一份资料文件。
type skillFile struct {
	RelPath string
	Content []byte
}

// SkillService 负责个人 Skill 的上传解析、知识库落库、Bot 绑定与删除。
//
// 隔离规则：Skill 严格按「租户 + 上传者」归属；绑定只允许 Bot 创建者操作；
// Skill 的知识库不随市场分发，也不对其他用户可见。
type SkillService struct {
	dao      dal.UserSkillDao
	botDao   dal.BotDao
	audit    audit.Recorder
	snowNode *snowflake.Node
}

func NewSkillService(dao dal.UserSkillDao, botDao dal.BotDao, rec audit.Recorder) *SkillService {
	if rec == nil {
		rec = audit.NoopRecorder{}
	}
	return &SkillService{
		dao:      dao,
		botDao:   botDao,
		audit:    rec,
		snowNode: snowflake.NewNode(snowflake.ShardKnowledge),
	}
}

// SkillDTO 是个人 Skill 的对外视图。
type SkillDTO struct {
	SkillId     int64
	OwnerId     int64
	Name        string
	Description string
	Version     string
	Status      int16
	KbId        int64
	SizeBytes   int64
	FileCount   int32
	CreatedAt   int64
	UpdatedAt   int64
}

// SkillFileDTO 是 Skill 内单个资料文件的对外视图。
type SkillFileDTO struct {
	FileId    int64
	SkillId   int64
	RelPath   string
	ObjectUrl string
	SizeBytes int64
}

// SkillUploadResult 汇总一次上传的结果，warnings 记录被跳过的文件原因。
type SkillUploadResult struct {
	SkillId   int64
	FileCount int32
	SizeBytes int64
	Warnings  []string
}

// Upload 解析并保存一个 Skill 包（zip 或多文件拼装的目录）。
//
// 同名 Skill 重复上传视为覆盖更新：复用原知识库，先清理旧文件行再写入新快照，
// 保证 /api/skill/list 里不会出现同名重复条目。
func (svc *SkillService) Upload(ctx context.Context, tenantId, ownerId int64, fileName string, data []byte) (SkillUploadResult, error) {
	if !tenant.IsValid(tenantId) {
		return SkillUploadResult{}, fmt.Errorf("缺少有效的租户上下文")
	}
	if ownerId <= 0 {
		return SkillUploadResult{}, fmt.Errorf("缺少有效的用户身份")
	}
	if len(data) == 0 {
		return SkillUploadResult{}, fmt.Errorf("Skill 包内容为空")
	}
	if int64(len(data)) > skillMaxPackageBytes {
		return SkillUploadResult{}, fmt.Errorf("Skill 包不能超过 %d MB", skillMaxPackageBytes>>20)
	}
	files, warnings, err := parseSkillPackage(fileName, data)
	if err != nil {
		return SkillUploadResult{}, err
	}
	if len(files) == 0 {
		return SkillUploadResult{}, fmt.Errorf("Skill 包中没有可用的资料文件")
	}
	name, description, version := readSkillManifest(files)
	if strings.TrimSpace(name) == "" {
		name = defaultSkillName(fileName)
	}
	if strings.TrimSpace(name) == "" {
		return SkillUploadResult{}, fmt.Errorf("无法确定 Skill 名称，请在 SKILL.md 的 frontmatter 中填写 name")
	}
	name = truncateRunes(strings.TrimSpace(name), skillMaxNameLen)
	description = truncateRunes(strings.TrimSpace(description), skillMaxDescLen)
	version = truncateRunes(strings.TrimSpace(version), skillMaxVersionLen)

	existing, err := svc.dao.GetUserSkillByName(tenantId, ownerId, name)
	if err != nil {
		return SkillUploadResult{}, err
	}
	kbId := existing.KbID
	if kbId == 0 {
		kbId, err = rpc.CreateKnowledgeBaseInTenant(ctx, tenantId, ownerId,
			truncateRunes(name+" Skill", skillMaxNameLen+6), description)
		if err != nil {
			return SkillUploadResult{}, fmt.Errorf("为Skill创建知识库失败: %w", err)
		}
	}

	skillId := existing.ID
	if skillId == 0 {
		skillId = svc.snowNode.Generate()
		if err := svc.dao.CreateUserSkill(model.UserSkill{
			ID:          skillId,
			TenantID:    tenantId,
			OwnerID:     ownerId,
			Name:        name,
			Description: description,
			Version:     version,
			Status:      model.UserSkillStatusPending,
			KbID:        kbId,
		}); err != nil {
			return SkillUploadResult{}, err
		}
	} else if err := svc.dao.UpdateUserSkill(tenantId, skillId, map[string]interface{}{
		"description": description,
		"version":     version,
		"status":      model.UserSkillStatusPending,
	}); err != nil {
		return SkillUploadResult{}, err
	}
	// 覆盖更新时先清空旧的资料行，再按本次快照重新写入。
	if err := svc.dao.DeleteSkillFiles(tenantId, skillId); err != nil {
		return SkillUploadResult{}, err
	}

	var (
		fileCount int32
		sizeBytes int64
	)
	for _, f := range files {
		fileURL, uploadErr := svc.uploadSkillFile(ctx, f)
		if uploadErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", f.RelPath, uploadErr))
			continue
		}
		fileName4Doc := strings.TrimSuffix(path.Base(f.RelPath), path.Ext(f.RelPath))
		if _, docErr := rpc.AddUserDocument(ctx, tenantId, kbId, ownerId, fileName4Doc, fileURL,
			strings.TrimPrefix(path.Ext(f.RelPath), "."), int64(len(f.Content))); docErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", f.RelPath, docErr))
			continue
		}
		if err := svc.dao.CreateSkillFile(model.UserSkillFile{
			ID:        svc.snowNode.Generate(),
			TenantID:  tenantId,
			SkillID:   skillId,
			RelPath:   f.RelPath,
			ObjectURL: fileURL,
			Hash:      storage.ComputeContentHash(f.Content),
			SizeBytes: int64(len(f.Content)),
		}); err != nil {
			return SkillUploadResult{}, err
		}
		fileCount++
		sizeBytes += int64(len(f.Content))
	}

	status := model.UserSkillStatusReady
	if fileCount == 0 {
		status = model.UserSkillStatusFailed
	}
	if err := svc.dao.UpdateUserSkill(tenantId, skillId, map[string]interface{}{
		"status":     status,
		"kb_id":      kbId,
		"file_count": fileCount,
		"size_bytes": sizeBytes,
	}); err != nil {
		return SkillUploadResult{}, err
	}
	svc.record(ctx, ownerId, tenantId, "upload", skillId,
		fmt.Sprintf("文件=%d 大小=%d 警告=%d", fileCount, sizeBytes, len(warnings)),
		auditResult(status == model.UserSkillStatusReady))
	return SkillUploadResult{
		SkillId:   skillId,
		FileCount: fileCount,
		SizeBytes: sizeBytes,
		Warnings:  warnings,
	}, nil
}

// uploadSkillFile 把资料文件写入 SeaweedFS，返回可被知识库解析的公开地址。
func (svc *SkillService) uploadSkillFile(ctx context.Context, f skillFile) (string, error) {
	ext := path.Ext(f.RelPath)
	objectName := storage.GenerateObjectName(storage.ComputeContentHash(f.Content), ext)
	if err := storage.Client.PutObject(ctx, objectName, bytes.NewReader(f.Content), "text/markdown"); err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}
	return storage.PublicURL + storage.BasePath + "/" + objectName, nil
}

// List 返回当前用户在该租户下的全部 Skill。
func (svc *SkillService) List(tenantId, ownerId int64) ([]SkillDTO, error) {
	if !tenant.IsValid(tenantId) {
		return nil, fmt.Errorf("缺少有效的租户上下文")
	}
	skills, err := svc.dao.ListUserSkills(tenantId, ownerId)
	if err != nil {
		return nil, err
	}
	dtos := make([]SkillDTO, 0, len(skills))
	for _, s := range skills {
		dtos = append(dtos, toSkillDTO(s))
	}
	return dtos, nil
}

// Get 读取 Skill 详情与其资料文件；不属于本人或本租户时返回空。
func (svc *SkillService) Get(tenantId, skillId int64) (SkillDTO, []SkillFileDTO, error) {
	skill, err := svc.dao.GetUserSkill(tenantId, skillId)
	if err != nil {
		return SkillDTO{}, nil, err
	}
	if skill.ID == 0 {
		return SkillDTO{}, nil, nil
	}
	files, err := svc.dao.ListSkillFiles(tenantId, skillId)
	if err != nil {
		return SkillDTO{}, nil, err
	}
	fileDTOs := make([]SkillFileDTO, 0, len(files))
	for _, f := range files {
		fileDTOs = append(fileDTOs, SkillFileDTO{
			FileId:    f.ID,
			SkillId:   f.SkillID,
			RelPath:   f.RelPath,
			ObjectUrl: f.ObjectURL,
			SizeBytes: f.SizeBytes,
		})
	}
	return toSkillDTO(skill), fileDTOs, nil
}

// Delete 删除 Skill：先解绑所有 Bot 上的绑定并回收知识库，再清理本地记录。
func (svc *SkillService) Delete(ctx context.Context, tenantId, skillId, operatorId int64) (bool, error) {
	skill, err := svc.dao.GetUserSkill(tenantId, skillId)
	if err != nil {
		return false, err
	}
	if skill.ID == 0 || skill.OwnerID != operatorId {
		return false, nil
	}
	binds, err := svc.dao.ListBotSkillsBySkill(tenantId, skillId)
	if err != nil {
		return false, err
	}
	// 先按 Bot 维度解除知识库绑定，避免留下指向已删除知识库的 bot_knowledge 记录。
	if skill.KbID > 0 {
		for _, b := range binds {
			if unbindErr := rpc.UnbindKnowledgeBase(ctx, tenantId, b.BotID, operatorId, skill.KbID); unbindErr != nil {
				klog.CtxWarnf(ctx, "Skill[%d]从Bot[%d]解绑知识库失败: %v", skillId, b.BotID, unbindErr)
			}
		}
	}
	if err := svc.dao.DeleteBotSkillsBySkill(tenantId, skillId); err != nil {
		return false, err
	}
	if skill.KbID > 0 {
		if delErr := rpc.DeleteKnowledgeBase(ctx, tenantId, skill.KbID, operatorId); delErr != nil {
			klog.CtxWarnf(ctx, "Skill[%d]回收知识库[%d]失败: %v", skillId, skill.KbID, delErr)
		}
	}
	if err := svc.dao.DeleteSkillFiles(tenantId, skillId); err != nil {
		return false, err
	}
	if err := svc.dao.DeleteUserSkill(tenantId, skillId); err != nil {
		return false, err
	}
	svc.record(ctx, operatorId, tenantId, "delete", skillId, "删除个人Skill", audit.ResultSuccess)
	return true, nil
}

// Bind 把 Skill 绑定到或从 Bot 上解绑；绑定后该 Skill 的知识库并入既有检索链路。
//
// 只允许 Bot 创建者操作，且 Skill 必须属于操作者本人；重复绑定幂等返回。
func (svc *SkillService) Bind(ctx context.Context, tenantId, ownerId, botId, skillId int64, enabled bool) (bool, error) {
	if !tenant.IsValid(tenantId) {
		return false, fmt.Errorf("缺少有效的租户上下文")
	}
	bot, err := svc.botDao.GetBot(tenantId, botId)
	if err != nil {
		return false, err
	}
	if bot.ID == 0 {
		return false, fmt.Errorf("Bot不存在")
	}
	if bot.IsSystem {
		return false, fmt.Errorf("系统Bot不支持绑定个人Skill")
	}
	if bot.CreatorID != ownerId {
		return false, fmt.Errorf("只有Bot创建者才能绑定Skill")
	}
	skill, err := svc.dao.GetUserSkill(tenantId, skillId)
	if err != nil {
		return false, err
	}
	if skill.ID == 0 {
		return false, fmt.Errorf("Skill不存在")
	}
	if skill.OwnerID != ownerId {
		return false, fmt.Errorf("无权使用该Skill")
	}

	bound, err := svc.skillBoundToBot(ctx, tenantId, botId, skill.KbID)
	if err != nil {
		klog.CtxWarnf(ctx, "查询Bot[%d]知识库绑定失败: %v", botId, err)
	}
	if enabled {
		if !bound {
			if err := rpc.BindKnowledgeBase(ctx, tenantId, botId, ownerId, skill.KbID); err != nil {
				return false, fmt.Errorf("把Skill知识库绑定到Bot失败: %w", err)
			}
		}
		if err := svc.dao.UpsertBotSkill(model.BotSkill{
			TenantID: tenantId,
			BotID:    botId,
			SkillID:  skillId,
			OwnerID:  ownerId,
			Enabled:  true,
		}); err != nil {
			return false, err
		}
		svc.record(ctx, ownerId, tenantId, "bind", skillId,
			fmt.Sprintf("绑定到Bot=%d", botId), audit.ResultSuccess)
		return true, nil
	}

	if bound && skill.KbID > 0 {
		if unbindErr := rpc.UnbindKnowledgeBase(ctx, tenantId, botId, ownerId, skill.KbID); unbindErr != nil {
			return false, fmt.Errorf("解除Skill知识库绑定失败: %w", unbindErr)
		}
	}
	if err := svc.dao.DeleteBotSkill(tenantId, botId, skillId); err != nil {
		return false, err
	}
	svc.record(ctx, ownerId, tenantId, "unbind", skillId,
		fmt.Sprintf("从Bot=%d解绑", botId), audit.ResultSuccess)
	return true, nil
}

// skillBoundToBot 判断 Skill 的知识库是否已经绑定在该 Bot 上，避免重复写 bot_knowledge。
func (svc *SkillService) skillBoundToBot(ctx context.Context, tenantId, botId, kbId int64) (bool, error) {
	if kbId <= 0 {
		return false, nil
	}
	kbIds, err := rpc.GetBotKnowledgeBaseIDs(ctx, tenantId, botId)
	if err != nil {
		return false, err
	}
	for _, id := range kbIds {
		if id == kbId {
			return true, nil
		}
	}
	return false, nil
}

// ListBotSkills 返回某个 Bot 已绑定的 Skill 列表，仅 Bot 创建者可读。
func (svc *SkillService) ListBotSkills(ctx context.Context, tenantId, botId, operatorId int64) ([]BotSkillDTO, error) {
	bot, err := svc.botDao.GetBot(tenantId, botId)
	if err != nil {
		return nil, err
	}
	if bot.ID == 0 || bot.CreatorID != operatorId {
		return nil, nil
	}
	binds, err := svc.dao.ListBotSkills(tenantId, botId)
	if err != nil {
		return nil, err
	}
	dtos := make([]BotSkillDTO, 0, len(binds))
	for _, b := range binds {
		skill, getErr := svc.dao.GetUserSkill(tenantId, b.SkillID)
		if getErr != nil {
			return nil, getErr
		}
		if skill.ID == 0 {
			continue
		}
		dtos = append(dtos, BotSkillDTO{
			SkillId:     skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
			Version:     skill.Version,
			Enabled:     b.Enabled,
		})
	}
	return dtos, nil
}

// BotSkillDTO 是 Bot 已绑定 Skill 的对外视图。
type BotSkillDTO struct {
	SkillId     int64
	Name        string
	Description string
	Version     string
	Enabled     bool
}

// record 统一写入 Skill 相关审计日志，失败只告警不影响主流程。
func (svc *SkillService) record(ctx context.Context, actorId, tenantId int64, action string,
	targetId int64, detail string, result int16) {
	event := audit.Event{
		EventType:  audit.EventUserSkill,
		ActorType:  audit.ActorUser,
		ActorID:    actorId,
		TenantID:   tenantId,
		Action:     action,
		TargetType: "user_skill",
		TargetID:   strconv.FormatInt(targetId, 10),
		Detail:     detail,
		Result:     result,
	}
	if err := svc.audit.Record(ctx, event); err != nil {
		klog.CtxWarnf(ctx, "写入Skill审计日志失败: %v", err)
	}
}

// auditResult 把布尔成功标记转成审计结果码。
func auditResult(success bool) int16 {
	if success {
		return audit.ResultSuccess
	}
	return audit.ResultFailure
}

func toSkillDTO(s model.UserSkill) SkillDTO {
	return SkillDTO{
		SkillId:     s.ID,
		OwnerId:     s.OwnerID,
		Name:        s.Name,
		Description: s.Description,
		Version:     s.Version,
		Status:      s.Status,
		KbId:        s.KbID,
		SizeBytes:   s.SizeBytes,
		FileCount:   s.FileCount,
		CreatedAt:   s.CreatedAt.UnixMilli(),
		UpdatedAt:   s.UpdatedAt.UnixMilli(),
	}
}

// defaultSkillName 在没有 frontmatter 时用包名兜底命名。
func defaultSkillName(fileName string) string {
	base := path.Base(strings.ReplaceAll(fileName, "\\", "/"))
	return strings.TrimSpace(strings.TrimSuffix(base, path.Ext(base)))
}

// readSkillManifest 从 SKILL.md 的 YAML frontmatter 中读取 name / description / version。
//
// 这里刻意不引入 YAML 依赖：frontmatter 只允许「key: value」单行标量，
// 与 Skill 包的约定一致，解析失败时按空值回落，不影响上传。
func readSkillManifest(files []skillFile) (name, description, version string) {
	for _, f := range files {
		if !strings.EqualFold(path.Base(f.RelPath), skillManifestName) {
			continue
		}
		meta := parseSkillFrontMatter(string(f.Content))
		return meta["name"], meta["description"], meta["version"]
	}
	return "", "", ""
}

// parseSkillFrontMatter 解析 markdown 文件头部的 frontmatter 块。
func parseSkillFrontMatter(content string) map[string]string {
	meta := make(map[string]string)
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---") {
		return meta
	}
	rest := normalized[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		value = strings.Trim(value, `"'`)
		if value == "" {
			continue
		}
		meta[key] = value
	}
	return meta
}

// parseSkillPackage 把上传的字节流展开为资料文件列表。
//
// 支持两种输入：zip 包（含 SKILL.md 与 references/*.md）与单个 markdown 文件。
// 逐文件做扩展名、体积与内容安全校验；被跳过的文件以 warning 形式回传，不阻断整体上传。
func parseSkillPackage(fileName string, data []byte) ([]skillFile, []string, error) {
	lower := strings.ToLower(strings.ReplaceAll(fileName, "\\", "/"))
	base := path.Base(lower)
	switch {
	case strings.HasSuffix(base, ".zip"):
		return parseSkillZip(data)
	case skillAllowedExt[path.Ext(base)]:
		if len(data) > skillMaxFileBytes {
			return nil, nil, fmt.Errorf("单个资料文件不能超过 %d MB", skillMaxFileBytes>>20)
		}
		if _, err := filterUserPrompt(string(data)); err != nil {
			return nil, nil, fmt.Errorf("Skill 内容包含不允许的内容: %w", err)
		}
		// 单文件上传保留原文件名，若本身就叫 SKILL.md 则直接作为清单文件解析。
		return []skillFile{{RelPath: path.Base(fileName), Content: data}}, nil, nil
	default:
		return nil, nil, fmt.Errorf("Skill 只支持 zip 包或 markdown 文件")
	}
}

// parseSkillZip 解析 zip 包，做路径穿越、体积与内容安全校验。
func parseSkillZip(data []byte) ([]skillFile, []string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("Skill 包不是合法的 zip: %w", err)
	}
	var (
		files    []skillFile
		warnings []string
		total    int64
	)
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		relPath, ok := normalizeSkillRelPath(entry.Name)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("%s: 非法路径已跳过", entry.Name))
			continue
		}
		if isSkillJunk(relPath) {
			continue
		}
		ext := strings.ToLower(path.Ext(relPath))
		if !skillAllowedExt[ext] {
			warnings = append(warnings, fmt.Sprintf("%s: 仅支持 md/markdown/txt 资料文件", relPath))
			continue
		}
		if len(files) >= skillMaxFiles {
			return nil, nil, fmt.Errorf("Skill 包最多包含 %d 个资料文件", skillMaxFiles)
		}
		content, readErr := readZipEntry(entry, skillMaxFileBytes)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", relPath, readErr))
			continue
		}
		total += int64(len(content))
		if total > skillMaxExpandBytes {
			return nil, nil, fmt.Errorf("Skill 包解压后不能超过 %d MB", skillMaxExpandBytes>>20)
		}
		if _, scanErr := filterUserPrompt(string(content)); scanErr != nil {
			return nil, nil, fmt.Errorf("文件[%s]包含不允许的内容: %w", relPath, scanErr)
		}
		files = append(files, skillFile{RelPath: relPath, Content: content})
	}
	files = stripCommonSkillPrefix(files)
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, warnings, nil
}

// readZipEntry 读取单个 zip 条目，超限即拒绝，防止解压炸弹。
func readZipEntry(entry *zip.File, limit int64) ([]byte, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}
	defer func() { _ = rc.Close() }()
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, io.LimitReader(rc, limit+1)); err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}
	if int64(buf.Len()) > limit {
		return nil, fmt.Errorf("文件超过 %d MB", limit>>20)
	}
	return buf.Bytes(), nil
}

// normalizeSkillRelPath 归一化包内路径，拒绝绝对路径与上级目录穿越。
func normalizeSkillRelPath(name string) (string, bool) {
	cleaned := strings.ReplaceAll(name, "\\", "/")
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "" || strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, ":") {
		return "", false
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." {
			return "", false
		}
	}
	return path.Clean(cleaned), true
}

// isSkillJunk 过滤打包工具生成的隐藏文件。
func isSkillJunk(relPath string) bool {
	base := path.Base(relPath)
	if strings.HasPrefix(relPath, "__MACOSX/") {
		return true
	}
	switch base {
	case ".DS_Store", "Thumbs.db", ".gitignore":
		return true
	}
	return strings.HasPrefix(base, "._")
}

// stripCommonSkillPrefix 去掉「单层套目录」打包前缀，例如 demo-skill/SKILL.md → SKILL.md。
func stripCommonSkillPrefix(files []skillFile) []skillFile {
	if len(files) == 0 {
		return files
	}
	prefix := ""
	for _, f := range files {
		idx := strings.Index(f.RelPath, "/")
		if idx <= 0 {
			return files
		}
		head := f.RelPath[:idx]
		if prefix == "" {
			prefix = head
			continue
		}
		if prefix != head {
			return files
		}
	}
	if prefix == "" {
		return files
	}
	stripped := make([]skillFile, 0, len(files))
	for _, f := range files {
		stripped = append(stripped, skillFile{RelPath: strings.TrimPrefix(f.RelPath, prefix+"/"), Content: f.Content})
	}
	return stripped
}
