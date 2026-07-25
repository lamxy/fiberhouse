package model

import (
	"context"
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type fakeDecoder struct{ err error }

func (d fakeDecoder) Decode(any) error { return d.err }

type fakeCursor struct {
	allCtx   context.Context
	closeCtx context.Context
}

func (c *fakeCursor) All(ctx context.Context, target any) error {
	c.allCtx = ctx
	return nil
}
func (c *fakeCursor) Close(ctx context.Context) error {
	c.closeCtx = ctx
	return nil
}

type fakeCollection struct {
	findCtx  context.Context
	countCtx context.Context
	cursor   *fakeCursor
	findErr  error
	indexes  []mongo.IndexModel
}

func (c *fakeCollection) CreateIndexes(_ context.Context, indexes []mongo.IndexModel) error {
	c.indexes = indexes
	return nil
}
func (*fakeCollection) InsertOne(context.Context, any) (*mongo.InsertOneResult, error) {
	return nil, nil
}
func (*fakeCollection) FindOne(context.Context, any, ...options.Lister[options.FindOneOptions]) resultDecoder {
	return fakeDecoder{}
}
func (c *fakeCollection) Find(ctx context.Context, _ any, _ ...options.Lister[options.FindOptions]) (exampleCursor, error) {
	c.findCtx = ctx
	if c.findErr != nil {
		return nil, c.findErr
	}
	return c.cursor, nil
}
func (c *fakeCollection) CountDocuments(ctx context.Context, _ any, _ ...options.Lister[options.CountOptions]) (int64, error) {
	c.countCtx = ctx
	return 0, nil
}
func (*fakeCollection) UpdateOne(context.Context, any, any, ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return nil, nil
}
func (*fakeCollection) DeleteOne(context.Context, any, ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return nil, nil
}

func TestExampleFilterBuildsStableStatusQueryAndPagination(t *testing.T) {
	filter, skip, limit, sort := buildFindQuery(ExampleFilter{
		Page: 2, PageSize: 25, Status: entity.ExampleStatusArchived,
	})
	if documentMap(filter)["status"] != entity.ExampleStatusArchived {
		t.Fatalf("filter = %#v", filter)
	}
	if skip != 25 || limit != 25 {
		t.Fatalf("skip/limit = %d/%d", skip, limit)
	}
	wantSort := bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}
	if !bsonEqual(sort, wantSort) {
		t.Fatalf("sort = %#v, want %#v", sort, wantSort)
	}
}

func TestReplacementDocumentNeverChangesIdentityOrCreatedAt(t *testing.T) {
	id := bson.NewObjectID()
	update := buildUpdate(id, &entity.Example{
		ID: id, Name: "name", Description: "description",
		Status: entity.ExampleStatusActive, Tags: []string{"tag"},
	})
	set := documentMap(documentMap(update)["$set"].(bson.D))
	if _, ok := set["_id"]; ok {
		t.Fatal("$set must not contain _id")
	}
	if _, ok := set["created_at"]; ok {
		t.Fatal("$set must not contain created_at")
	}
	for _, key := range []string{"name", "description", "status", "tags", "updated_at"} {
		if _, ok := set[key]; !ok {
			t.Fatalf("$set missing %q: %#v", key, set)
		}
	}
}

func TestFindUsesCallerContextForEveryMongoOperation(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	cursor := &fakeCursor{}
	collection := &fakeCollection{cursor: cursor}
	m := &ExampleModel{collection: collection}

	if _, _, err := m.Find(ctx, ExampleFilter{Page: 1, PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if collection.findCtx != ctx || collection.countCtx != ctx ||
		cursor.allCtx != ctx || cursor.closeCtx != ctx {
		t.Fatalf("contexts: find=%p all=%p count=%p close=%p want=%p",
			collection.findCtx, cursor.allCtx, collection.countCtx, cursor.closeCtx, ctx)
	}
}

func TestFindReturnsCollectionErrorBeforeCursorCleanup(t *testing.T) {
	sentinel := errors.New("find failed")
	m := &ExampleModel{collection: &fakeCollection{findErr: sentinel}}
	if _, _, err := m.Find(context.Background(), ExampleFilter{}); !errors.Is(err, sentinel) {
		t.Fatalf("Find error = %v, want sentinel", err)
	}
}

func TestEnsureIndexesCreatesUniqueNameAndStableListIndexes(t *testing.T) {
	collection := &fakeCollection{}
	m := &ExampleModel{collection: collection}
	if err := m.EnsureIndexes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(collection.indexes) != 2 {
		t.Fatalf("indexes = %d, want 2", len(collection.indexes))
	}
	wantName := bson.D{{Key: "name", Value: 1}}
	wantList := bson.D{
		{Key: "status", Value: 1},
		{Key: "created_at", Value: -1},
		{Key: "_id", Value: -1},
	}
	if !bsonEqual(collection.indexes[0].Keys.(bson.D), wantName) ||
		!bsonEqual(collection.indexes[1].Keys.(bson.D), wantList) {
		t.Fatalf("index keys = %#v", collection.indexes)
	}
	indexOptions := &options.IndexOptions{}
	for _, setter := range collection.indexes[0].Options.List() {
		if err := setter(indexOptions); err != nil {
			t.Fatal(err)
		}
	}
	if indexOptions.Unique == nil || !*indexOptions.Unique {
		t.Fatal("name index must be unique")
	}
}

func bsonEqual(a, b bson.D) bool {
	aa, _ := bson.Marshal(a)
	bb, _ := bson.Marshal(b)
	return string(aa) == string(bb)
}

func documentMap(doc bson.D) map[string]any {
	result := make(map[string]any, len(doc))
	for _, element := range doc {
		result[element.Key] = element.Value
	}
	return result
}
