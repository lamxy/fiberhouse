package constant

import (
	"github.com/lamxy/fiberhouse/example_application"
	"time"
)

// 业务相关的常量 module.CollXXX
const (

	// ==== task ======
	TaskMaxRetryDefault    = 3               // 任务最大重试次数
	TaskHandleDelayDefault = 1 * time.Minute // 任务处理延迟时间

	// XLanguageFlag 语言自定义http标头
	XLanguageFlag        = "X-language-flag"
	DefaultMongoDatabase = "test" // "default"

	// MongoInstanceKey Mongo实例key
	MongoInstanceKey = example_application.KEY_MONGODB

	// DbNameMongo Mongo主库名，默认为test: fiberhouse.DefaultMongoDBConfName
	DbNameMongo = "test"

	// MysqlInstanceKey Mysql实例key
	MysqlInstanceKey = example_application.KEY_MYSQL

	// 全局管理模块-服务-仓库-模型层级-模块顶级名称：Name[层级]Example
	NameModuleExample = "ExampleModule"

	// mongodb 集合集中管理
	// CollExample mongodb 集合样例集合
	CollExample = "example"
)
