package model

import "time"

// 个人 Skill 状态机：0=待解析 1=可用 2=解析失败 3=已停用。
//
// 本轮只支持「资料型 Skill」：内容全部落在该 Skill 独占的知识库里，
// 运行期仅作为检索资料被 Agent 读取，不执行任何脚本或工具。
// 其中 3=已停用 为保留状态，留给后续的治理/下架动作，当前上传流程只会写入 0/1/2。
const (
	UserSkillStatusPending int16 = 0
	UserSkillStatusReady   int16 = 1
	UserSkillStatusFailed  int16 = 2
	UserSkillStatusOffline int16 = 3
)

// ValidUserSkillStatus 判断状态取值是否在状态机范围内。
func ValidUserSkillStatus(status int16) bool {
	switch status {
	case UserSkillStatusPending, UserSkillStatusReady, UserSkillStatusFailed, UserSkillStatusOffline:
		return true
	default:
		return false
	}
}

// UserSkill 是用户上传的个人 Skill 包（目录/zip），一个 Skill 独占一个知识库。
type UserSkill struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID    int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	OwnerID     int64     `gorm:"not null;index" json:"owner_id"`
	Name        string    `gorm:"type:varchar(64);not null" json:"name"`
	Description string    `gorm:"type:varchar(512);default:''" json:"description"`
	Version     string    `gorm:"type:varchar(32);default:''" json:"version"`
	Status      int16     `gorm:"not null;default:0;index" json:"status"`
	KbID        int64     `gorm:"not null;default:0;index" json:"kb_id"`
	SizeBytes   int64     `gorm:"not null;default:0" json:"size_bytes"`
	FileCount   int32     `gorm:"not null;default:0" json:"file_count"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (UserSkill) TableName() string {
	return "user_skill"
}

// UserSkillFile 记录 Skill 包内的单个资料文件，供前端展示与溯源。
type UserSkillFile struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID  int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	SkillID   int64     `gorm:"not null;index" json:"skill_id"`
	RelPath   string    `gorm:"type:varchar(255);not null" json:"rel_path"`
	ObjectURL string    `gorm:"type:varchar(500);not null" json:"object_url"`
	Hash      string    `gorm:"type:varchar(64);default:''" json:"hash"`
	SizeBytes int64     `gorm:"not null;default:0" json:"size_bytes"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (UserSkillFile) TableName() string {
	return "user_skill_file"
}

// BotSkill 是 Bot 与 Skill 的绑定关系；enabled=false 表示保留绑定但运行期不参与检索。
type BotSkill struct {
	TenantID  int64     `gorm:"primaryKey" json:"tenant_id"`
	BotID     int64     `gorm:"primaryKey" json:"bot_id"`
	SkillID   int64     `gorm:"primaryKey" json:"skill_id"`
	OwnerID   int64     `gorm:"not null;default:0;index" json:"owner_id"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (BotSkill) TableName() string {
	return "bot_skill"
}
