// Package audit 提供审计日志的统一模型与写入入口。
//
// 各服务只需在自己的 AutoMigrate 中带上 audit.Log，并注入一个 Recorder，
// 即可记录开放平台调用、Bot 发布/审核、租户管理、参考单价维护、总结生成等关键动作。
package audit

import (
	"context"

	"gorm.io/gorm"
)

// 事件类型（与规划文档「审计日志」一节对齐）。
const (
	EventOpenAPIRequest  = "open_api_request" // 开放平台调用
	EventBotPublish      = "bot_publish"      // Bot 发布 / 审核
	EventTenantManage    = "tenant_manage"    // 租户管理
	EventModelPrice      = "model_price"      // 参考单价维护
	EventSummaryGenerate = "summary_generate" // 总结生成
	EventAdminManage     = "admin_manage"     // 管理后台账号维护
	EventUserSkill       = "user_skill"       // 个人 Skill 上传 / 绑定 / 删除
)

// 操作者类型。
const (
	ActorUser   = "user"
	ActorAdmin  = "admin"
	ActorApp    = "app"
	ActorSystem = "system"
)

// 动作结果。
const (
	ResultSuccess = int16(0)
	ResultFailure = int16(1)
)

// Log 是审计日志记录，表名固定为 audit_log。
type Log struct {
	ID         int64  `gorm:"primarykey" json:"id"`
	CreatedAt  int64  `gorm:"autoCreateTime;index" json:"created_at"`
	EventType  string `gorm:"type:varchar(64);index;not null" json:"event_type"`
	ActorType  string `gorm:"type:varchar(32);index" json:"actor_type"`
	ActorID    int64  `gorm:"index" json:"actor_id"`
	TenantID   int64  `gorm:"index" json:"tenant_id"`
	Action     string `gorm:"type:varchar(64);index" json:"action"`
	TargetType string `gorm:"type:varchar(64)" json:"target_type"`
	TargetID   string `gorm:"type:varchar(64);index" json:"target_id"`
	Detail     string `gorm:"type:text" json:"detail"`
	IP         string `gorm:"type:varchar(64)" json:"ip"`
	Result     int16  `gorm:"type:tinyint;default:0;not null" json:"result"`
}

// TableName 固定审计日志表名，避免 GORM 复数化差异。
func (Log) TableName() string {
	return "audit_log"
}

// Event 描述一次待写入的审计事件。
type Event struct {
	EventType  string
	ActorType  string
	ActorID    int64
	TenantID   int64
	Action     string
	TargetType string
	TargetID   string
	Detail     string
	IP         string
	Result     int16
}

// Recorder 抽象审计写入，便于在服务内部注入与测试替换。
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

type gormRecorder struct {
	db *gorm.DB
}

// NewRecorder 基于 GORM 创建审计写入器。
func NewRecorder(db *gorm.DB) Recorder {
	return &gormRecorder{db: db}
}

// Record 落库一条审计日志。
func (r *gormRecorder) Record(_ context.Context, event Event) error {
	return r.db.Create(&Log{
		EventType:  event.EventType,
		ActorType:  event.ActorType,
		ActorID:    event.ActorID,
		TenantID:   event.TenantID,
		Action:     event.Action,
		TargetType: event.TargetType,
		TargetID:   event.TargetID,
		Detail:     event.Detail,
		IP:         event.IP,
		Result:     event.Result,
	}).Error
}

// NoopRecorder 是不落库的实现，用于未接入审计的服务或测试。
type NoopRecorder struct{}

// Record 忽略所有事件。
func (NoopRecorder) Record(context.Context, Event) error {
	return nil
}
