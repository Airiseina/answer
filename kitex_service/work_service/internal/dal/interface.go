package dal

import (
	"github.com/Airiseina/answer/kitex_service/work_service/internal/model"

	"gorm.io/gorm"
)

type summaryDao struct {
	db *gorm.DB
}

func NewSummaryDao(db *gorm.DB) SummaryDao {
	return &summaryDao{db}
}

type SummaryDao interface {
	CreateReport(report *model.SummaryReport) error
	UpdateReport(id int64, updates map[string]interface{}) error
	GetReport(userID, id int64) (model.SummaryReport, error)
	// GetReportByPeriod 按 (user_id, type, period_start) 查已有报告，供幂等复用。
	GetReportByPeriod(userID int64, reportType string, periodStart int64) (model.SummaryReport, error)
	ListReports(userID int64, reportType string, offset, limit int) ([]model.SummaryReport, error)
	GetSchedule(userID int64) (model.SummarySchedule, error)
	SaveSchedule(schedule *model.SummarySchedule) error
}
