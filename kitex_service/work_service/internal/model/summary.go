package model

import "time"

// 总结类型。
const (
	SummaryTypeDaily  = "daily"  // 日总结
	SummaryTypeWeekly = "weekly" // 周总结
)

// 总结生成状态。
const (
	SummaryStatusPending   int16 = 0 // 待生成
	SummaryStatusRunning   int16 = 1 // 生成中
	SummaryStatusCompleted int16 = 2 // 完成
	SummaryStatusFailed    int16 = 3 // 失败
)

// DefaultSummaryTimezone 总结调度的默认时区，评审确认固定为东八区。
const DefaultSummaryTimezone = "Asia/Shanghai"

// SummaryReport 是系统 Bot 生成的一份日/周总结报告。
//
// 说明：总结聚合用户「全部会话」，因此不按单个会话存储；
// PeriodStart/PeriodEnd 记录统计窗口，便于按日期/周次回溯查询。
// (user_id, type, period_start) 唯一，保证调度重试时不会生成重复报告。
type SummaryReport struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID    int64     `gorm:"not null;default:0;index" json:"tenant_id"` // 用户个人租户
	UserID      int64     `gorm:"not null;index;uniqueIndex:uk_summary_user_period" json:"user_id"`
	Type        string    `gorm:"type:varchar(16);not null;index;uniqueIndex:uk_summary_user_period" json:"type"` // daily / weekly
	PeriodStart int64     `gorm:"not null;default:0;uniqueIndex:uk_summary_user_period" json:"period_start"`
	PeriodEnd   int64     `gorm:"not null;default:0" json:"period_end"`
	Content     string    `gorm:"type:text" json:"content"`
	Model       string    `gorm:"type:varchar(100);default:''" json:"model"`
	Status      int16     `gorm:"not null;default:0;index" json:"status"`
	Error       string    `gorm:"type:varchar(255);default:''" json:"error"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (SummaryReport) TableName() string {
	return "summary_report"
}

// SummarySchedule 用户的总结订阅设置。
//
// 缺省即视为「日/周总结均开启」，由服务层在读取时补默认值，
// 因此这里只在用户显式修改过之后才落库。
type SummarySchedule struct {
	ID            int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	UserID        int64     `gorm:"not null;uniqueIndex" json:"user_id"`
	DailyEnabled  bool      `gorm:"not null;default:true" json:"daily_enabled"`
	WeeklyEnabled bool      `gorm:"not null;default:true" json:"weekly_enabled"`
	Timezone      string    `gorm:"type:varchar(64);not null;default:'Asia/Shanghai'" json:"timezone"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (SummarySchedule) TableName() string {
	return "summary_schedule"
}
