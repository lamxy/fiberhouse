package cache

import (
	"context"
	"testing"
	"time"
)

// legacyCache deliberately implements only the long-standing Cache contract.
// Adding lifecycle methods to Cache would break existing third-party backends.
type legacyCache struct{}

func (legacyCache) Get(context.Context, string, *CacheOption) (string, error) {
	return "", ErrKeyNotFound
}
func (legacyCache) Set(context.Context, string, interface{}, *CacheOption) error {
	return nil
}
func (legacyCache) Delete(context.Context, ...string) error { return nil }
func (legacyCache) Close() error                            { return nil }
func (legacyCache) Wait() error                             { return nil }
func (legacyCache) GetLevel() Level                         { return Local }

var _ Cache = legacyCache{}

func TestCompatibilityLegacyOptionChainAndGetCached(t *testing.T) {
	ctx := context.Background()
	option := NewCacheOption(newCacheOptionTestContext()).
		Local().
		EnableCache().
		SetCacheKey("compatibility:key").
		SetContextCtx(ctx).
		SetLocalTTL(time.Minute).
		SetRemoteTTLWithRandom(5*time.Minute, time.Minute).
		SetSyncStrategyWriteRemoteOnly().
		EnableProtectionAll().
		DisableCache()

	got, err := GetCached(option, func(gotCtx context.Context) (string, error) {
		if gotCtx != ctx {
			t.Fatalf("loader context = %v, want caller context", gotCtx)
		}
		return "legacy-result", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "legacy-result" {
		t.Fatalf("GetCached() = %q, want legacy-result", got)
	}
}

func TestCompatibilityLevelAndStrategyValues(t *testing.T) {
	if Local != 1 || Remote != 2 || Level2 != 3 {
		t.Fatalf("Level values = (%d, %d, %d), want (1, 2, 3)", Local, Remote, Level2)
	}
	if WriteBoth != 1 || WriteRemoteOnly != 2 || AsyncWriteBoth != 3 || AsyncWriteRemoteOnly != 4 {
		t.Fatalf("Strategy values = (%d, %d, %d, %d), want (1, 2, 3, 4)",
			WriteBoth, WriteRemoteOnly, AsyncWriteBoth, AsyncWriteRemoteOnly)
	}
}
