package globalmanager

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

// smallObj 为被管理的示例对象
type smallObj struct {
	ID uint64
}

// helper: 预加载若干 key
func preloadKeys(gm *GlobalManager, count uint64) {
	for i := uint64(1); i <= count; i++ {
		name := fmt.Sprintf("pre_%d", i)
		// 忽略返回值（并行读场景只需成功注册即可）
		gm.Register(name, func(id uint64) InitializerFunc {
			return func() (interface{}, error) {
				return &smallObj{ID: id}, nil
			}
		}(i))
	}
}

// 读重写轻场景：每 100 次读 1 次写 => 读占 99%
func BenchmarkGlobalManager_ReadHeavy(b *testing.B) {
	const (
		initialKeys   = 5000 // 预热 key 数量
		writeInterval = 100  // 每 100 次操作 1 次写 => 写占 1%
	)
	gm := NewGlobalManager()
	preloadKeys(gm, initialKeys)

	var seq uint64 = initialKeys // 当前已注册最大序号（预热后起点）
	var ops uint64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			i := atomic.AddUint64(&ops, 1)
			// 写：1%
			if i%writeInterval == 0 {
				id := atomic.AddUint64(&seq, 1)
				name := fmt.Sprintf("dyn_%d", id)
				gm.Register(name, func(localID uint64) InitializerFunc {
					return func() (interface{}, error) {
						return &smallObj{ID: localID}, nil
					}
				}(id))
				continue
			}
			// 读：随机读取 [1, seq]
			max := atomic.LoadUint64(&seq)
			if max == 0 {
				continue
			}
			target := uint64(r.Int63n(int64(max))) + 1
			var key KeyName
			if target <= initialKeys {
				key = fmt.Sprintf("pre_%d", target)
			} else {
				key = fmt.Sprintf("dyn_%d", target)
			}
			_, _ = gm.Get(key) // 忽略错误（极端竞争下可能读到尚未完成初始化的失败记录后重试）
		}
	})
}

// 读写均衡场景：写 / 读 约 50% / 50%
func BenchmarkGlobalManager_Balanced(b *testing.B) {
	const (
		initialKeys = 2000
	)
	gm := NewGlobalManager()
	preloadKeys(gm, initialKeys)

	var seq uint64 = initialKeys
	var ops uint64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			i := atomic.AddUint64(&ops, 1)
			// 奇偶各一半：写 / 读 约 50% / 50%
			if i&1 == 0 {
				id := atomic.AddUint64(&seq, 1)
				name := fmt.Sprintf("dyn_%d", id)
				gm.Register(name, func(localID uint64) InitializerFunc {
					return func() (interface{}, error) {
						return &smallObj{ID: localID}, nil
					}
				}(id))
			} else {
				max := atomic.LoadUint64(&seq)
				if max == 0 {
					continue
				}
				target := uint64(r.Int63n(int64(max))) + 1
				var key KeyName
				if target <= initialKeys {
					key = fmt.Sprintf("pre_%d", target)
				} else {
					key = fmt.Sprintf("dyn_%d", target)
				}
				_, _ = gm.Get(key)
			}
		}
	})
}

// 写重读轻场景：每 100 次写 1 次读 => 读占 1%
func BenchmarkGlobalManager_WriteHeavy(b *testing.B) {
	const (
		initialKeys  = 1000
		readInterval = 100 // 每 100 次写 1 次读 => 读占 1%
	)
	gm := NewGlobalManager()
	preloadKeys(gm, initialKeys)

	var seq uint64 = initialKeys
	var ops uint64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			i := atomic.AddUint64(&ops, 1)
			// 读：1%
			if i%readInterval == 0 {
				max := atomic.LoadUint64(&seq)
				if max == 0 {
					continue
				}
				target := uint64(r.Int63n(int64(max))) + 1
				var key KeyName
				if target <= initialKeys {
					key = fmt.Sprintf("pre_%d", target)
				} else {
					key = fmt.Sprintf("dyn_%d", target)
				}
				_, _ = gm.Get(key)
				continue
			}
			// 写：99%
			id := atomic.AddUint64(&seq, 1)
			name := fmt.Sprintf("dyn_%d", id)
			gm.Register(name, func(localID uint64) InitializerFunc {
				return func() (interface{}, error) {
					return &smallObj{ID: localID}, nil
				}
			}(id))
		}
	})
}

// 对 Register 本身的纯写吞吐做一个基准（单线程），比较无 Get 干扰
func BenchmarkGlobalManager_PureRegister(b *testing.B) {
	gm := NewGlobalManager()
	var seq uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := atomic.AddUint64(&seq, 1)
		name := fmt.Sprintf("only_%d", id)
		gm.Register(name, func(localID uint64) InitializerFunc {
			return func() (interface{}, error) {
				return &smallObj{ID: localID}, nil
			}
		}(id))
	}
}

// 对已初始化实例的纯读取吞吐（全部命中已存在且已初始化）
func BenchmarkGlobalManager_PureGet(b *testing.B) {
	gm := NewGlobalManager()
	preloadKeys(gm, 5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("pre_%d", (i%5000)+1)
		//key := "pre_" + strconv.Itoa((i%5000)+1)
		_, _ = gm.Get(key)
	}
}
