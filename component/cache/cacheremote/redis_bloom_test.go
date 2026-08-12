package cacheremote

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type scriptedCommandExecutor struct {
	mu       sync.Mutex
	commands [][]interface{}
	results  []*redis.Cmd
	onDo     func(context.Context, []interface{})
}

func (e *scriptedCommandExecutor) Do(ctx context.Context, args ...interface{}) *redis.Cmd {
	e.mu.Lock()
	e.commands = append(e.commands, append([]interface{}(nil), args...))
	var result *redis.Cmd
	if len(e.results) > 0 {
		result = e.results[0]
		e.results = e.results[1:]
	} else {
		result = redis.NewCmdResult(int64(1), nil)
	}
	onDo := e.onDo
	e.mu.Unlock()
	if onDo != nil {
		onDo(ctx, args)
	}
	return result
}

func (e *scriptedCommandExecutor) recorded() [][]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([][]interface{}(nil), e.commands...)
}

func bloomConfig() RedisBloomConfig {
	return RedisBloomConfig{Key: "cache:keys", Capacity: 100, ErrorRate: 0.01, BatchSize: 2}
}

func TestRedisBloomPrepareRecordsReserveAndMapsUnavailable(t *testing.T) {
	t.Run("reserve", func(t *testing.T) {
		executor := &scriptedCommandExecutor{}
		filter, err := NewRedisBloomFilter(executor, bloomConfig())
		require.NoError(t, err)
		require.NoError(t, filter.Prepare(context.Background()))
		require.Equal(t, [][]interface{}{{"BF.RESERVE", "cache:keys", 0.01, uint(100)}}, executor.recorded())
	})

	t.Run("unknown command", func(t *testing.T) {
		executor := &scriptedCommandExecutor{results: []*redis.Cmd{
			redis.NewCmdResult(nil, errors.New("ERR unknown command 'BF.RESERVE'")),
		}}
		filter, err := NewRedisBloomFilter(executor, bloomConfig())
		require.NoError(t, err)
		require.ErrorIs(t, filter.Prepare(context.Background()), ErrRedisBloomUnavailable)
	})
}

func TestRedisBloomPreloadLookupAddAndReadiness(t *testing.T) {
	executor := &scriptedCommandExecutor{results: []*redis.Cmd{
		redis.NewCmdResult("OK", nil),
		redis.NewCmdResult(int64(1), nil),
		redis.NewCmdResult([]interface{}{int64(1), int64(1)}, nil),
		redis.NewCmdResult([]interface{}{int64(1)}, nil),
		redis.NewCmdResult(int64(0), nil),
		redis.NewCmdResult(int64(1), nil),
	}}
	filter, err := NewRedisBloomFilter(executor, bloomConfig())
	require.NoError(t, err)
	require.NoError(t, filter.Prepare(context.Background()))
	require.False(t, filter.Ready())

	mayContain, err := filter.MayContain(context.Background(), "before-preload")
	require.NoError(t, err)
	require.True(t, mayContain, "an unready filter must not reject a key")
	require.NoError(t, filter.Add(context.Background(), "added-before-preload"))
	require.False(t, filter.Ready(), "Add must never make the filter authoritative")

	require.NoError(t, filter.Preload(context.Background(), []string{"a", "b", "c"}))
	require.True(t, filter.Ready())
	mayContain, err = filter.MayContain(context.Background(), "missing")
	require.NoError(t, err)
	require.False(t, mayContain)
	require.NoError(t, filter.Add(context.Background(), "added-after-preload"))
	require.True(t, filter.Ready())

	require.True(t, reflect.DeepEqual([][]interface{}{
		{"BF.RESERVE", "cache:keys", 0.01, uint(100)},
		{"BF.ADD", "cache:keys", "added-before-preload"},
		{"BF.MADD", "cache:keys", "a", "b"},
		{"BF.MADD", "cache:keys", "c"},
		{"BF.EXISTS", "cache:keys", "missing"},
		{"BF.ADD", "cache:keys", "added-after-preload"},
	}, executor.recorded()))
}

func TestRedisBloomPreloadFailureAndCancellationLeaveFilterUnready(t *testing.T) {
	t.Run("partial failure", func(t *testing.T) {
		executor := &scriptedCommandExecutor{results: []*redis.Cmd{
			redis.NewCmdResult("OK", nil),
			redis.NewCmdResult([]interface{}{int64(1), int64(1)}, nil),
			redis.NewCmdResult(nil, errors.New("write failed")),
		}}
		filter, err := NewRedisBloomFilter(executor, bloomConfig())
		require.NoError(t, err)
		require.NoError(t, filter.Prepare(context.Background()))
		require.Error(t, filter.Preload(context.Background(), []string{"a", "b", "c"}))
		require.False(t, filter.Ready())
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		executor := &scriptedCommandExecutor{results: []*redis.Cmd{
			redis.NewCmdResult("OK", nil),
			redis.NewCmdResult([]interface{}{int64(1), int64(1)}, nil),
		}, onDo: func(_ context.Context, args []interface{}) {
			if args[0] == "BF.MADD" {
				cancel()
			}
		}}
		filter, err := NewRedisBloomFilter(executor, bloomConfig())
		require.NoError(t, err)
		require.NoError(t, filter.Prepare(context.Background()))
		require.ErrorIs(t, filter.Preload(ctx, []string{"a", "b", "c"}), context.Canceled)
		require.False(t, filter.Ready())
		require.Len(t, executor.recorded(), 2, "cancellation must stop before the next batch")
	})
}

func TestRedisBloomConfigAndBusyKeyPolicy(t *testing.T) {
	for _, config := range []RedisBloomConfig{
		{Capacity: 1, ErrorRate: 0.1},
		{Key: "cache:keys", ErrorRate: 0.1},
		{Key: "cache:keys", Capacity: 1, ErrorRate: 0},
		{Key: "cache:keys", Capacity: 1, ErrorRate: 1},
		{Key: "cache:keys", Capacity: 1, ErrorRate: 0.1, BatchSize: -1},
	} {
		_, err := NewRedisBloomFilter(&scriptedCommandExecutor{}, config)
		require.Error(t, err)
	}

	busy := errors.New("BUSYKEY Key already exists")
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "allowed"}[allowed], func(t *testing.T) {
			config := bloomConfig()
			config.AllowExisting = allowed
			filter, err := NewRedisBloomFilter(&scriptedCommandExecutor{results: []*redis.Cmd{redis.NewCmdResult(nil, busy)}}, config)
			require.NoError(t, err)
			if allowed {
				require.NoError(t, filter.Prepare(context.Background()))
			} else {
				require.ErrorIs(t, filter.Prepare(context.Background()), busy)
			}
		})
	}
}
