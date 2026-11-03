package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/model"

	"gorm.io/gorm"
)

func (d *summaryDao) CreateReport(report *model.SummaryReport) error {
	if err := d.db.Create(report).Error; err != nil {
		return fmt.Errorf("创建总结报告失败: %w", err)
	}
	return nil
}

func (d *summaryDao) UpdateReport(id int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	if err := d.db.Model(&model.SummaryReport{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新总结报告失败: %w", err)
	}
	return nil
}

// GetReport 按 ID 读取总结报告，强制限定 user_id：
// 总结仅本人可见，即便拿到别人的 report_id 也不能越权读取。
func (d *summaryDao) GetReport(userID, id int64) (model.SummaryReport, error) {
	var report model.SummaryReport
	err := d.db.Where("id = ? AND user_id = ?", id, userID).First(&report).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SummaryReport{}, nil
		}
		return model.SummaryReport{}, fmt.Errorf("查询总结报告失败: %w", err)
	}
	return report, nil
}

func (d *summaryDao) ListReports(userID int64, reportType string, offset, limit int) ([]model.SummaryReport, error) {
	query := d.db.Where("user_id = ?", userID)
	if reportType != "" {
		query = query.Where("type = ?", reportType)
	}
	var reports []model.SummaryReport
	if err := query.Order("period_start DESC, id DESC").Offset(offset).Limit(limit).Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("查询总结列表失败: %w", err)
	}
	return reports, nil
}

// GetReportByPeriod 按周期查询报告，用于调度重试与手动触发的幂等复用。
func (d *summaryDao) GetReportByPeriod(userID int64, reportType string, periodStart int64) (model.SummaryReport, error) {
	var report model.SummaryReport
	err := d.db.Where("user_id = ? AND type = ? AND period_start = ?", userID, reportType, periodStart).First(&report).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SummaryReport{}, nil
		}
		return model.SummaryReport{}, fmt.Errorf("按周期查询总结报告失败: %w", err)
	}
	return report, nil
}

func (d *summaryDao) GetSchedule(userID int64) (model.SummarySchedule, error) {
	var schedule model.SummarySchedule
	err := d.db.Where("user_id = ?", userID).First(&schedule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SummarySchedule{}, nil
		}
		return model.SummarySchedule{}, fmt.Errorf("查询总结订阅失败: %w", err)
	}
	return schedule, nil
}

// SaveSchedule 以 user_id 为幂等键写入订阅开关：不存在则创建，存在则更新。
func (d *summaryDao) SaveSchedule(schedule *model.SummarySchedule) error {
	var existing model.SummarySchedule
	err := d.db.Where("user_id = ?", schedule.UserID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("查询总结订阅失败: %w", err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if createErr := d.db.Create(schedule).Error; createErr != nil {
			return fmt.Errorf("创建总结订阅失败: %w", createErr)
		}
		return nil
	}
	updates := map[string]interface{}{
		"daily_enabled":  schedule.DailyEnabled,
		"weekly_enabled": schedule.WeeklyEnabled,
		"timezone":       schedule.Timezone,
	}
	if updateErr := d.db.Model(&model.SummarySchedule{}).Where("id = ?", existing.ID).Updates(updates).Error; updateErr != nil {
		return fmt.Errorf("更新总结订阅失败: %w", updateErr)
	}
	schedule.ID = existing.ID
	return nil
}
