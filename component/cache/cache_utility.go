// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/lamxy/fiberhouse"
	frameUtils "github.com/lamxy/fiberhouse/utils"
	"golang.org/x/sync/singleflight"
)

// Event identifies a cache-aside coordinator outcome that may be observed.
type Event uint8

const (
	EventCacheHit Event = iota + 1
	EventCacheMiss
	EventCacheError
	EventFillError
	EventFilterError
)

// FilterFailureMode determines the coordinator behavior when an existence
// filter cannot make a decision.
type FilterFailureMode uint8

const (
	FilterFailOpen FilterFailureMode = iota + 1
	FilterFailClosed
)

// Observer receives non-critical coordinator events. It must not block.
type Observer func(Event)

// CoordinatorOption configures a Coordinator.
type CoordinatorOption func(*Coordinator)

// WithObserver installs an optional observer. A nil observer is ignored.
func WithObserver(observer Observer) CoordinatorOption {
	return func(c *Coordinator) {
		if observer != nil {
			c.observer = observer
		}
	}
}

// WithExistenceFilter installs the authoritative key existence filter used by
// Bloom-enabled requests.
func WithExistenceFilter(filter ExistenceFilter) CoordinatorOption {
	return func(c *Coordinator) {
		if filter != nil {
			c.filter = filter
		}
	}
}

// WithFilterFailureMode configures whether filter errors allow or reject a
// request. The default remains fail open to preserve availability.
func WithFilterFailureMode(mode FilterFailureMode) CoordinatorOption {
	return func(c *Coordinator) {
		if mode == FilterFailClosed {
			c.filterFailureMode = mode
		}
	}
}

// Coordinator owns the singleflight group that protects an entire cache-aside
// operation, not merely the backend Get call.
type Coordinator struct {
	cache             Cache
	flights           singleflight.Group
	observer          Observer
	filter            ExistenceFilter
	filterFailureMode FilterFailureMode
}

type flightResult[R any] struct {
	value R
}

// NewCoordinator constructs a full-path cache-aside coordinator for backend.
func NewCoordinator(backend Cache, options ...CoordinatorOption) (*Coordinator, error) {
	if isNilCache(backend) {
		return nil, errors.New("cache coordinator requires a backend")
	}
	c := &Coordinator{
		cache:             backend,
		observer:          func(Event) {},
		filterFailureMode: FilterFailOpen,
	}
	for _, option := range options {
		if option != nil {
			option(c)
		}
	}
	return c, nil
}

// GetOrLoad returns a decoded cache hit or executes one shared read-through
// flight for a miss. Go does not permit generic methods, so this is a function.
func GetOrLoad[R any](coordinator *Coordinator, request *CacheOption, loader func(context.Context) (R, error)) (R, error) {
	var zero R
	if coordinator == nil || isNilCache(coordinator.cache) {
		return zero, errors.New("cache coordinator requires a backend")
	}
	if request == nil {
		return zero, errors.New("cache option is required")
	}
	if loader == nil {
		return zero, errors.New("cache loader is required")
	}
	if err := request.Valid(); err != nil {
		return zero, err
	}
	if !request.IsCache() {
		return loader(request.GetContextCtx())
	}
	if err := coordinator.allowByFilter(request); err != nil {
		return zero, err
	}
	sharedRequest := snapshotCacheOption(request)

	value, err := coordinator.cache.Get(request.GetContextCtx(), request.GetCacheKey(), request)
	if err == nil {
		decoded, err := decodeCached[R](request, value)
		if err != nil {
			return zero, err
		}
		coordinator.observer(EventCacheHit)
		return decoded, nil
	}
	if !IsMiss(err) && request.GetCacheErrorPolicy() != LoadOnCacheError {
		coordinator.observer(EventCacheError)
		return zero, err
	}
	coordinator.observer(EventCacheMiss)

	result := coordinator.flights.DoChan(flightKey[R](coordinator.cache, sharedRequest), func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(sharedRequest.GetContextCtx()), sharedRequest.GetLoadTimeout())
		defer cancel()

		cached, err := coordinator.cache.Get(loadCtx, sharedRequest.GetCacheKey(), sharedRequest)
		if err == nil {
			value, err := decodeCached[R](sharedRequest, cached)
			return flightResult[R]{value: value}, err
		}
		if !IsMiss(err) && sharedRequest.GetCacheErrorPolicy() != LoadOnCacheError {
			coordinator.observer(EventCacheError)
			return zero, err
		}

		loaded, err := loader(loadCtx)
		if err != nil {
			return zero, err
		}
		encoded, err := sharedRequest.GetJsonWrapper().Marshal(loaded)
		if err != nil {
			return zero, err
		}
		if err := coordinator.cache.Set(loadCtx, sharedRequest.GetCacheKey(), frameUtils.UnsafeString(encoded), sharedRequest); err != nil {
			coordinator.observer(EventFillError)
		} else if sharedRequest.GetBloomFilterState() && coordinator.filter != nil {
			if err := coordinator.filter.Add(loadCtx, sharedRequest.GetCacheKey()); err != nil {
				coordinator.observer(EventFilterError)
			}
		}
		return flightResult[R]{value: loaded}, nil
	})

	select {
	case <-request.GetContextCtx().Done():
		return zero, request.GetContextCtx().Err()
	case shared := <-result:
		if shared.Err != nil {
			return zero, shared.Err
		}
		decoded, ok := shared.Val.(flightResult[R])
		if !ok {
			return zero, fmt.Errorf("cache coordinator returned %T for %s", shared.Val, reflect.TypeFor[R]())
		}
		return decoded.value, nil
	}
}

