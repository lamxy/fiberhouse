package repository

import (
	"context"
	"errors"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/model"
	"github.com/lamxy/fiberhouse/exception"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

// ExampleRepository Example仓库，负责Example业务的数据持久化操作，继承fiberhouse.RepositoryLocator仓库定位器接口，具备获取上下文、配置、日志、注册实例等功能
type ExampleRepository struct {
	fiberhouse.RepositoryLocator
	Model *model.ExampleModel
}

func NewExampleRepository(ctx fiberhouse.IApplicationContext, m *model.ExampleModel) *ExampleRepository {
	return &ExampleRepository{
		RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyExampleRepository()),
		Model:             m,
	}
}

// GetKeyExampleRepository 获取 ExampleRepository 注册键名
func GetKeyExampleRepository(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleRepository 注册 ExampleRepository 到容器（延迟初始化）并返回注册key；由上层服务层作为依赖组件的属性key使用，既实现了延迟初始化单例，又实现了依赖注入
// 见 api.CommonHandler 示例，引用了 service.TestService 作为依赖组件，并通过 service.RegisterKeyTestService(ctx) 注册依赖组件到容器并返回注册key
func RegisterKeyExampleRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
		m := model.NewExampleModel(ctx)
		return NewExampleRepository(ctx, m), nil
	})
}

// GetExampleById 根据ID获取Example示例数据
func (r *ExampleRepository) GetExampleById(id string) (*entity.Example, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := r.Model.GetExampleByID(ctx, id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, exception.GetNotFoundDocument() // 返回error
		}
		exception.GetInternalError().RespError(err.Error()).Panic() // 直接panic
	}
	return result, nil
}

// CreateExample 创建Example示例数据
func (r *ExampleRepository) CreateExample(req *requestvo.ExampleReqVo) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	example := &entity.Example{
		Name:    req.ExamName,
		Age:     req.ExamAge,
		Courses: req.Courses,
		Profile: req.Profile,
		Timestamps: fields.Timestamps{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	insertID, err := r.Model.SaveExample(ctx, example)
	if err != nil {
		return "", err
	}
	if insertID == bson.NilObjectID {
		return "", exception.GetInternalError().RespError("insert example failed")
	}
	return insertID.Hex(), nil
}

// GetExamples 分页获取Example示例数据
func (r *ExampleRepository) GetExamples(page, size int) ([]entity.Example, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results, err := r.Model.GetExamples(ctx, page, size)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, exception.GetNotFoundDocument() // 返回error
		}
		return nil, err
	}
	return results, nil
}
