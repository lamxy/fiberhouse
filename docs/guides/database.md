# 数据库

FiberHouse 提供 GORM/MySQL 与 MongoDB v2 client 包装、健康检查、GlobalManager 生命周期接口，以及供业务 model 组合使用的 locator 基类。它们都是应用可选组件：导入 package、创建 `FiberHouse` 或调用默认 provider 集合都不会创建数据库、schema、table 或 collection。

数据库实例不在默认 provider/manager 集合中。应用需要注册 initializer、定义实例 key，并决定是否在启动期强制连接；仓库示例只展示一种选择。

## MySQL / GORM 构造

```go
db, err := dbmysql.NewMysqlDb(appCtx) // 默认配置路径 database.mysql
db, err := dbmysql.NewMysqlDb(appCtx, "tenant.primary")
```

`NewMysqlDb` 调用 `NewClient`，后者依次：

1. 读取 DSN；空值立即返回 error。
2. 配置 GORM logger。
3. 用 MySQL driver 打开 GORM，固定设置 `SkipDefaultTransaction=true`、`PrepareStmt=true`。
4. 取得 `*sql.DB` 并设置 pool。
5. 用 10 秒 context 执行 `PingContext`；失败则构造失败。

实际消费的配置键是：

| 路径 | 作用 |
|---|---|
| `<base>.dsn` | MySQL DSN，必需；DSN 中的数据库必须由部署/迁移流程准备 |
| `<base>.gorm.maxIdleConns` / `maxOpenConns` | pool 上限 |
| `<base>.gorm.connMaxLifetime` / `connMaxIdleTime` | 数值乘以 `time.Second` |
| `<base>.gorm.logger.enable` | 是否启用 GORM logger |
| `<base>.gorm.logger.level` | `silent`、`error`、`warn`、`info`；未知值回退 error |
| `<base>.gorm.logger.slowThreshold` | 数值乘以 `time.Millisecond` |
| `<base>.gorm.logger.colorful` / `skipDefaultFields` | logger 选项 |

示例配置中的 `<base>.pingTry` 当前没有被构造器读取；MySQL 无论该值如何都会在构造时 ping。`gorm.Open`、ping 或 pool 获取失败会返回 error，但已创建到一半的 handle 没有独立的失败回收编排。

## MongoDB 构造

```go
db, err := dbmongo.NewMongoDb(appCtx) // 默认配置路径 database.mongodb
```

MongoDB `NewClient` 创建 BSON registry 和 driver options，然后调用 `mongo.Connect`。实际读取：

- `applyURI`；
- `maxPoolSize`、`minPoolSize`；
- `maxConnIdleTime`、`connectTimeout`、`socketTimeout`、`heartbeatInterval`，均按秒解释；
- 固定 `writeconcern.Majority()`、`readpref.SecondaryPreferred()`；
- 固定 BSON 选项 `UseJSONStructTags`、`ErrorOnInlineDuplicates`、`IntMinSize`。

构造时没有调用 ping。`mongo.Connect` 返回成功只表示 client 已构造，服务可达性需通过后续操作或 `IsHealthy` 检查。示例配置使用 `clientTimeout`，而当前构造器读取的是 `socketTimeout`；示例的 `pingTry` 同样没有被读取。正式配置应以当前消费键为准，不要复制未消费字段。

## GlobalManager 注册

应用可把两个构造器注册为 initializer：

```go
func (app *Application) ConfigGlobalInitializers() globalmanager.InitializerMap {
	return globalmanager.InitializerMap{
		"db.mysql": func() (any, error) {
			return dbmysql.NewMysqlDb(app.Ctx, "database.mysql")
		},
		"db.mongo": func() (any, error) {
			return dbmongo.NewMongoDb(app.Ctx, "database.mongodb")
		},
	}
}
```

这些 key 是应用契约，不必与配置路径相同。只有进入 `ConfigRequiredGlobalKeys()` 或第一次 `Get` 的 key 才会初始化；而标准 FrameStarter 对 required key 的失败只记日志并继续，严格 fail-fast 需要应用入口额外处理。

[`example_application`](../../example_application/) 的 `Application.ConfigRequiredGlobalKeys` 把 MySQL、MongoDB 都列为 required，因而完整 Web 示例启动会依赖两者。这不是最小 FiberHouse 应用要求，也不是生产环境必须同时采用两种数据库。

## Health、Rebuild 与 Close

两种 wrapper 都实现 GlobalManager 的 `HealthChecker`、`Rebuilder` 和 `Closable`：

