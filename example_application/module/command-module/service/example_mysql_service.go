package service

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
	"gorm.io/gorm"
	"time"
)

type ExampleMysqlService struct {
	fiberhouse.ServiceLocator
	Model *model.ExampleMysqlModel
}

func NewExampleMysqlService(ctx fiberhouse.ContextCommander, m *model.ExampleMysqlModel) *ExampleMysqlService {
	return &ExampleMysqlService{
		ServiceLocator: fiberhouse.NewService(ctx).SetName(GetKeyExampleMysqlService()),
		Model:          m,
	}
}

// GetKeyExampleMysqlService 定义和获取 ExampleMysqlService 注册到全局管理器的实例key
func GetKeyExampleMysqlService(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleMysqlService", fiberhouse.GetNamespace([]string{"command-module"}, ns...)...)
}

// RegisterKeyExampleMysqlService 注册 ExampleMysqlService 到全局管理器，并返回注册实例key
func RegisterKeyExampleMysqlService(ctx fiberhouse.ContextCommander, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleMysqlService(ns...), func() (interface{}, error) {
		// 示例: 推荐命令应用中使用依赖注入的方式初始化服务对象
		var (
			zero *ExampleMysqlService
			wrap = component.NewWrap[*ExampleMysqlService]()
		)
		dc := ctx.GetDigContainer().Provide(func() fiberhouse.ContextCommander { return ctx }).
			Provide(model.NewExampleMysqlModel).
			Provide(NewExampleMysqlService)
		// 检查依赖注入是否成功
		if dc.GetErrorCount() > 0 {
			return zero, fmt.Errorf("ExampleMysqlService RegisterKeyExampleMysqlService error: %v", dc.GetProvideErrs())
		}
		// 解析实例
		err := component.Invoke[*ExampleMysqlService](wrap)
		return wrap.Get(), err
	})
}

// AutoMigrate 自动创建数据库表结构
func (s *ExampleMysqlService) AutoMigrate() error {
	return s.Model.AutoMigrate()
}

// TestOrm 测试ORM
func (s *ExampleMysqlService) TestOrm(ctx fiberhouse.ContextCommander, op string, id uint) error {
	// op: c-创建用户，r-查询用户，u-更新用户，d-删除用户

	ctxWithTimeout := context.WithValue(context.Background(), "test", "orm")

	switch op {
	case "c":
		// 创建用户
		user := &entity.User{
			Name: fmt.Sprintf("TestUser_%d", time.Now().UnixNano()%10000),
			Age:  uint8(18 + time.Now().UnixNano()%43),
			Desc: sql.NullString{
				String: "This is a test user",
				Valid:  true,
			},
		}

		err := s.Model.CreateUser(ctxWithTimeout, user)
		if err != nil {
			s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
				Err(err).Msg("TestOrm create user failed")
			return err
		}

		s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
			Uint("userId", user.ID).Str("name", user.Name).Msg("TestOrm create user success")

	case "r":
		// 根据ID查询用户
		user, err := s.Model.GetUserByID(ctxWithTimeout, id)
		if err != nil {
			s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
				Err(err).Msg("TestOrm get user by id failed")
			return err
		}

		s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
			Uint("id", user.ID).Str("name", user.Name).Uint8("age", user.Age).
			Msg("TestOrm get user by id success")

	case "u":
		// 更新用户
		// 1. 使用map更新
		updates := map[string]interface{}{
			"name": "updated testUser with map",
			"age":  26,
			"desc": "new update",
		}

		err := s.Model.UpdateUser(ctxWithTimeout, id, updates)
		if err != nil {
			s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
				Err(err).Msg("TestOrm update user failed")
			return err
		}

		s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
			Interface("updates", updates).Msg("TestOrm update user success")

		// 2. 使用结构体更新
		user := &entity.User{
			Model: gorm.Model{ID: id},
			Name:  "updated testUser with struct",
			Age:   27,
			Desc: sql.NullString{
				String: "will",
				Valid:  true,
			},
		}

		err = s.Model.UpdateUserStruct(ctxWithTimeout, user)
		if err != nil {
			s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
				Err(err).Msg("TestOrm update user struct failed")
			return err
		}

		s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
			Str("name", user.Name).Uint8("age", user.Age).Msg("TestOrm update user struct success")

	case "d":
		// 删除用户（软删除）
		err := s.Model.DeleteUser(ctxWithTimeout, id)
		if err != nil {
			s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
				Err(err).Msg("TestOrm delete user failed")
			return err
		}

		s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
			Uint("id", 1).Msg("TestOrm delete user success")
	default:
		return fmt.Errorf("TestOrm unknown op: %s", op)
	}
	return nil
}

// TestOk 测试服务是否可用
func (s *ExampleMysqlService) TestOk() string {
	return "ExampleMysqlService.TestOK: OK"
}

// =========   CURD  ================================

