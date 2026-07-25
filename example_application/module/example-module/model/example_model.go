// Package model 是 example 模块的存储层：负责 MongoDB 集合、索引定义以及
// 查询/更新文档的构造，返回驱动原生的错误与面向存储的类型（bson.ObjectID），
// 不做任何翻译。它仅依赖驱动（component/database/dbmongo、mongo-driver）与
// entity；上下文处理与错误翻译由 repository 中的调用方负责——本层不得自行
// 替换上下文。
package model

import (
	"context"
	"fmt"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/database/dbmongo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ExampleFilter 承载 ExampleModel.Find 的分页与状态过滤参数。
type ExampleFilter struct {
	Page     int
	PageSize int
	Status   entity.ExampleStatus
}

type resultDecoder interface {
	Decode(any) error
}

type exampleCursor interface {
	All(context.Context, any) error
	Close(context.Context) error
}

type exampleCollection interface {
	CreateIndexes(context.Context, []mongo.IndexModel) error
	InsertOne(context.Context, any) (*mongo.InsertOneResult, error)
	FindOne(context.Context, any, ...options.Lister[options.FindOneOptions]) resultDecoder
	Find(context.Context, any, ...options.Lister[options.FindOptions]) (exampleCursor, error)
	CountDocuments(context.Context, any, ...options.Lister[options.CountOptions]) (int64, error)
	UpdateOne(context.Context, any, any, ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error)
	DeleteOne(context.Context, any, ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error)
}

type mongoCollection struct {
	collection *mongo.Collection
}

func (c mongoCollection) CreateIndexes(ctx context.Context, indexes []mongo.IndexModel) error {
	_, err := c.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
func (c mongoCollection) InsertOne(ctx context.Context, doc any) (*mongo.InsertOneResult, error) {
	return c.collection.InsertOne(ctx, doc)
}
func (c mongoCollection) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) resultDecoder {
	return c.collection.FindOne(ctx, filter, opts...)
}
func (c mongoCollection) Find(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) (exampleCursor, error) {
	return c.collection.Find(ctx, filter, opts...)
}
func (c mongoCollection) CountDocuments(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return c.collection.CountDocuments(ctx, filter, opts...)
}
func (c mongoCollection) UpdateOne(ctx context.Context, filter, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return c.collection.UpdateOne(ctx, filter, update, opts...)
}
func (c mongoCollection) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return c.collection.DeleteOne(ctx, filter, opts...)
}

// ExampleModel 是 example 集合基于 MongoDB 的存储实现，实现了
// repository.ExampleModelStore。每个方法只使用调用方（repository）传入的
// context.Context——绝不能替换为自己的上下文（如 context.Background()），
// 否则会悄然破坏请求的取消/超时/链路追踪传播。
type ExampleModel struct {
	dbmongo.MongoLocator // 继承 model 定位层接口
	collection           exampleCollection
}

// NewExampleModel 构建一个 ExampleModel，绑定到从 ctx 解析出的、为 example
// 模块配置的 MongoDB 集合。
func NewExampleModel(ctx fiberhouse.IApplicationContext) *ExampleModel {
	locator := dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).
		SetDbName(constant.DbNameMongo).
		SetTable(constant.CollExample).
		SetName(GetKeyExampleModel()).(dbmongo.MongoLocator)
	model := NewExampleModelWithCollection(locator.GetCollection(locator.GetColl()))
	model.MongoLocator = locator
	return model
}

// NewExampleModelWithCollection 保持生产环境的持久化行为，同时允许调用方
// 选用一个已配置好的 MongoDB 集合。
func NewExampleModelWithCollection(collection *mongo.Collection) *ExampleModel {
	return &ExampleModel{
		collection: mongoCollection{collection: collection},
	}
}

// GetKeyExampleModel 返回用于定位 ExampleModel 实例的注册键，
// 可通过 ns 追加命名空间。
func GetKeyExampleModel(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleModel", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleModel 注册一个惰性初始化器（通过 NewExampleModel 构建
// ExampleModel），并返回其注册键。
func RegisterKeyExampleModel(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleModel(ns...), func() (interface{}, error) {
		return NewExampleModel(ctx), nil
	})
}