| 操作 | MySQL | MongoDB |
|---|---|---|
| `IsHealthy()` | 10 秒 context，`sql.DB.PingContext` | 10 秒 context，对固定数据库 `test` 执行 `{ping: 1}` |
| `Rebuild(...)` | 重新 `NewClient`，替换 `Client` | 重新 `NewClient`，替换 `Client` |
| `Close()` | `sql.DB.Close()` | 5 秒 context，`Client.Disconnect()` |

MongoDB health 使用固定的 `test` 数据库，不读取 model 的数据库名。GlobalManager 在未初始化对象上不会触发 health；其 keepalive 也不会主动创建 lazy client。

两个 `ReNewClient` 只在写替换时持有 wrapper 的 mutex，业务读取 `Client` 不持有同一读锁；重建没有等待旧查询结束，也没有关闭旧 client。GlobalManager 的 `Rebuild` 再次把同一个 wrapper 存回 entry，同样不补齐迁移和关闭。把重建用于生产前，应用必须建立停流、切换、等待和旧 client 回收协议。

标准 HTTP shutdown 调用 `ClearAll(true)`，不会逐项 `Close`。GlobalManager `Release` 只会重置成功关闭的 `Closable`，也不能替代资源所有者安排数据库 client 的关闭；因此数据库创建者仍应显式关闭 client，不依赖容器清空。详见[《GlobalManager》](global-manager.md)。

## MySQL model 基类

`dbmysql.NewMysqlModel(ctx, optionalInstanceKey...)` 从 Context 的 GlobalManager 取得 `*MysqlDb`。未传 key 时使用应用的 `GetDBMysqlKey()`；key 缺失会 panic，实例类型错误会在直接断言处 panic。

`MysqlModel` 保存 Context、DB、table 和名称。`SetTable(name, prefixes...)` 支持零至两个下划线前缀，`GetTableName` 只计算名称。`SetDbName` 保存的是 locator 元数据，不会更改 DSN 或切换 GORM 当前数据库；真正连接到哪个库仍由 MySQL DSN 和业务 GORM 调用决定。

业务 model 通常组合该基类，再显式使用：

```go
m.GetDB().Client.Table(m.GetTable()).Where(...)
```

框架不会自动执行 `AutoMigrate`。CLI 示例中的 `AutoMigrate` 是业务命令行为，而且它也不等价于创建 DSN 中不存在的数据库。

## MongoDB model 基类

`dbmongo.NewMongoModel` 同样按 optional key 或 `GetDBMongoKey()` 取得 `*MongoDb`，并有相同的 panic 边界。使用默认数据库前必须调用 `SetDbName`；名称为空时 `GetDatabase`/`GetCollection` 会构造 internal exception 并 panic。

`SetColl`/`SetTable` 保存集合名并支持前缀，但 `GetCollection(coll, ...)` 仍要求调用方显式传入非空集合名；它不会自动读取 `MongoModel.Coll`。`GetClientDatabase(name)` 可临时选择其他数据库。locator 只选择 client 上的 database/collection handle，不创建部署资源或验证 collection schema。

## `MongoDecimal` registry

每次 `dbmongo.NewClient` 都创建 BSON registry，并为 `govalues/decimal.Decimal` 注册 [`mongodecimal.MongoDecimal`](../../component/database/dbmongo/internal/mongodecimal/mongo_decimal.go) encoder/decoder。它在 Go decimal 字符串与 BSON Decimal128 之间转换；类型不符、Decimal128 解析以及 BSON reader/writer 错误会原样包装返回。

该 codec 随 Mongo client options 生效，不是 GlobalManager 中的独立服务，也不代表其他 decimal package 或任意数值类型会自动转换。自定义 registry 时若覆盖当前 registry，需要重新考虑这项注册。

## 配置与运行检查清单

- 由部署或 migration 明确创建 MySQL database/schema；不要指望导入或构造器完成。
- 用当前源码消费的键检查配置，尤其是 MongoDB `socketTimeout` 和未消费的 `pingTry`。
- 让 DSN/URI、认证和 TLS 来自安全配置来源，不照搬示例明文值。
- 根据外部服务容量设置 pool 和 timeout；缺失数值会变成零值，不等于示例默认。
- 在启动入口决定连接失败是记录后继续还是 fail-fast。
- 为查询停流、worker 停止、client close 和日志 close 指定顺序；记录关闭错误。
- 不在有并发读者时直接调用当前 `Rebuild`。

源码入口：[`component/database/dbmysql/mysql.go`](../../component/database/dbmysql/mysql.go)、[`component/database/dbmysql/mysql_model_impl.go`](../../component/database/dbmysql/mysql_model_impl.go)、[`component/database/dbmongo/mongo.go`](../../component/database/dbmongo/mongo.go) 与 [`component/database/dbmongo/mongo_model_impl.go`](../../component/database/dbmongo/mongo_model_impl.go)。
