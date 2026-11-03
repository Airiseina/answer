package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/redis/go-redis/v9"
)

// summaryTimezone 总结调度时区：优先读配置，缺省东八区（与 model.DefaultSummaryTimezone 一致）。
const summaryRunLockPrefix = "summary:run"

// SummaryScheduler 负责在每天/每周固定时刻为所有用户生成日/周总结。
//
// 设计要点：
//  1. 单实例内用 running 标志避免上一轮未结束又启动新一轮；
//  2. 多实例间用 Garnet/Redis 的 run 锁保证同一 (类型, 日期) 只有一个实例在跑；
//  3. 用 user_id 取模把用户摊到错峰窗口内，避免整点同时打满 LLM；
//  4. GenerateSummary 自身按 (user, type, period_start) 幂等，重复触发只会复用报告。
type SummaryScheduler struct {
	svc          *service.WorkService
	rdb          *redis.Client
	tickInterval time.Duration
	sendHour     int
	stagger      time.Duration
	pageSize     int32
	lockTTL      time.Duration
	concurrency  int
	running      int32
}

// NewSummaryScheduler 按配置构造调度器。
func NewSummaryScheduler(svc *service.WorkService, rdb *redis.Client) *SummaryScheduler {
	tick := time.Duration(config.V.GetInt("summary.tick_interval")) * time.Second
	if tick <= 0 {
		tick = time.Minute
	}
	sendHour := config.V.GetInt("summary.send_hour")
	if sendHour < 0 || sendHour > 23 {
		sendHour = 21
	}
	staggerMinutes := config.V.GetInt("summary.stagger_minutes")
	if staggerMinutes <= 0 {
		staggerMinutes = 30
	}
	pageSize := config.V.GetInt("summary.page_size")
	if pageSize <= 0 {
		pageSize = 500
	}
	return &SummaryScheduler{
		svc:          svc,
		rdb:          rdb,
		tickInterval: tick,
		sendHour:     sendHour,
		stagger:      time.Duration(staggerMinutes) * time.Minute,
		pageSize:     int32(pageSize),
		lockTTL:      10 * time.Minute,
		concurrency:  4,
	}
}

// Start 启动调度循环；随传入 ctx 取消而退出。
func (s *SummaryScheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *SummaryScheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()
	// 启动后先跑一次，覆盖进程在发送时刻之后才拉起的情况。
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			klog.Info("总结调度已停止")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick 每个扫描周期触发一次：判断是否到点、抢占 run 锁、再错峰分发用户。
func (s *SummaryScheduler) tick(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return // 上一轮尚未结束
	}
	defer atomic.StoreInt32(&s.running, 0)

	loc := summaryLocation()
	now := time.Now().In(loc)
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), s.sendHour, 0, 0, 0, loc)
	if now.Before(windowStart) {
		return // 今天还没到发送时刻
	}

	reportTypes := []string{model.SummaryTypeDaily}
	if now.Weekday() == time.Sunday {
		reportTypes = append(reportTypes, model.SummaryTypeWeekly)
	}
	for _, reportType := range reportTypes {
		s.runType(ctx, reportType, now, windowStart)
	}
}

// runType 处理单一类型（daily/weekly）：抢锁、枚举用户、错峰生成。
func (s *SummaryScheduler) runType(ctx context.Context, reportType string, now, windowStart time.Time) {
	lockKey := fmt.Sprintf("%s:%s:%s", summaryRunLockPrefix, reportType, windowStart.Format("2006-01-02"))
	if !s.acquireLock(ctx, lockKey) {
		return // 其他实例正在处理
	}
	defer s.releaseLock(ctx, lockKey)

	offsetMinutes := int64(s.stagger / time.Minute)
	if offsetMinutes <= 0 {
		offsetMinutes = 1
	}
	generated, skipped := 0, 0
	var afterUserID int64
	for {
		users, err := rpc.ListActiveUsers(ctx, afterUserID, s.pageSize)
		if err != nil {
			klog.CtxErrorf(ctx, "总结调度枚举用户失败: %v", err)
			return
		}
		if len(users) == 0 {
			break
		}
		var wg sync.WaitGroup
		sem := make(chan struct{}, s.concurrency)
		for _, user := range users {
			afterUserID = user.UserID
			// 错峰：按 user_id 取模决定该用户在本窗口内的分钟偏移。
			due := windowStart.Add(time.Duration(user.UserID%offsetMinutes) * time.Minute)
			if now.Before(due) {
				skipped++
				continue
			}
			if !s.svc.SummaryEnabled(ctx, user.UserID, reportType) {
				skipped++
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(userID int64) {
				defer wg.Done()
				defer func() { <-sem }()
				if _, genErr := s.svc.GenerateSummary(ctx, userID, reportType, 0, 0); genErr != nil {
					klog.CtxWarnf(ctx, "生成[%s]总结失败 user=%d: %v", reportType, userID, genErr)
					return
				}
				generated++
			}(user.UserID)
		}
		wg.Wait()
		if len(users) < int(s.pageSize) {
			break
		}
	}
	klog.CtxInfof(ctx, "[%s]总结调度完成: 生成/复用=%d, 跳过=%d", reportType, generated, skipped)
}

// acquireLock 抢占 run 锁；rdb 不可用时退化为「单实例直接执行」，由幂等保证正确性。
func (s *SummaryScheduler) acquireLock(ctx context.Context, key string) bool {
	if s.rdb == nil {
		return true
	}
	ok, err := s.rdb.SetNX(ctx, key, 1, s.lockTTL).Result()
	if err != nil {
		klog.CtxWarnf(ctx, "总结调度抢锁失败，本轮按单实例执行: %v", err)
		return true
	}
	return ok
}

func (s *SummaryScheduler) releaseLock(ctx context.Context, key string) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		klog.CtxWarnf(ctx, "总结调度释放锁失败: %v", err)
	}
}

// summaryLocation 返回调度时区；容器缺少 tzdata 时退回固定东八区。
func summaryLocation() *time.Location {
	if loc, err := time.LoadLocation(model.DefaultSummaryTimezone); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*60*60)
}
