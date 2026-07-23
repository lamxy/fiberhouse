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

type ExampleModel struct {
	dbmongo.MongoLocator
	collection exampleCollection
}

func NewExampleModel(ctx fiberhouse.IApplicationContext) *ExampleModel {
	locator := dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).
		SetDbName(constant.DbNameMongo).
		SetTable(constant.CollExample).
		SetName(GetKeyExampleModel()).(dbmongo.MongoLocator)
	return &ExampleModel{
		MongoLocator: locator,
		collection:   mongoCollection{collection: locator.GetCollection(locator.GetColl())},
	}
}

func GetKeyExampleModel(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleModel", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func RegisterKeyExampleModel(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleModel(ns...), func() (interface{}, error) {
		return NewExampleModel(ctx), nil
	})
}

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

func (m *ExampleModel) FindByID(ctx context.Context, id bson.ObjectID) (*entity.Example, error) {
	var example entity.Example
	err := m.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&example)
	if err != nil {
		return nil, err
	}
	return &example, nil
}

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

func buildUpdate(_ bson.ObjectID, example *entity.Example) bson.D {
	return bson.D{{Key: "$set", Value: bson.D{
		{Key: "name", Value: example.Name},
		{Key: "description", Value: example.Description},
		{Key: "status", Value: example.Status},
		{Key: "tags", Value: example.Tags},
		{Key: "updated_at", Value: example.UpdatedAt},
	}}}
}

func (m *ExampleModel) Delete(ctx context.Context, id bson.ObjectID) (bool, error) {
	result, err := m.collection.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}
