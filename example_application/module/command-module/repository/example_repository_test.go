package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type contextKey string

func dryRunRepository(t *testing.T) (*exampleRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{
		DSN:                       "root@tcp(localhost:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		TranslateError:         true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	return NewExampleRepository(model.NewExampleMysqlModelWithDB(db)).(*exampleRepository), db
}

func TestRepositoryTranslatesMySQLDuplicateKeyError(t *testing.T) {
	driverErr := &gomysql.MySQLError{Number: 1062, Message: "Duplicate entry 'alpha'"}
	err := translateError("create example record", driverErr)
	if !errors.Is(err, ErrDuplicate) || !errors.Is(err, driverErr) {
		t.Fatalf("translated error = %v, want stable duplicate error wrapping MySQL cause", err)
	}
}

func TestRepositoryListNormalizesPaginationAndBuildsStableQuery(t *testing.T) {
	repo, db := dryRunRepository(t)
	normalized := normalizeListOptions(ListOptions{Page: -1, PageSize: 0})
	if normalized.Page != 1 || normalized.PageSize != 20 {
		t.Fatalf("normalized options = %#v", normalized)
	}
	var statements []string
	db.Callback().Query().After("gorm:query").Register("test:capture", func(tx *gorm.DB) {
		statements = append(statements, tx.Statement.SQL.String())
	})

	_, _, err := repo.List(context.Background(), ListOptions{Page: 2, PageSize: 5, Status: "active"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("captured %d queries, want count and list", len(statements))
	}
	if !strings.Contains(strings.ToUpper(statements[0]), "COUNT(*)") {
		t.Errorf("count query = %q", statements[0])
	}
	listSQL := statements[1]
	for _, fragment := range []string{"status = ?", "ORDER BY created_at DESC, id DESC", "LIMIT ?", "OFFSET ?"} {
		if !strings.Contains(listSQL, fragment) {
			t.Errorf("list query %q does not contain %q", listSQL, fragment)
		}
	}
}

func TestRepositoryTranslatesDuplicateAndNotFoundErrors(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		repo, db := dryRunRepository(t)
		db.Callback().Create().Before("gorm:create").Register("test:duplicate", func(tx *gorm.DB) {
			tx.AddError(gorm.ErrDuplicatedKey)
		})

		err := repo.Create(context.Background(), &entity.ExampleRecord{Name: "alpha", Status: "active"})
		if !errors.Is(err, ErrDuplicate) || !errors.Is(err, gorm.ErrDuplicatedKey) {
			t.Fatalf("Create() error = %v, want stable duplicate error wrapping driver cause", err)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		repo, db := dryRunRepository(t)
		db.Callback().Query().Before("gorm:query").Register("test:not-found", func(tx *gorm.DB) {
			tx.AddError(gorm.ErrRecordNotFound)
		})

		_, err := repo.Get(context.Background(), 41)
		if !errors.Is(err, ErrNotFound) || !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("Get() error = %v, want stable not-found error wrapping driver cause", err)
		}
	})
}

func TestRepositoryUpdatePreservesExplicitEmptyDescription(t *testing.T) {
	repo, db := dryRunRepository(t)
	var statement string
	db.Callback().Update().After("gorm:update").Register("test:capture", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
		tx.RowsAffected = 1
	})

	empty := ""
	_, err := repo.Update(context.Background(), 7, UpdateInput{Description: &empty})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !strings.Contains(statement, "description") {
		t.Fatalf("update query %q does not update description", statement)
	}
	if strings.Contains(statement, "name") || strings.Contains(statement, "status") {
		t.Fatalf("update query %q includes fields not requested", statement)
	}
}

func TestRepositoryDeleteIsHardDeleteAndPropagatesContext(t *testing.T) {
	repo, db := dryRunRepository(t)
	ctx := context.WithValue(context.Background(), contextKey("trace"), "request-19")
	var captured context.Context
	var unscoped bool
	db.Callback().Delete().Before("gorm:delete").Register("test:capture", func(tx *gorm.DB) {
		captured = tx.Statement.Context
		unscoped = tx.Statement.Unscoped
		tx.RowsAffected = 1
	})

	if err := repo.Delete(ctx, 9); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if captured != ctx {
		t.Fatal("Delete() did not propagate the caller context to GORM")
	}
	if !unscoped {
		t.Fatal("Delete() must issue an explicit hard delete")
	}
}