func (c *Coordinator) allowByFilter(request *CacheOption) error {
	if !request.GetBloomFilterState() || c.filter == nil || !c.filter.Ready() {
		return nil
	}
	mayContain, err := c.filter.MayContain(request.GetContextCtx(), request.GetCacheKey())
	if err != nil {
		c.observer(EventFilterError)
		if ctxErr := request.GetContextCtx().Err(); ctxErr != nil {
			return ctxErr
		}
		if c.filterFailureMode == FilterFailOpen {
			return nil
		}
		return NewErrRejectedByBloomFilter(request.GetCacheKey())
	}
	if !mayContain {
		return NewErrRejectedByBloomFilter(request.GetCacheKey())
	}
	return nil
}

func decodeCached[R any](request *CacheOption, value string) (R, error) {
	var decoded R
	if err := request.GetJsonWrapper().Unmarshal(frameUtils.UnsafeBytes(value), &decoded); err != nil {
		return decoded, err
	}
	return decoded, nil
}

// snapshotCacheOption detaches an asynchronous flight from the caller-owned
// option, which may be returned to its pool as soon as that caller cancels.
func snapshotCacheOption(request *CacheOption) *CacheOption {
	snapshot := *request
	if request.localTTLConfig != nil {
		localTTL := *request.localTTLConfig
		snapshot.localTTLConfig = &localTTL
	}
	if request.remoteTTLConfig != nil {
		remoteTTL := *request.remoteTTLConfig
		snapshot.remoteTTLConfig = &remoteTTL
	}
	return &snapshot
}

func flightKey[R any](backend Cache, request *CacheOption) string {
	namespace := string(request.GetDefaultInstanceKey())
	if namespace == "" {
		namespace = fmt.Sprintf("level:%d", backend.GetLevel())
	}
	return namespace + "\x00" + request.GetCacheKey() + "\x00" + reflect.TypeFor[R]().String()
}

func isNilCache(cache Cache) bool {
	if cache == nil {
		return true
	}
	value := reflect.ValueOf(cache)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

var coordinatorsByBackend sync.Map

// GetCached is kept for compatibility. New callers should retain a Coordinator
// and call GetOrLoad directly so all callers share the same full-path flight.
func GetCached[R any](
	cacheOption *CacheOption,
	loader func(context.Context) (R, error),
	fallback ...func() (R, error),
) (R, error) {
	var zero R
	if cacheOption == nil {
		return zero, errors.New("cache option is required")
	}
	if err := cacheOption.Valid(); err != nil {
		return zero, err
	}
	if !cacheOption.IsCache() {
		return loader(cacheOption.GetContextCtx())
	}

	backend, err := NewFactory(cacheOption.GetContext()).Lookup(cacheOption.GetCacheLevel())
	if err != nil {
		return zero, err
	}
	coordinator, err := coordinatorFor(backend)
	if err != nil {
		return zero, err
	}
	value, err := GetOrLoad(coordinator, cacheOption, loader)
	if err == nil {
		return value, nil
	}
	var circuitOpen ErrCircuitBreakerOpen
	if errors.As(err, &circuitOpen) {
		if len(fallback) > 0 {
			return fallback[0]()
		}
		return zero, circuitOpen
	}
	return zero, err
}

func coordinatorFor(backend Cache) (*Coordinator, error) {
	identity, cacheable := cacheIdentity(backend)
	if !cacheable {
		return NewCoordinator(backend)
	}
	if existing, ok := coordinatorsByBackend.Load(identity); ok {
		return existing.(*Coordinator), nil
	}
	created, err := NewCoordinator(backend)
	if err != nil {
		return nil, err
	}
	actual, _ := coordinatorsByBackend.LoadOrStore(identity, created)
	return actual.(*Coordinator), nil
}

func cacheIdentity(backend Cache) (any, bool) {
	value := reflect.ValueOf(backend)
	if value.Kind() == reflect.Pointer && !value.IsNil() {
		return struct {
			typeName string
			pointer  uintptr
		}{reflect.TypeOf(backend).String(), value.Pointer()}, true
	}
	if reflect.TypeOf(backend).Comparable() {
		return backend, true
	}
	return nil, false
}

// Factory resolves cache backends from the application's registry.
type Factory struct {
	ctx fiberhouse.IContext
}

// NewFactory creates a cache factory.
func NewFactory(ctx fiberhouse.IContext) *Factory {
	return &Factory{ctx: ctx}
}

// Lookup returns the requested cache backend without compatibility panics.
func (cf *Factory) Lookup(level Level) (Cache, error) {
	if cf == nil || cf.ctx == nil {
		return nil, errors.New("cache factory requires a context")
	}
	starter := cf.ctx.GetStarter()
	if starter == nil || starter.GetApplication() == nil {
		return nil, errors.New("cache factory requires an application")
	}
	application := starter.GetApplication()
	var key string
	switch level {
	case Local:
		key = application.GetLocalCacheKey()
	case Remote:
		key = application.GetRemoteCacheKey()
	case Level2:
		key = application.GetLevel2CacheKey()
	default:
		return nil, fmt.Errorf("unsupported cache level: %d", level)
	}
	return fiberhouse.GetInstance[Cache](key)
}

// GetCache preserves the original panic-on-resolution-failure behavior.
func (cf *Factory) GetCache(level Level) Cache {
	backend, err := cf.Lookup(level)
	if err != nil {
		panic(err)
	}
	return backend
}
