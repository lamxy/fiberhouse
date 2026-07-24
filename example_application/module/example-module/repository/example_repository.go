// Package repository 是 example 模块的持久化编排层：负责在面向 service 的
// 字符串/领域类型与 model 层面向存储的类型之间转换（例如 hex id <-> bson.ObjectID），
// 将 model 错误归一化为下方定义的稳定哨兵错误，并惰性确保索引存在。它依赖 model
// （并间接依赖数据库驱动），但不向 service 暴露任何 MongoDB 特有类型。
package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ExampleStore 实现返回的稳定领域错误。调用方（service、transport）必须用
// errors.Is 与这些哨兵值比较，而非匹配错误字符串。
var (
	// ErrInvalidID 在任何存储访问之前，当传入的 id 不是格式良好的标识符
	//（例如不是合法的 hex ObjectID）时返回。
	ErrInvalidID = errors.New("invalid example id")
	// ErrNotFound 在查询、更新或删除针对一个不存在的 id 时返回。
	ErrNotFound = errors.New("example not found")
	// ErrConflict 在创建违反唯一性约束时返回。
	ErrConflict = errors.New("example already exists")
	// ErrUnchanged 由 Update 返回，表示目标存在但本次写入未修改任何文档
	//（例如该 patch 是空操作）。
	ErrUnchanged = errors.New("example unchanged")
)

// ExampleModelStore 是 ExampleRepository 所依赖的面向存储的契约。实现方基于
// bson.ObjectID 与原始 entity.Example 值操作，并返回驱动原生错误
// （如 mongo.ErrNoDocuments）；翻译为稳定的
// ErrInvalidID/ErrNotFound/ErrConflict/ErrUnchanged 哨兵错误发生在本包的
// translateModelError 中，而非 model 层。
type ExampleModelStore interface {
	EnsureIndexes(context.Context) error
	Insert(context.Context, *entity.Example) (bson.ObjectID, error)
	FindByID(context.Context, bson.ObjectID) (*entity.Example, error)
	Find(context.Context, model.ExampleFilter) ([]entity.Example, int64, error)
	Replace(context.Context, bson.ObjectID, *entity.Example) (bool, error)
	Delete(context.Context, bson.ObjectID) (bool, error)
}

// ListOptions 承载 ExampleStore.List 的分页与过滤参数。
type ListOptions struct {
	Page     int
	PageSize int
	Status   entity.ExampleStatus
}

// ExampleStore 是由 service 消费的 repository 层契约。所有方法都接收调用方的
// context.Context 并原样传播到 model 层。错误为上文的稳定哨兵值（ErrInvalidID、
// ErrNotFound、ErrConflict、ErrUnchanged）；调用方应使用 errors.Is。
//
// Update 替换已有记录的可变字段，且不是 upsert：若 id 未解析到已有 example，
// 返回 ErrNotFound；若替换确为空操作，返回 ErrUnchanged。
// List 返回给定页的条目与匹配总数，并以确定性顺序排列（委托给 model 层）。
type ExampleStore interface {
	Create(context.Context, *entity.Example) error
	Get(context.Context, string) (*entity.Example, error)
	List(context.Context, ListOptions) ([]entity.Example, int64, error)
	Update(context.Context, string, *entity.Example) error
	Delete(context.Context, string) error
}

// ExampleRepository 是由 ExampleModelStore 支撑的默认 ExampleStore 实现。
// 它在首次使用时惰性确保索引存在，并把 model 层错误翻译为稳定的
// ExampleStore 哨兵错误。
type ExampleRepository struct {
	fiberhouse.RepositoryLocator
	Model ExampleModelStore

	readyMu     sync.Mutex
	initialized bool
}

// NewExampleRepository 构建一个绑定到指定应用上下文与 ExampleModelStore 的
// ExampleRepository。
func NewExampleRepository(ctx fiberhouse.IApplicationContext, store ExampleModelStore) *ExampleRepository {
	return &ExampleRepository{
		RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyExampleRepository()),
		Model:             store,
	}
}

