package ratelimit

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel/metric"

	"golang.org/x/time/rate"
)

type Config struct {
	Rate  rate.Limit
	Burst int
}

type userBucket struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

type RateLimiter struct {
	configs map[string]Config
	buckets sync.Map
	done    chan struct{}
}

func NewRateLimiter() *RateLimiter {
	cfg := map[string]Config{
		"translate":       {Rate: rate.Every(3 * time.Second), Burst: 3},
		"suggest_replies": {Rate: rate.Every(10 * time.Second), Burst: 2},
		"summarize":       {Rate: rate.Every(30 * time.Second), Burst: 1},
	}
	return &RateLimiter{
		configs: cfg,
		done:    make(chan struct{}),
	}
}

func bucketKey(userID int64, endpointKey string) string {
	return fmt.Sprintf("%d:%s", userID, endpointKey)
}

func (rl *RateLimiter) Allow(userID int64, endpointKey string) bool {
	cfg, ok := rl.configs[endpointKey]
	if !ok {
		return true
	}
	key := bucketKey(userID, endpointKey)
	val, _ := rl.buckets.LoadOrStore(key, &userBucket{
		limiter:    rate.NewLimiter(cfg.Rate, cfg.Burst),
		lastAccess: time.Now(),
	})
	b := val.(*userBucket)
	b.lastAccess = time.Now()
	return b.limiter.Allow()
}

func (rl *RateLimiter) WaitDuration(userID int64, endpointKey string) time.Duration {
	_, ok := rl.configs[endpointKey]
	if !ok {
		return 0
	}
	key := bucketKey(userID, endpointKey)
	val, ok := rl.buckets.Load(key)
	if !ok {
		return 0
	}
	b := val.(*userBucket)
	r := b.limiter.Reserve()
	if !r.OK() {
		return 0
	}
	d := r.Delay()
	r.Cancel()
	return d
}

func (rl *RateLimiter) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup(interval * 3)
			case <-rl.done:
				return
			}
		}
	}()
}

func (rl *RateLimiter) cleanup(maxAge time.Duration) {
	now := time.Now()
	rl.buckets.Range(func(key, value interface{}) bool {
		b := value.(*userBucket)
		if now.Sub(b.lastAccess) > maxAge {
			rl.buckets.Delete(key)
		}
		return true
	})
}

func (rl *RateLimiter) Stop() {
	close(rl.done)
}

var Default = NewRateLimiter()

func CheckRateLimit(c *app.RequestContext, userID int64, endpointKey string, rl *RateLimiter) bool {
	if rl.Allow(userID, endpointKey) {
		return true
	}
	if meter.M != nil && meter.M.RateLimitHitTotal != nil {
		meter.M.RateLimitHitTotal.Add(context.Background(), 1, metric.WithAttributes())
	}
	retryAfter := int(math.Ceil(rl.WaitDuration(userID, endpointKey).Seconds()))
	if retryAfter < 1 {
		retryAfter = 1
	}
	response.Error(c, "请求过于频繁", fmt.Sprintf("请%d秒后再试", retryAfter))
	return false
}
