// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cachelocal

// CacheMetricsInfo 缓存统计信息结构
type CacheMetricsInfo struct {
	HitRatio      float64 `json:"hitRatio"`      // 命中率
	MissRatio     float64 `json:"missRatio"`     // 未命中率
	TotalHits     uint64  `json:"totalHits"`     // 总命中次数
	TotalMisses   uint64  `json:"totalMisses"`   // 总未命中次数
	TotalRequests uint64  `json:"totalRequests"` // 总请求次数
	KeysAdded     uint64  `json:"keysAdded"`     // 添加的键总数
	KeysEvicted   uint64  `json:"keysEvicted"`   // 被驱逐的键总数
	KeysUpdated   uint64  `json:"keysUpdated"`   // 更新的键总数
	CostAdded     uint64  `json:"costAdded"`     // 添加的总成本
	CostEvicted   uint64  `json:"costEvicted"`   // 被驱逐的总成本
	SetsDropped   uint64  `json:"setsDropped"`   // 被丢弃的设置操作数
	SetsRejected  uint64  `json:"setsRejected"`  // 被拒绝的设置操作数
	GetsDropped   uint64  `json:"getsDropped"`   // 被丢弃的获取操作数
	GetsKept      uint64  `json:"getsKept"`      // 保留的获取操作数
}
