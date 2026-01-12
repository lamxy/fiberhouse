package model

import (
	"context"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/database/dbmongo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/utils"
	"github.com/lamxy/fiberhouse/exception"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ExampleModel Example模型，继承MongoLocator定位器接口，具备获取上下文、配置、日志、注册实例等功能 以及基本的mongodb操作能力
type ExampleModel struct {
	dbmongo.MongoLocator
	ctx context.Context // 可选属性
}

func NewExampleModel(ctx fiberhouse.IApplicationContext) *ExampleModel {
	return &ExampleModel{
		MongoLocator: dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).SetDbName(constant.DbNameMongo).SetTable(constant.CollExample).
			SetName(GetKeyExampleModel()).(dbmongo.MongoLocator), // 设置当前模型的配置项名(mongodb)和库名(test)
		ctx: context.Background(),
	}
}

// GetKeyExampleModel 获取模型注册key
func GetKeyExampleModel(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleModel", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleModel 注册模型到容器（延迟初始化）并返回注册key
func RegisterKeyExampleModel(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleModel(ns...), func() (interface{}, error) {
		return NewExampleModel(ctx), nil
	})
}

// GetExampleByID 根据ID获取样例文档
func (m *ExampleModel) GetExampleByID(ctx context.Context, oid string) (*entity.Example, error) {
	_id, err := bson.ObjectIDFromHex(oid)
	if err != nil {
		exception.GetInputError().RespData(err.Error()).Panic()
	}
	filter := bson.D{{"_id", _id}}
	opts := options.FindOne().SetProjection(bson.M{
		"_id":     0,
		"profile": 0,
	})
	var example entity.Example
	err = m.GetCollection(m.GetColl()).FindOne(ctx, filter, opts).Decode(&example)
	if err != nil {
		return nil, err
	}
	return &example, nil
}

// GetExamples 获取样例文档列表
func (m *ExampleModel) GetExamples(ctx context.Context, page, size int) ([]entity.Example, error) {
	var examples []entity.Example
	skip, limit := utils.PageParams(int64(page), int64(size))
	filter := bson.D{}
	opts := options.Find().SetProjection(bson.M{
		"profile": 0,
		"courses": 0,
	}).SetSkip(skip).SetLimit(limit).SetSort(bson.M{"created_at": -1})
	cursor, err := m.GetCollection(m.GetColl()).Find(m.ctx, filter, opts)
	defer func() {
		_ = cursor.Close(ctx)
	}()
	if err != nil {
		return nil, exception.GetInternalError().RespData(err)
	}
	if errCur := cursor.All(m.ctx, &examples); errCur != nil {
		return nil, errCur
	}
	return examples, nil
}

// SaveExample 创建样例文档
func (m *ExampleModel) SaveExample(ctx context.Context, doc *entity.Example) (bson.ObjectID, error) {
	var (
		result *mongo.InsertOneResult
		err    error
	)

	result, err = m.MongoLocator.GetCollection(m.GetColl()).InsertOne(ctx, doc)
	if err != nil {
		return bson.NilObjectID, err
	}

	if !result.Acknowledged {
		return bson.NilObjectID, exception.GetInternalError().RespData("Insert not acknowledged")
	}

	return result.InsertedID.(bson.ObjectID), nil
}

// SaveMany 创建多个样例文档
func (m *ExampleModel) SaveMany(ctx context.Context, docs []interface{}) ([]interface{}, error) {
	var (
		result *mongo.InsertManyResult
		err    error
	)
	result, err = m.GetCollection(m.GetColl()).InsertMany(ctx, docs)
	if err != nil {
		return nil, err
	}
	return result.InsertedIDs, err
}

// UpdateExample 更新样例文档
func (m *ExampleModel) UpdateExample(ctx context.Context, upExample *entity.Example) (bool, error) {
	filter := bson.D{{"_id", upExample.ID}}
	update := bson.D{{"$set", upExample}}
	opts := options.UpdateOne().SetUpsert(true)
	var (
		result *mongo.UpdateResult
		err    error
	)
	result, err = m.GetCollection(m.GetColl()).UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return false, err
	}
	if result.MatchedCount > 0 {
		return true, nil // 更新成功
	}
	if result.UpsertedCount > 0 {
		return true, nil // 插入成功
	}
	return false, nil // 未更新也未插入
}

// DeleteExample 删除样例文档
func (m *ExampleModel) DeleteExample(ctx context.Context, id bson.ObjectID) (bool, error) {
	filter := bson.D{{"_id", id}}
	var (
		result *mongo.DeleteResult
		err    error
	)
	result, err = m.GetCollection(m.GetColl()).DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}
