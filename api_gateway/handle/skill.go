package handle

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// 个人 Skill：只支持「资料型」zip 包或 markdown 文件，上传后由 bot_service 解析入库。
const maxSkillUploadSize = 10 << 20

type userSkillItem struct {
	SkillId     string `json:"skill_id"`
	OwnerId     string `json:"owner_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Status      int16  `json:"status"`
	KbId        string `json:"kb_id"`
	SizeBytes   int64  `json:"size_bytes"`
	FileCount   int32  `json:"file_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type userSkillFileItem struct {
	FileId    string `json:"file_id"`
	SkillId   string `json:"skill_id"`
	RelPath   string `json:"rel_path"`
	ObjectUrl string `json:"object_url"`
	SizeBytes int64  `json:"size_bytes"`
}

type botSkillItem struct {
	SkillId     string `json:"skill_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
}

func convertUserSkillInfo(info *bot.UserSkillInfo) userSkillItem {
	return userSkillItem{
		SkillId:     fmt.Sprintf("%d", info.SkillId),
		OwnerId:     fmt.Sprintf("%d", info.OwnerId),
		Name:        info.Name,
		Description: info.Description,
		Version:     info.Version,
		Status:      info.Status,
		KbId:        fmt.Sprintf("%d", info.KbId),
		SizeBytes:   info.SizeBytes,
		FileCount:   info.FileCount,
		CreatedAt:   fmt.Sprintf("%d", info.CreatedAt),
		UpdatedAt:   fmt.Sprintf("%d", info.UpdatedAt),
	}
}

// UploadUserSkill 上传个人 Skill 包（zip 或多文件拼装目录），解析后落库并建专属知识库。
func UploadUserSkill(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, "参数错误", "请选择要上传的 Skill 包")
		return
	}
	if file.Size > maxSkillUploadSize {
		response.Error(c, "文件过大", "Skill 包不能超过 10MB")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(io.LimitReader(src, maxSkillUploadSize+1))
	if err != nil {
		response.Error(c, "上传失败", "读取文件失败")
		return
	}
	if len(data) > maxSkillUploadSize {
		response.Error(c, "文件过大", "Skill 包不能超过 10MB")
		return
	}
	resp, err := rpc.UploadUserSkill(ctx, &bot.UploadUserSkillReq{
		TenantId: identity.TenantId,
		OwnerId:  identity.Id,
		FileName: file.Filename,
		Data:     data,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "上传个人Skill失败: %v", err)
		response.Error(c, "上传Skill失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{
		"skill_id":   fmt.Sprintf("%d", resp.SkillId),
		"file_count": resp.FileCount,
		"size_bytes": resp.SizeBytes,
		"warnings":   resp.Warnings,
	})
}

// ListUserSkills 返回当前用户的个人 Skill 列表。
func ListUserSkills(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	resp, err := rpc.ListUserSkills(ctx, &bot.ListUserSkillsReq{
		TenantId: identity.TenantId,
		OwnerId:  identity.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询个人Skill列表失败: %v", err)
		response.Error(c, "查询Skill列表失败", err.Error())
		return
	}
	items := make([]userSkillItem, 0, len(resp.Skills))
	for _, s := range resp.Skills {
		items = append(items, convertUserSkillInfo(s))
	}
	response.Success(c, map[string]interface{}{"skills": items})
}

// GetUserSkill 读取个人 Skill 详情与资料文件列表。
func GetUserSkill(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	skillID, err := strconv.ParseInt(c.Query("skill_id"), 10, 64)
	if err != nil || skillID <= 0 {
		response.Error(c, "参数错误", "skill_id无效")
		return
	}
	resp, err := rpc.GetUserSkill(ctx, &bot.GetUserSkillReq{
		TenantId: identity.TenantId,
		SkillId:  skillID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询个人Skill[%d]失败: %v", skillID, err)
		response.Error(c, "查询Skill失败", err.Error())
		return
	}
	if !resp.Success || resp.Skill == nil {
		response.Error(c, "Skill不存在", nil)
		return
	}
	// 读取接口只按「租户 + ID」查询，这里再校验归属，避免同租户内互相窥探他人 Skill。
	if resp.Skill.OwnerId != identity.Id {
		response.Error(c, "Skill不存在", nil)
		return
	}
	files := make([]userSkillFileItem, 0, len(resp.Files))
	for _, f := range resp.Files {
		files = append(files, userSkillFileItem{
			FileId:    fmt.Sprintf("%d", f.FileId),
			SkillId:   fmt.Sprintf("%d", f.SkillId),
			RelPath:   f.RelPath,
			ObjectUrl: f.ObjectUrl,
			SizeBytes: f.SizeBytes,
		})
	}
	response.Success(c, map[string]interface{}{
		"skill": convertUserSkillInfo(resp.Skill),
		"files": files,
	})
}

type deleteUserSkillReq struct {
	SkillId int64 `json:"skill_id,string" vd:"$ > 0"`
}

// DeleteUserSkill 删除个人 Skill，连同其专属知识库与所有 Bot 绑定一并回收。
func DeleteUserSkill(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req deleteUserSkillReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.DeleteUserSkill(ctx, &bot.DeleteUserSkillReq{
		TenantId:   identity.TenantId,
		SkillId:    req.SkillId,
		OperatorId: identity.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除个人Skill[%d]失败: %v", req.SkillId, err)
		response.Error(c, "删除Skill失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type bindUserSkillReq struct {
	BotId   int64 `json:"bot_id,string" vd:"$ > 0"`
	SkillId int64 `json:"skill_id,string" vd:"$ > 0"`
	Enabled bool  `json:"enabled"`
}

// BindUserSkill 把个人 Skill 绑定到 / 解绑自指定 Bot，绑定后并入知识库检索链路。
func BindUserSkill(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req bindUserSkillReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.BindUserSkill(ctx, &bot.BindUserSkillReq{
		TenantId: identity.TenantId,
		OwnerId:  identity.Id,
		BotId:    req.BotId,
		SkillId:  req.SkillId,
		Enabled:  req.Enabled,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "绑定个人Skill[%d]到Bot[%d]失败: %v", req.SkillId, req.BotId, err)
		response.Error(c, "Skill绑定失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

// ListBotSkills 返回某个 Bot 已绑定的个人 Skill，仅 Bot 创建者可读。
func ListBotSkills(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	botID, err := strconv.ParseInt(c.Query("bot_id"), 10, 64)
	if err != nil || botID <= 0 {
		response.Error(c, "参数错误", "bot_id无效")
		return
	}
	resp, err := rpc.ListBotSkills(ctx, &bot.ListBotSkillsReq{
		TenantId:   identity.TenantId,
		BotId:      botID,
		OperatorId: identity.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询Bot[%d]已绑定Skill失败: %v", botID, err)
		response.Error(c, "查询Bot Skill失败", err.Error())
		return
	}
	items := make([]botSkillItem, 0, len(resp.Skills))
	for _, s := range resp.Skills {
		items = append(items, botSkillItem{
			SkillId:     fmt.Sprintf("%d", s.SkillId),
			Name:        s.Name,
			Description: s.Description,
			Version:     s.Version,
			Enabled:     s.Enabled,
		})
	}
	response.Success(c, map[string]interface{}{"skills": items})
}
