//go:build liveintegration

package dbmysql

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// nonAlnumUnderscore 匹配任何非字母、数字、下划线的字符，用于把 t.Name()
// （可能因子测试嵌套而含有 "/" 等字符）净化成合法的 MySQL 标识符片段。
var nonAlnumUnderscore = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// sanitizeForTableName 把任意字符串转换成只包含字母、数字、下划线的片段，
// 供拼接 MySQL 表名使用（MySQL 表名不允许包含 "/"、"-"、"." 等字符）。
//
// MySQL 标识符长度上限是 64 字符：若直接拼接完整 t.Name()（子测试嵌套时
// 可能很长）加纳秒时间戳，实测会超限触发 "Identifier name ... is too
// long"（错误码 1059）。因此这里额外对净化后的字符串做 fnv32a 哈希，
// 只保留 8 位十六进制摘要，既避免超长，又保留对不同测试名/不同运行的
// 区分度（配合调用方额外拼接的纳秒时间戳，实际唯一性由两者共同保证）。
func sanitizeForTableName(s string) string {
	clean := nonAlnumUnderscore.ReplaceAllString(s, "_")
	h := fnv.New32a()
	_, _ = h.Write([]byte(clean))
	return fmt.Sprintf("%x", h.Sum32())
}

// liveTestRecord 是本测试专属的最小 GORM model，仅用于验证仓库 MySQL
// client 的建表/写入/读取路径是否对真实数据库正常工作，不模拟真实业务
// schema。
type liveTestRecord struct {
	ID     uint `gorm:"primaryKey"`
	Marker string
}

// TestLive_MysqlDb_CreateWriteReadDropClose 针对真实 MySQL 容器
// （127.0.0.1:3306，库 test）验证 MysqlDb 的建表-写入-读取-删表-关闭
// 完整流程。
//
// 表名唯一性验证结论：本地实测（真实容器，MySQL 9.4.0，
// gorm.io/gorm v1.31.2 + gorm.io/driver/mysql v1.6.0）确认
// db.Client.Table(name).Migrator().AutoMigrate(...) /
// .Migrator().DropTable(...) 这种动态表名写法工作正常——AutoMigrate 会
// 尊重 Table() 指定的表名建表，两个不同的动态表名之间数据互不串扰，
// DropTable 也确实按 Table() 指定的表名删除，未观察到"忽略 Table()、
// 仍用 model 默认表名"的问题。因此本测试直接采用 Table()+Migrator()
// 方案，未改用 TableName() 方法方案。
func TestLive_MysqlDb_CreateWriteReadDropClose(t *testing.T) {
	dsn := "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"
	ctx := newTestMysqlAppContext(t, dsn)

	db, err := NewMysqlDb(ctx, "test.mysql")
	require.NoError(t, err, "must be able to connect to the live MySQL container")
	// t.Cleanup 是 LIFO：先注册 Close，再注册 DropTable，
	// 这样实际执行顺序是 DropTable 先跑（后注册先执行），
	// Close 最后跑，保证删表发生在连接关闭之前。
	t.Cleanup(func() { _ = db.Close() })

	// 表名必须是合法的 MySQL 标识符：t.Name() 在子测试场景下可能包含 "/"，
	// 用 sanitizeForTableName 只保留字母、数字、下划线；再拼接纳秒时间戳，
	// 避免并发 CI 运行或失败重跑相互污染。
	tableSuffix := sanitizeForTableName(fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano()))
	tableName := "p1_4c_live_" + tableSuffix

	require.NoError(t, db.Client.Table(tableName).Migrator().AutoMigrate(&liveTestRecord{}))
	t.Cleanup(func() {
		_ = db.Client.Table(tableName).Migrator().DropTable(&liveTestRecord{})
	})

	marker := fmt.Sprintf("marker-%d", time.Now().UnixNano())
	require.NoError(t, db.Client.Table(tableName).Create(&liveTestRecord{Marker: marker}).Error)

	var got liveTestRecord
	require.NoError(t, db.Client.Table(tableName).Where("marker = ?", marker).First(&got).Error)
	require.Equal(t, marker, got.Marker)
}
