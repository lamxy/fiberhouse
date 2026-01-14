package model

import (
	"context"
	"errors"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/database/dbmysql"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"gorm.io/gorm"
	"time"
)

type ExampleMysqlModel struct {
	dbmysql.MysqlLocator
	Ctx fiberhouse.IContext
}

// NewExampleMysqlModel 构造函数
func NewExampleMysqlModel(ctx fiberhouse.ICommandContext) *ExampleMysqlModel {
	return &ExampleMysqlModel{
		MysqlLocator: dbmysql.NewMysqlModel(ctx, constant.MysqlInstanceKey).SetDbName("test").SetTable("user").SetName("MysqlModel").(dbmysql.MysqlLocator),
		Ctx:          ctx,
	}
}

// AutoMigrate 自动创建数据表结构
func (m *ExampleMysqlModel) AutoMigrate() error {
	// 创建test数据库（如果不存在）
	err := m.GetDB().Client.Exec("CREATE DATABASE IF NOT EXISTS test").Error
	if err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).Err(err).Msg("create database error.")
		return err
	}

	err = m.GetDB().Client.AutoMigrate(&entity.User{})
	if err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).Err(err).Msg("db.AutoMigrate error.")
		return err
	}
	err = m.GetDB().Client.AutoMigrate(&entity.Class{})
	if err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).Err(err).Msg("db.AutoMigrate error.")
		return err
	}
	return nil
}

/*func (m *ExampleMysqlModel) GetUser() (*entity.User, error) {
	db := m.GetDB().Client
	var (
		user entity.User
	)

	if err := db.Where("name like ?", "%tom%").Select([]string{"name", "age"}).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).Err(err).Msg("db.First not found!")
			return nil, err
		}
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).Err(err).Str("errType", reflect.TypeOf(err).String()).Msg("mysql error.")
		return nil, err
	}

	return &user, nil
}*/

// ==================== User CRUD Methods ====================

// CreateUser 创建用户
func (m *ExampleMysqlModel) CreateUser(ctx context.Context, user *entity.User) error {
	db := m.GetDB().Client.WithContext(ctx)
	user.UpdatedTs = time.Now().UnixMilli()

	if err := db.Create(user).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Msg("CreateUser error")
		return err
	}
	return nil
}

// GetUserByID 根据ID获取用户
func (m *ExampleMysqlModel) GetUserByID(ctx context.Context, id uint) (*entity.User, error) {
	db := m.GetDB().Client.WithContext(ctx)
	var user entity.User

	if err := db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			m.GetContext().GetLogger().Warn(m.Ctx.GetConfig().LogOriginCMD()).
				Uint("id", id).Msg("GetUserByID not found")
			return nil, err
		}
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Uint("id", id).Msg("GetUserByID error")
		return nil, err
	}
	return &user, nil
}

// GetUsersByName 根据名称获取用户列表
func (m *ExampleMysqlModel) GetUsersByName(ctx context.Context, name string) ([]entity.User, error) {
	db := m.GetDB().Client.WithContext(ctx)
	var users []entity.User

	if err := db.Where("name = ?", name).Find(&users).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Str("name", name).Msg("GetUsersByName error")
		return nil, err
	}
	return users, nil
}

// ListUsers 分页查询用户列表
func (m *ExampleMysqlModel) ListUsers(ctx context.Context, page, size int, nameLike string) ([]entity.User, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	db := m.GetDB().Client.WithContext(ctx)
	var users []entity.User
	var total int64

	query := db.Model(&entity.User{})
	if nameLike != "" {
		query = query.Where("name LIKE ?", "%"+nameLike+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Msg("ListUsers count error")
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&users).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Int("page", page).Int("size", size).Msg("ListUsers query error")
		return nil, 0, err
	}

	return users, total, nil
}

// UpdateUser 更新用户信息
func (m *ExampleMysqlModel) UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// 自动更新时间戳
	updates["updated_ts"] = time.Now().UnixMilli()

	db := m.GetDB().Client.WithContext(ctx)
	result := db.Model(&entity.User{}).Where("id = ?", id).Updates(updates)

	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Uint("id", id).Msg("UpdateUser error")
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// UpdateUserStruct 通过结构体更新用户
func (m *ExampleMysqlModel) UpdateUserStruct(ctx context.Context, user *entity.User) error {
	user.UpdatedTs = time.Now().UnixMilli()

	db := m.GetDB().Client.WithContext(ctx)
	result := db.Save(user)

	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Uint("id", user.ID).Msg("UpdateUserStruct error")
		return result.Error
	}

	return nil
}

