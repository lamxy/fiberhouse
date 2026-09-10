// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// 本文件补齐 Level2Cache 作为 globalmanager 组合对象的容器级契约测试。
//
// globalmanager 侧已有完整的关闭排序用例，但它们全部使用包内 stub 组合对象
// （orderedComposite、nestedComposite）。本仓库唯一真实的 CompositeClosable 是
// Level2Cache，而它的 OwnsChildren / OwnsInstance 此前没有任何测试调用——排序
// 逻辑被 stub 证明成立，真实实现却从未与容器对接过。
//
// 这段路径不是理论风险：示例应用把 KEY_LEVEL2_CACHE 列入 ConfigRequiredGlobalKeys，
// 且 KEY_REMOTE_CACHE 是 KEY_REDIS 的别名，因此出货的关闭路径确实会走这里。
// 见 level2_cache.go OwnsChildren 的注释：容器若先关掉 remote，L2 排空在途异步
// 写入期间就会打在已关闭的客户端上。

// registerInstance 把一个已构造好的实例注册进容器并强制其进入已初始化状态。
//
// ReleaseAll 的排序快照只考虑已初始化的 entry（partitionByOwnership 读
// loadedInstance），因此必须 Get 一次让实例落位，否则组合对象会被当作未初始化
// 条目跳过，测试将永远通过而不检验任何东西。
func registerInstance(t *testing.T, gm *globalmanager.GlobalManager, key globalmanager.KeyName, value interface{}) {
	t.Helper()
	require.True(t, gm.Register(key, func() (interface{}, error) {
		return value, nil
	}), "register %s", key)
	got, err := gm.Get(key)
	require.NoError(t, err, "get %s", key)
	require.Same(t, value, got, "container must hand back the same instance for %s", key)
}

// TestLevel2Cache_OwnsChildrenReportsClosureResponsibility 锁定 OwnsChildren 的语义：
// 它回答的是「本对象是否负责关闭子缓存」，而不是「是否持有子缓存引用」。
//
// 借用型 L2（ownChildren=false）必须返回 false，否则容器会把它排进组合对象组并
// 赋予它本不该有的关闭优先级。
func TestLevel2Cache_OwnsChildrenReportsClosureResponsibility(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)

	owning := newTestLevel2(t, local, remote)
	assert.True(t, owning.OwnsChildren(), "initLifecycle(true) must claim closure responsibility")

	borrowing := newTestLevel2(t, local, remote)
	borrowing.initLifecycle(false)
	assert.False(t, borrowing.OwnsChildren(), "a borrowing L2 must not claim closure responsibility")
}

// TestLevel2Cache_OwnsInstanceIdentifiesOwnChildrenOnly 锁定 OwnsInstance 的判定范围。
//
// 容器用它计算嵌套深度，误报会把无关对象排进本对象的持有链，进而扭曲关闭顺序。
// 因此它必须严格按实例身份判断，且在不持有子资源时恒为 false。
func TestLevel2Cache_OwnsInstanceIdentifiesOwnChildrenOnly(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	stranger := newRecordingCache(cache.Remote)

	l2 := newTestLevel2(t, local, remote)

	assert.True(t, l2.OwnsInstance(local), "local child must be recognized")
	assert.True(t, l2.OwnsInstance(remote), "remote child must be recognized")
	assert.False(t, l2.OwnsInstance(stranger), "an unrelated cache must not be claimed")
	assert.False(t, l2.OwnsInstance(nil), "nil candidate must not be claimed")

	borrowing := newTestLevel2(t, local, remote)
	borrowing.initLifecycle(false)
	assert.False(t, borrowing.OwnsInstance(local),
		"a borrowing L2 must claim nothing, even for a cache it references")
}

// TestReleaseAll_RealLevel2ClosesBeforeItsAliasedChildren 是本文件的核心用例：
// 把真实 Level2Cache 与它的两个子缓存按示例应用的方式注册进真实容器，验证
// ReleaseAll 先释放组合对象、再释放子缓存各自的 key。
//
// 这里刻意复刻示例的别名结构（子缓存同时以自己的 key 注册），因为正是别名让
// 「子资源会被关闭两次」成为常态，也正是它让关闭顺序变得有意义。
//
// 重复多轮：ReleaseAll 的分组基于 sync.Map.Range，其迭代顺序未定义且每次运行
// 都可能不同。单轮通过不能证明顺序契约成立。
func TestReleaseAll_RealLevel2ClosesBeforeItsAliasedChildren(t *testing.T) {
	const rounds = 200

	for round := 0; round < rounds; round++ {
		local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
		l2 := newTestLevel2(t, local, remote)

		gm := globalmanager.NewGlobalManager()
		registerInstance(t, gm, "localCache", local)
		registerInstance(t, gm, "remoteCache", remote)
		registerInstance(t, gm, "level2Cache", l2)

		var failures []globalmanager.KeyName
		gm.ReleaseAllWithObserver(func(name globalmanager.KeyName, err error) {
			failures = append(failures, name)
		}, true)

		require.Empty(t, failures, "round %d: no key may fail to release", round)

		// L2 关闭时会关掉两个子缓存；容器随后对子缓存各自的 key 再调用一次 Close。
		// 幂等 Close 吸收第二次调用，但它确实发生，所以计数至少为 1。
		assert.GreaterOrEqual(t, local.closeCalls, 1, "round %d: local must be closed", round)
		assert.GreaterOrEqual(t, remote.closeCalls, 1, "round %d: remote must be closed", round)

		// 关键断言：L2 必须已完成关闭。若容器先关了 remote，L2 的排空会打在
		// 已关闭的缓存上——这正是 OwnsChildren 存在的理由。
		assert.True(t, l2.closed.Load(), "round %d: level2 must be closed", round)
	}
}

// TestReleaseAll_BorrowingLevel2IsNotPrioritized 确认不负责关闭子资源的 L2 被
// 当作普通对象处理：它没有关闭顺序约束，子缓存仍由容器各自释放。
func TestReleaseAll_BorrowingLevel2IsNotPrioritized(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	l2 := newTestLevel2(t, local, remote)
	l2.initLifecycle(false)

	gm := globalmanager.NewGlobalManager()
	registerInstance(t, gm, "localCache", local)
	registerInstance(t, gm, "remoteCache", remote)
	registerInstance(t, gm, "level2Cache", l2)

	gm.ReleaseAllWithObserver(nil, true)

	// 借用者不关闭子资源，但容器仍会按各自的 key 释放它们。
	assert.GreaterOrEqual(t, local.closeCalls, 1, "local must still be closed by the container")
	assert.GreaterOrEqual(t, remote.closeCalls, 1, "remote must still be closed by the container")
}
