//go:build liveintegration

package dbmongo

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// newTestMongoAppContext 构造用于测试的最小 fiberhouse.IContext，参照
// component/database/dbmysql/mysql_test.go 的 newTestMysqlAppContext 写法。
// 配置键对应 mongo.go 的 NewClient（第 55-98 行）实际读取路径：
// applyURI（aConf.String）、maxPoolSize/minPoolSize（aConf.Int64，内部转
// uint64）、maxConnIdleTime/connectTimeout/socketTimeout/heartbeatInterval
// （均为 aConf.Duration，内部乘以 time.Second）。已逐一核对，键名与规格
// 文件示例一致，无出入。
func newTestMongoAppContext(t *testing.T, applyURI string) fiberhouse.IContext {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"test.mongodb.applyURI":          applyURI,
		"test.mongodb.maxPoolSize":       int64(20),
		"test.mongodb.minPoolSize":       int64(2),
		"test.mongodb.maxConnIdleTime":   int64(60),
		"test.mongodb.connectTimeout":    int64(5),
		"test.mongodb.socketTimeout":     int64(5),
		"test.mongodb.heartbeatInterval": int64(10),
	})
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

// nonSafeCollectionChar 匹配任何非字母、数字、下划线的字符，用于把
// t.Name()（子测试嵌套时可能包含 "/"）净化成安全的 collection 名片段。
// MongoDB 对 collection 名称的限制比 MySQL 表名宽松得多（上限 255
// 字节，不禁止下划线，只禁止 "$"、null 字符、以 "system." 开头），因此
// 这里不需要像 dbmysql 的 sanitizeForTableName 那样额外做哈希压缩。
var nonSafeCollectionChar = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizeForCollectionName(s string) string {
	return nonSafeCollectionChar.ReplaceAllString(s, "_")
}

// liveTestDoc 是本测试专属的最小 bson model，仅用于验证仓库 MongoDB
// client 的连接/写入/读取路径是否对真实数据库正常工作，不模拟真实业务
// schema。
type liveTestDoc struct {
	Marker string `bson:"marker"`
}

// TestLive_MongoDb_CreateWriteReadDropClose 针对真实 MongoDB 容器
// （127.0.0.1:27037，库 test）验证 MongoDb 的建 collection-写入-读取-
// 删除-关闭完整流程。
//
// 清理顺序：t.Cleanup 是 LIFO，先注册 Close，后注册 Drop，实际执行顺序
// 是 Drop 先跑（后注册先执行），Close 最后跑，保证删除 collection 发生
// 在连接关闭之前，与 P1-4c（dbmysql）的做法一致。
func TestLive_MongoDb_CreateWriteReadDropClose(t *testing.T) {
	applyURI := "mongodb://admin:admin@127.0.0.1:27037/?authSource=admin"
	ctx := newTestMongoAppContext(t, applyURI)

	db, err := NewMongoDb(ctx, "test.mongodb")
	require.NoError(t, err, "must be able to connect to the live MongoDB container")
	t.Cleanup(func() { _ = db.Close() })

	// mongo.Connect 是懒连接，显式 Ping 一次确认真正建立了网络连接。
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	require.True(t, db.PingTry(pingCtx))

	// collection 名称必须唯一，避免并发 CI 运行或失败重跑相互污染；
	// t.Name() 中可能出现的 "/" 需要净化为下划线等安全字符。
	collName := "p1_4d_live_" + sanitizeForCollectionName(fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano()))
	require.LessOrEqual(t, len(collName), 255, "collection name must not exceed MongoDB's 255-byte limit")

	coll := db.Client.Database("test").Collection(collName)
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = coll.Drop(dropCtx)
	})

	marker := fmt.Sprintf("marker-%d", time.Now().UnixNano())
	insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer insertCancel()
	_, err = coll.InsertOne(insertCtx, liveTestDoc{Marker: marker})
	require.NoError(t, err)

	// filter 参数用 bson.D，与 mongo.go 的 PingTry 既有写法风格保持一致。
	findCtx, findCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer findCancel()
	var got liveTestDoc
	require.NoError(t, coll.FindOne(findCtx, bson.D{{Key: "marker", Value: marker}}).Decode(&got))
	require.Equal(t, marker, got.Marker)
}
