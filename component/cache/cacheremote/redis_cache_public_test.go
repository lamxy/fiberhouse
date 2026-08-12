package cacheremote_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/component/cache/cacheremote"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/sony/gobreaker/v2"
)

type publicRedisHook struct {
	err error
}

func (h publicRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h publicRedisHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(context.Context, redis.Cmder) error { return h.err }
}

func (h publicRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestRedisDb_PublicBreakerOptionClassifiesGetFailures(t *testing.T) {
	infraErr := errors.New("redis transport unavailable")
	tests := []struct {
		name     string
		hookErr  error
		cancel   bool
		wantMiss bool
		wantOpen bool
	}{
		{name: "infrastructure error", hookErr: infraErr, wantOpen: true},
		{name: "cache miss", hookErr: redis.Nil, wantMiss: true},
		{name: "caller cancellation", hookErr: context.Canceled, cancel: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
				"test.redis.host": "scripted.invalid", "test.redis.port": "6379",
			})
			logger := zerolog.Nop()
			appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
			breaker := cache.NewCircuitBreakerWrap("public-redis", &gobreaker.Settings{
				ReadyToTrip:   func(counts gobreaker.Counts) bool { return counts.TotalFailures >= 1 },
				OnStateChange: func(string, gobreaker.State, gobreaker.State) {},
			})
			rd, err := cacheremote.NewRedisDbWithOptions(
				appCtx,
				cacheremote.WithRedisConfigPath("test.redis"),
				cacheremote.WithRedisCircuitBreaker(breaker),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = rd.Close() })
			rd.GetRedisClient().AddHook(publicRedisHook{err: tc.hookErr})

			ctx := context.Background()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			_, err = rd.Get(ctx, "key", cache.NewCacheOption(appCtx).EnableCircuitBreaker())
			if tc.wantMiss {
				if !cache.IsMiss(err) {
					t.Fatalf("Get() error = %v, want canonical miss", err)
				}
			} else if !errors.Is(err, tc.hookErr) {
				t.Fatalf("Get() error = %v, want %v", err, tc.hookErr)
			}
			if gotOpen := !breaker.Allow(); gotOpen != tc.wantOpen {
				t.Fatalf("breaker open = %v, want %v", gotOpen, tc.wantOpen)
			}
		})
	}
}
