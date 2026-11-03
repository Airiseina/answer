// Package tenanttest 为多租户相关的用例提供测试夹具。
//
// 它构建 DryRun 模式的 GORM DB：不会真正连接数据库，也不会执行 SQL，
// 只把查询编译成 SQL 语句，供用例断言「租户过滤条件是否被正确附加」。
// 这样无需在 CI 里起 MySQL / PostgreSQL 也能守住越权回归。
package tenanttest

import (
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewMySQL 返回一个 DryRun 的 MySQL 会话。
func NewMySQL(t testing.TB) *gorm.DB {
	t.Helper()
	return NewMySQLWithLogger(t, logger.Default.LogMode(logger.Silent))
}

// NewMySQLWithLogger 与 NewMySQL 相同，但允许注入自定义 Logger。
//
// GORM 在 DryRun 下依然会回调 Logger.Trace（见 callbacks.processor.Execute），
// 因此用例可以借此拿到编译后的完整语句（含已绑定的参数值），
// 用来断言「查询带了哪些过滤条件」这类无法从返回值观察的行为。
func NewMySQLWithLogger(t testing.TB, l logger.Interface) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "answer:answer@tcp(127.0.0.1:3306)/answer?charset=utf8&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		SkipDefaultTransaction: true,
		Logger:                 l,
	})
	if err != nil {
		t.Fatalf("构建 DryRun MySQL 会话失败: %v", err)
	}
	return db
}

// NewPostgres 返回一个 DryRun 的 PostgreSQL 会话。
func NewPostgres(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 user=answer password=answer dbname=answer_chat port=5432 sslmode=disable",
	}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("构建 DryRun PostgreSQL 会话失败: %v", err)
	}
	return db
}

// Capture 执行 build 并返回其编译出的 SQL 与绑定参数。
// build 里应当只做语句构造（例如 tx.Where(...).First(&model)），DryRun 下不会真正执行。
//
// 注意 SkipDefaultTransaction：GORM 默认给写操作（Create/Update/Delete）套一层事务，
// 而开启事务需要真实连接，DryRun 也拦不住，会让语句在编译前就因连接失败而中断。
// 关掉默认事务后，写语句同样能被编译出来，越权断言才能覆盖写路径。
func Capture(t testing.TB, db *gorm.DB, build func(tx *gorm.DB) *gorm.DB) (string, []interface{}) {
	t.Helper()
	tx := build(db.Session(&gorm.Session{DryRun: true, SkipDefaultTransaction: true}))
	if tx == nil || tx.Statement == nil {
		t.Fatalf("DryRun 未生成 Statement")
	}
	return tx.Statement.SQL.String(), tx.Statement.Vars
}