// DeleteUser 软删除用户
func (m *ExampleMysqlModel) DeleteUser(ctx context.Context, id uint) error {
	db := m.GetDB().Client.WithContext(ctx)

	result := db.Delete(&entity.User{}, id)
	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Uint("id", id).Msg("DeleteUser error")
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// HardDeleteUser 硬删除用户
func (m *ExampleMysqlModel) HardDeleteUser(ctx context.Context, id uint) error {
	db := m.GetDB().Client.WithContext(ctx)

	result := db.Unscoped().Delete(&entity.User{}, id)
	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Uint("id", id).Msg("HardDeleteUser error")
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// ==================== Class CRUD Methods ====================

// CreateClass 创建班级
func (m *ExampleMysqlModel) CreateClass(ctx context.Context, class *entity.Class) error {
	db := m.GetDB().Client.WithContext(ctx)

	if err := db.Create(class).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Msg("CreateClass error")
		return err
	}
	return nil
}

// GetClassByID 根据ID获取班级
func (m *ExampleMysqlModel) GetClassByID(ctx context.Context, id uint) (*entity.Class, error) {
	db := m.GetDB().Client.WithContext(ctx)
	var class entity.Class

	if err := db.First(&class, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			m.GetContext().GetLogger().Warn(m.Ctx.GetConfig().LogOriginCMD()).
				Uint("id", id).Msg("GetClassByID not found")
			return nil, err
		}
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Uint("id", id).Msg("GetClassByID error")
		return nil, err
	}
	return &class, nil
}

// GetClassesByUserID 根据用户ID获取班级列表
func (m *ExampleMysqlModel) GetClassesByUserID(ctx context.Context, userID uint64) ([]entity.Class, error) {
	db := m.GetDB().Client.WithContext(ctx)
	var classes []entity.Class

	if err := db.Where("user_id = ?", userID).Find(&classes).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Uint64("user_id", userID).Msg("GetClassesByUserID error")
		return nil, err
	}
	return classes, nil
}

// ListClasses 分页查询班级列表
func (m *ExampleMysqlModel) ListClasses(ctx context.Context, page, size int, nameLike string) ([]entity.Class, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	db := m.GetDB().Client.WithContext(ctx)
	var classes []entity.Class
	var total int64

	query := db.Model(&entity.Class{})
	if nameLike != "" {
		query = query.Where("name LIKE ?", "%"+nameLike+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Msg("ListClasses count error")
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&classes).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Int("page", page).Int("size", size).Msg("ListClasses query error")
		return nil, 0, err
	}

	return classes, total, nil
}

// UpdateClass 更新班级信息
func (m *ExampleMysqlModel) UpdateClass(ctx context.Context, id uint, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	db := m.GetDB().Client.WithContext(ctx)
	result := db.Model(&entity.Class{}).Where("id = ?", id).Updates(updates)

	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Uint("id", id).Msg("UpdateClass error")
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// DeleteClass 软删除班级
func (m *ExampleMysqlModel) DeleteClass(ctx context.Context, id uint) error {
	db := m.GetDB().Client.WithContext(ctx)

	result := db.Delete(&entity.Class{}, id)
	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Uint("id", id).Msg("DeleteClass error")
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// ==================== 批量操作方法 ====================

// BatchCreateUsers 批量创建用户
func (m *ExampleMysqlModel) BatchCreateUsers(ctx context.Context, users []entity.User) error {
	if len(users) == 0 {
		return nil
	}

	db := m.GetDB().Client.WithContext(ctx)
	now := time.Now().UnixMilli()

	// 设置更新时间
	for i := range users {
		users[i].UpdatedTs = now
	}

	if err := db.CreateInBatches(users, 100).Error; err != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(err).Int("count", len(users)).Msg("BatchCreateUsers error")
		return err
	}

	return nil
}

// BatchDeleteUsers 批量删除用户
func (m *ExampleMysqlModel) BatchDeleteUsers(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	db := m.GetDB().Client.WithContext(ctx)

	result := db.Delete(&entity.User{}, ids)
	if result.Error != nil {
		m.GetContext().GetLogger().Error(m.Ctx.GetConfig().LogOriginCMD()).
			Err(result.Error).Interface("ids", ids).Msg("BatchDeleteUsers error")
		return result.Error
	}

	return nil
}

// ==================== 事务操作方法 ====================

// CreateUserWithClasses 创建用户并关联班级（事务）
func (m *ExampleMysqlModel) CreateUserWithClasses(ctx context.Context, user *entity.User, classes []entity.Class) error {
	db := m.GetDB().Client.WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// 创建用户
		user.UpdatedTs = time.Now().UnixMilli()
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// 为班级设置用户ID
		for i := range classes {
			classes[i].UserId = uint64(user.ID)
		}

		// 批量创建班级
		if len(classes) > 0 {
			if err := tx.CreateInBatches(classes, 100).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