// CreateUser 创建用户
func (s *ExampleMysqlService) CreateUser(ctx context.Context, user *entity.User) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Str("name", user.Name).Uint8("age", user.Age).Msg("CreateUser start")

	err := s.Model.CreateUser(ctx, user)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Msg("CreateUser failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", user.ID).Msg("CreateUser success")
	return nil
}

// GetUserByID 根据ID获取用户
func (s *ExampleMysqlService) GetUserByID(ctx context.Context, id uint) (*entity.User, error) {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Msg("GetUserByID start")

	user, err := s.Model.GetUserByID(ctx, id)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Uint("id", id).Msg("GetUserByID failed")
		return nil, err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Str("name", user.Name).Msg("GetUserByID success")
	return user, nil
}

// GetUsersByName 根据名称获取用户列表
func (s *ExampleMysqlService) GetUsersByName(ctx context.Context, name string) ([]entity.User, error) {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Str("name", name).Msg("GetUsersByName start")

	users, err := s.Model.GetUsersByName(ctx, name)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Str("name", name).Msg("GetUsersByName failed")
		return nil, err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Str("name", name).Int("count", len(users)).Msg("GetUsersByName success")
	return users, nil
}

// ListUsers 分页查询用户列表
func (s *ExampleMysqlService) ListUsers(ctx context.Context, page, size int, nameLike string) ([]entity.User, int64, error) {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Int("page", page).Int("size", size).Str("nameLike", nameLike).Msg("ListUsers start")

	users, total, err := s.Model.ListUsers(ctx, page, size, nameLike)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Int("page", page).Int("size", size).Msg("ListUsers failed")
		return nil, 0, err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Int("page", page).Int("size", size).Int("count", len(users)).Int64("total", total).Msg("ListUsers success")
	return users, total, nil
}

// UpdateUser 更新用户信息
func (s *ExampleMysqlService) UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Interface("updates", updates).Msg("UpdateUser start")

	err := s.Model.UpdateUser(ctx, id, updates)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Uint("id", id).Msg("UpdateUser failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Msg("UpdateUser success")
	return nil
}

// UpdateUserStruct 通过结构体更新用户
func (s *ExampleMysqlService) UpdateUserStruct(ctx context.Context, user *entity.User) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", user.ID).Str("name", user.Name).Msg("UpdateUserStruct start")

	err := s.Model.UpdateUserStruct(ctx, user)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Uint("id", user.ID).Msg("UpdateUserStruct failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", user.ID).Msg("UpdateUserStruct success")
	return nil
}

// DeleteUser 软删除用户
func (s *ExampleMysqlService) DeleteUser(ctx context.Context, id uint) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Msg("DeleteUser start")

	err := s.Model.DeleteUser(ctx, id)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Uint("id", id).Msg("DeleteUser failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Msg("DeleteUser success")
	return nil
}

// HardDeleteUser 硬删除用户
func (s *ExampleMysqlService) HardDeleteUser(ctx context.Context, id uint) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Msg("HardDeleteUser start")

	err := s.Model.HardDeleteUser(ctx, id)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Uint("id", id).Msg("HardDeleteUser failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("id", id).Msg("HardDeleteUser success")
	return nil
}

// ==================== Batch Operations ====================

// BatchCreateUsers 批量创建用户
func (s *ExampleMysqlService) BatchCreateUsers(ctx context.Context, users []entity.User) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Int("count", len(users)).Msg("BatchCreateUsers start")

	if len(users) == 0 {
		return nil
	}

	err := s.Model.BatchCreateUsers(ctx, users)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Int("count", len(users)).Msg("BatchCreateUsers failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Int("count", len(users)).Msg("BatchCreateUsers success")
	return nil
}

// BatchDeleteUsers 批量删除用户
func (s *ExampleMysqlService) BatchDeleteUsers(ctx context.Context, ids []uint) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Interface("ids", ids).Msg("BatchDeleteUsers start")

	if len(ids) == 0 {
		return nil
	}

	err := s.Model.BatchDeleteUsers(ctx, ids)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Interface("ids", ids).Msg("BatchDeleteUsers failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Interface("ids", ids).Msg("BatchDeleteUsers success")
	return nil
}

// ==================== Transaction Operations ====================

// CreateUserWithClasses 创建用户并关联班级（事务）
func (s *ExampleMysqlService) CreateUserWithClasses(ctx context.Context, user *entity.User, classes []entity.Class) error {
	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Str("userName", user.Name).Int("classCount", len(classes)).Msg("CreateUserWithClasses start")

	err := s.Model.CreateUserWithClasses(ctx, user, classes)
	if err != nil {
		s.GetContext().GetLogger().Error(s.GetContext().GetConfig().LogOriginCMD()).
			Err(err).Str("userName", user.Name).Msg("CreateUserWithClasses failed")
		return err
	}

	s.GetContext().GetLogger().Info(s.GetContext().GetConfig().LogOriginCMD()).
		Uint("userId", user.ID).Int("classCount", len(classes)).Msg("CreateUserWithClasses success")
	return nil
}
