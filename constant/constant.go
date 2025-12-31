// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package constant

// 通用常量定义
const (
	// RegisterKeyPrefix 通用对象注册key前缀
	RegisterKeyPrefix = "__key_"

	// ContextKeyPrefix 上下文注册key前缀
	ContextKeyPrefix = "__ctx_"

	// LogOriginKeyPrefix 不同日志源的日志器注册key前缀
	LogOriginKeyPrefix = "__logOriginKey_"

	// LogWriterKeyPrefix 日志写入器注册key前缀
	LogWriterKeyPrefix = "__logWriterKey_"

	// CacheProtectionKeyPrefix 缓存保护器注册key前缀
	CacheProtectionKeyPrefix = "__cacheProtectionKey_"

	// GlobalAppIContext 全局应用上下文IContext的注册key
	GlobalAppIContext = ContextKeyPrefix + "app_i_context"

	// DBConfPrefix DB默认配置的前缀
	DBConfPrefix = "database"
	// CacheConfPrefix Cache默认配置的前缀
	CacheConfPrefix = "cache"
	// DefaultMongoDBConfName 默认mongodb的配置路径名
	DefaultMongoDBConfName = "database.mongodb"
	// DefaultRedisDBConfName 默认redis的配置路径名
	DefaultRedisDBConfName = "cache.redis"
	// DefaultLocalCacheConfName 默认本地缓存配置路径名
	DefaultLocalCacheConfName = "cache.local"
	// DefaultLevel2CacheConfName 默认远程缓存配置路径名
	DefaultLevel2CacheConfName = "cache.level2"
	// DefaultMysqlDBConfName 默认mysql的配置路径名
	DefaultMysqlDBConfName = "database.mysql"

	// DefaultMongoDatabase mongodb默认数据库名
	DefaultMongoDatabase = "test"
	// DefaultMysqlDatabase mysql默认数据库名
	DefaultMysqlDatabase = "test"
	// DefaultRedisDBIndex redis的默认DB索引号
	DefaultRedisDBIndex = 0

	// DefaultKeepaliveTime 默认全局实例保活检查间隔
	DefaultKeepaliveTime = 180

	// DefaultPageSize 默认分页大小
	DefaultPageSize = 20

	// 默认框架启动器提供者类型标识
	FrameTypeWithDefaultFrameStarter = "DefaultFrameStarter"
	CoreTypeWithFiber                = "fiber"
	CoreTypeWithGin                  = "gin"
	JsonCodecWithStd                 = "std_json_codec"
	JsonCodecWithSonic               = "sonic_json_codec"
	JsonCodecWithGoJson              = "go_json_codec"
)