// EnsureIndexes 创建集合所需的索引：name 上的唯一索引（对应 repository 将重复键
// 错误翻译成的 ErrConflict 语义），以及支撑 List 确定性排序的
// status/created_at/_id 复合索引。可安全地重复调用。
func (m *ExampleModel) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetName("example_name_unique").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName("example_status_created_id"),
		},
	}
	return m.collection.CreateIndexes(ctx, indexes)
}

// Insert 持久化 example 并返回其生成的 ObjectID。当写入未被确认，或插入的 id
// 不是预期类型时，返回错误。
func (m *ExampleModel) Insert(ctx context.Context, example *entity.Example) (bson.ObjectID, error) {
	result, err := m.collection.InsertOne(ctx, example)
	if err != nil {
		return bson.NilObjectID, err
	}
	if !result.Acknowledged {
		return bson.NilObjectID, fmt.Errorf("insert example: not acknowledged")
	}
	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return bson.NilObjectID, fmt.Errorf("insert example: unexpected id type %T", result.InsertedID)
	}
	return id, nil
}

// FindByID 按 ObjectID 获取单个文档；若无匹配，返回 mongo.ErrNoDocuments
// （不翻译）。
func (m *ExampleModel) FindByID(ctx context.Context, id bson.ObjectID) (*entity.Example, error) {
	var example entity.Example
	err := m.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&example)
	if err != nil {
		return nil, err
	}
	return &example, nil
}

// Find 返回匹配 query 的一页文档以及匹配总数，按 created_at 降序、再按 _id
// 降序进行确定性排序（见 buildFindQuery），使分页在多次调用间保持稳定。
//
// 游标在注册 defer Close 之前打开：这样 Find 出错时可立即返回，不会留下一个 nil
// 游标去关闭；只有成功打开的游标才会被 defer 关闭。
func (m *ExampleModel) Find(ctx context.Context, query ExampleFilter) ([]entity.Example, int64, error) {
	filter, skip, limit, sort := buildFindQuery(query)
	cursor, err := m.collection.Find(ctx, filter, options.Find().SetSkip(skip).SetLimit(limit).SetSort(sort))
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	examples := make([]entity.Example, 0)
	if err := cursor.All(ctx, &examples); err != nil {
		return nil, 0, err
	}
	total, err := m.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return examples, total, nil
}

// buildFindQuery 规范化 page/pageSize（默认 page 1、size 20），并为 Find 构建
// filter、skip、limit 和 sort 文档。排序（created_at 降序、_id 降序）是本模块
// 确定性列表排序的唯一权威来源——此处任何改动都会端到端地改变分页行为。
func buildFindQuery(query ExampleFilter) (bson.D, int64, int64, bson.D) {
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	filter := bson.D{}
	if query.Status != "" {
		filter = append(filter, bson.E{Key: "status", Value: query.Status})
	}
	sort := bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}
	return filter, int64((page - 1) * pageSize), int64(pageSize), sort
}

// Replace 覆盖由 id 标识的文档的可变字段（经 buildUpdate），并报告本次写入是否
// 真的修改了文档。它绝不触碰 created_at，因此原始创建时间在更新间得以保留。
// 返回 false 且 error 为 nil，表示文档存在但本次写入是空操作，repository 会将其
// 呈现为 ErrUnchanged。
func (m *ExampleModel) Replace(ctx context.Context, id bson.ObjectID, example *entity.Example) (bool, error) {
	result, err := m.collection.UpdateOne(
		ctx,
		bson.D{{Key: "_id", Value: id}},
		buildUpdate(id, example),
	)
	if err != nil {
		return false, err
	}
	return result.ModifiedCount > 0, nil
}

// buildUpdate 为 Replace 构建 $set 文档。它刻意省略 created_at，使更新永不覆盖
// 原始创建时间——只有 updated_at 与可变内容字段会变化。
func buildUpdate(_ bson.ObjectID, example *entity.Example) bson.D {
	return bson.D{{Key: "$set", Value: bson.D{
		{Key: "name", Value: example.Name},
		{Key: "description", Value: example.Description},
		{Key: "status", Value: example.Status},
		{Key: "tags", Value: example.Tags},
		{Key: "updated_at", Value: example.UpdatedAt},
	}}}
}

// Delete 删除由 id 标识的文档，并报告是否确实删除了一个文档。
func (m *ExampleModel) Delete(ctx context.Context, id bson.ObjectID) (bool, error) {
	result, err := m.collection.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}
