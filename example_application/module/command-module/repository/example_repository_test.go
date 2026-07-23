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

func TestRepositoryPropagatesContextAcrossOperations(t *testing.T) {
	capture := func(contexts *[]context.Context) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			*contexts = append(*contexts, tx.Statement.Context)
		}
	}
	tests := []struct {
		name     string
		register func(*gorm.DB, *[]context.Context)
		run      func(context.Context, *exampleRepository) error
		want     int
	}{
		{
			name: "migrate",
			register: func(db *gorm.DB, contexts *[]context.Context) {
				db.Callback().Raw().Before("gorm:raw").Register("test:context", capture(contexts))
				db.Callback().Row().Before("gorm:row").Register("test:context", capture(contexts))
			},
			run: func(ctx context.Context, repo *exampleRepository) error {
				return repo.Migrate(ctx)
			},
			want: 4,
		},
		{
			name: "create",
			register: func(db *gorm.DB, contexts *[]context.Context) {
				db.Callback().Create().Before("gorm:create").Register("test:context", capture(contexts))
			},
			run: func(ctx context.Context, repo *exampleRepository) error {
				return repo.Create(ctx, &entity.ExampleRecord{Name: "alpha", Status: "active"})
			},
			want: 1,
		},
		{
			name: "get",
			register: func(db *gorm.DB, contexts *[]context.Context) {
				db.Callback().Query().Before("gorm:query").Register("test:context", capture(contexts))
			},
			run: func(ctx context.Context, repo *exampleRepository) error {
				_, err := repo.Get(ctx, 7)
				return err
			},
			want: 1,
		},
		{
			name: "list count and rows",
			register: func(db *gorm.DB, contexts *[]context.Context) {
				db.Callback().Query().Before("gorm:query").Register("test:context", capture(contexts))
			},
			run: func(ctx context.Context, repo *exampleRepository) error {
				_, _, err := repo.List(ctx, ListOptions{Page: 1, PageSize: 20})
				return err
			},
			want: 2,
		},
		{
			name: "update and follow-up get",
			register: func(db *gorm.DB, contexts *[]context.Context) {
				db.Callback().Update().Before("gorm:update").Register("test:context-update", func(tx *gorm.DB) {
					capture(contexts)(tx)
					tx.RowsAffected = 1
				})
				db.Callback().Query().Before("gorm:query").Register("test:context-query", capture(contexts))
			},
			run: func(ctx context.Context, repo *exampleRepository) error {
				name := "updated"
				_, err := repo.Update(ctx, 7, UpdateInput{Name: &name})
				return err
			},
			want: 2,
		},
		{
			name: "delete",
			register: func(db *gorm.DB, contexts *[]context.Context) {
				db.Callback().Delete().Before("gorm:delete").Register("test:context", func(tx *gorm.DB) {
					capture(contexts)(tx)
					tx.RowsAffected = 1
				})
			},
			run: func(ctx context.Context, repo *exampleRepository) error {
				return repo.Delete(ctx, 7)
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, db := dryRunRepository(t)
			ctx := context.WithValue(context.Background(), contextKey("operation"), tt.name)
			var contexts []context.Context
			tt.register(db, &contexts)

			if err := tt.run(ctx, repo); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			if len(contexts) != tt.want {
				t.Fatalf("captured %d contexts, want %d", len(contexts), tt.want)
			}
			for i, captured := range contexts {
				if captured != ctx {
					t.Fatalf("callback %d context did not match caller context", i)
				}
			}
		})
	}
}