// GetKeyExampleRepository 返回用于定位 ExampleRepository 实例的注册键，
// 可通过 ns 追加命名空间。
func GetKeyExampleRepository(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleRepository 注册一个惰性初始化器（构建由 model.NewExampleModel
// 支撑的 ExampleRepository），并返回其注册键。
func RegisterKeyExampleRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
		return NewExampleRepository(ctx, model.NewExampleModel(ctx)), nil
	})
}

// Create 插入 example，并在成功时设置其生成的 ID。
func (r *ExampleRepository) Create(ctx context.Context, example *entity.Example) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	id, err := r.Model.Insert(ctx, example)
	if err != nil {
		return translateModelError(err)
	}
	example.ID = id
	return nil
}

// Get 按 hex 编码的 id 获取 example；若 rawID 不是合法的 ObjectID hex 字符串，
// 返回 ErrInvalidID；若不存在该 example，返回 ErrNotFound。
func (r *ExampleRepository) Get(ctx context.Context, rawID string) (*entity.Example, error) {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return nil, ErrInvalidID
	}
	if err := r.ready(ctx); err != nil {
		return nil, err
	}
	example, err := r.Model.FindByID(ctx, id)
	if err != nil {
		return nil, translateModelError(err)
	}
	return example, nil
}

// List 以确定性顺序返回匹配 opts.Status（为空则匹配所有状态）的一页 example
// 以及匹配总数。即使没有匹配项，返回的切片也绝不为 nil。
func (r *ExampleRepository) List(ctx context.Context, opts ListOptions) ([]entity.Example, int64, error) {
	if err := r.ready(ctx); err != nil {
		return nil, 0, err
	}
	examples, total, err := r.Model.Find(ctx, model.ExampleFilter{
		Page: opts.Page, PageSize: opts.PageSize, Status: opts.Status,
	})
	if err != nil {
		return nil, 0, translateModelError(err)
	}
	if examples == nil {
		examples = make([]entity.Example, 0)
	}
	return examples, total, nil
}

// Update 替换由 rawID 标识的 example 的可变字段。这不是 upsert：id 格式非法时
// 返回 ErrInvalidID；目标存在但替换未修改任何内容（调用方的 patch 是空操作）时
// 返回 ErrUnchanged；其余情况返回经 translateModelError 翻译的 model 层错误。
// 它不会创建新记录。
func (r *ExampleRepository) Update(ctx context.Context, rawID string, example *entity.Example) error {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ErrInvalidID
	}
	if err := r.ready(ctx); err != nil {
		return err
	}
	changed, err := r.Model.Replace(ctx, id, example)
	if err != nil {
		return translateModelError(err)
	}
	if !changed {
		return ErrUnchanged
	}
	return nil
}

// Delete 删除由 rawID 标识的 example；id 格式非法时返回 ErrInvalidID，
// 未删除任何文档时返回 ErrNotFound。
func (r *ExampleRepository) Delete(ctx context.Context, rawID string) error {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ErrInvalidID
	}
	if err := r.ready(ctx); err != nil {
		return err
	}
	deleted, err := r.Model.Delete(ctx, id)
	if err != nil {
		return translateModelError(err)
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

// ready 惰性地确保索引恰好被创建一次，由 readyMu 守护，避免并发的首次调用
// 在 EnsureIndexes 上产生竞态。
func (r *ExampleRepository) ready(ctx context.Context) error {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()

	if r.initialized {
		return nil
	}
	if err := r.Model.EnsureIndexes(ctx); err != nil {
		return err
	}
	r.initialized = true
	return nil
}

// translateModelError 将 model 层的驱动原生错误映射为稳定的 ExampleStore
// 哨兵错误（ErrNotFound、ErrConflict），其余错误原样透传。
func translateModelError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return ErrConflict
	default:
		return err
	}
}

// CreateExample 在传输层于下一个切片迁移完成之前，临时支撑旧的演示服务。
func (r *ExampleRepository) CreateExample(ctx context.Context, req *requestvo.ExampleReqVo) (string, error) {
	example := &entity.Example{Name: req.ExamName, Status: entity.ExampleStatusActive}
	if err := r.Create(ctx, example); err != nil {
		return "", err
	}
	return example.ID.Hex(), nil
}
