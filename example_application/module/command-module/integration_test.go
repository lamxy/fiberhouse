package commandmodule_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/service"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var safeDatabaseName = regexp.MustCompile(`^[a-z0-9_]+$`)

func TestMySQLCRUDIntegration(t *testing.T) {
	if os.Getenv("FIBERHOUSE_INTEGRATION") != "1" {
		t.Skip("set FIBERHOUSE_INTEGRATION=1 to run real-service integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runID := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	databaseName := "fiberhouse_it_" + runID
	if !safeDatabaseName.MatchString(databaseName) {
		t.Fatalf("unsafe generated database name %q", databaseName)
	}

	baseDSN := envOr("FIBERHOUSE_MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/?charset=utf8mb4&parseTime=true&loc=Local&timeout=10s")
	cfg, err := gomysql.ParseDSN(baseDSN)
	if err != nil {
		t.Fatalf("invalid FIBERHOUSE_MYSQL_DSN: %v", err)
	}
	cfg.DBName = ""
	admin, err := gorm.Open(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("MySQL unavailable at FIBERHOUSE_MYSQL_DSN: %v", err)
	}
	if err := admin.WithContext(ctx).Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatalf("create isolated integration database: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if dropErr := admin.WithContext(cleanupCtx).Exec("DROP DATABASE `" + databaseName + "`").Error; dropErr != nil {
			t.Errorf("drop isolated integration database %q: %v", databaseName, dropErr)
		}
		sqlDB, _ := admin.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	cfg.DBName = databaseName
	db, err := gorm.Open(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	app := service.NewExampleMysqlService(repository.NewExampleRepository(model.NewExampleMysqlModelWithDB(db)))
	if err := app.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	created, err := app.Create(ctx, service.CreateInput{Name: "integration_" + runID, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Get(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := app.List(ctx, service.ListInput{Page: 1, PageSize: 10, Status: "active"})
	if err != nil || listed.Total != 1 {
		t.Fatalf("list result = %#v, err = %v", listed, err)
	}
	archived := "archived"
	updated, err := app.Update(ctx, created.ID, service.UpdateInput{Status: &archived})
	if err != nil || updated.Status != archived {
		t.Fatalf("update result = %#v, err = %v", updated, err)
	}
	updated, err = app.Update(ctx, created.ID, service.UpdateInput{Status: &archived})
	if err != nil || updated.Status != archived {
		t.Fatalf("same-value update result = %#v, err = %v", updated, err)
	}
	if err := app.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
