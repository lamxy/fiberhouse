package examplemodule_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type integrationMongoStore struct {
	collection *mongo.Collection
}

func (s integrationMongoStore) Create(ctx context.Context, example *entity.Example) error {
	result, err := s.collection.InsertOne(ctx, example)
	if err != nil {
		return err
	}
	example.ID = result.InsertedID.(bson.ObjectID)
	return nil
}

func (s integrationMongoStore) Get(ctx context.Context, id string) (*entity.Example, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, repository.ErrInvalidID
	}
	var example entity.Example
	if err := s.collection.FindOne(ctx, bson.D{{Key: "_id", Value: objectID}}).Decode(&example); err != nil {
		return nil, err
	}
	return &example, nil
}

func (s integrationMongoStore) List(ctx context.Context, list repository.ListOptions) ([]entity.Example, int64, error) {
	filter := bson.D{}
	if list.Status != "" {
		filter = append(filter, bson.E{Key: "status", Value: list.Status})
	}
	total, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := s.collection.Find(ctx, filter, options.Find().
		SetSkip(int64((list.Page-1)*list.PageSize)).
		SetLimit(int64(list.PageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var examples []entity.Example
	if err := cursor.All(ctx, &examples); err != nil {
		return nil, 0, err
	}
	return examples, total, nil
}

func (s integrationMongoStore) Update(ctx context.Context, id string, example *entity.Example) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return repository.ErrInvalidID
	}
	result, err := s.collection.ReplaceOne(ctx, bson.D{{Key: "_id", Value: objectID}}, example)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return repository.ErrUnchanged
	}
	return nil
}

func (s integrationMongoStore) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return repository.ErrInvalidID
	}
	result, err := s.collection.DeleteOne(ctx, bson.D{{Key: "_id", Value: objectID}})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func TestMongoCRUDAndRedisRoundTripIntegration(t *testing.T) {
	requireIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	runID := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	mongoURI := envOr("FIBERHOUSE_MONGODB_URI", "mongodb://admin:admin@127.0.0.1:27037/?authSource=admin")
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatal(err)
	}
	defer mongoClient.Disconnect(context.Background())
	if err := mongoClient.Ping(ctx, nil); err != nil {
		t.Fatalf("MongoDB unavailable at FIBERHOUSE_MONGODB_URI: %v", err)
	}

	collection := mongoClient.Database(envOr("FIBERHOUSE_MONGODB_DATABASE", "test")).
		Collection(envOr("FIBERHOUSE_MONGODB_COLLECTION", "example"))
	store := integrationMongoStore{collection: collection}
	app := &service.ExampleService{Store: store}
	var createdID bson.ObjectID
	t.Cleanup(func() {
		if !createdID.IsZero() {
			_, _ = collection.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: createdID}})
		}
	})

	created, err := app.Create(ctx, requestvo.CreateExampleReqVo{Name: "integration_" + runID, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	createdID, err = bson.ObjectIDFromHex(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Get(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := app.List(ctx, requestvo.ListExamplesReqVo{Page: 1, PageSize: 100, Status: "active"})
	if err != nil || listed.Total < 1 {
		t.Fatalf("list result = %#v, err = %v", listed, err)
	}
	archived := "archived"
	if _, err := app.Update(ctx, created.ID, requestvo.UpdateExampleReqVo{Status: &archived}); err != nil {
		t.Fatal(err)
	}
	if err := app.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	createdID = bson.NilObjectID

	redisAddr := envOr("FIBERHOUSE_REDIS_ADDR", "127.0.0.1:6379")
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	key := "fiberhouse:integration:" + runID
	t.Cleanup(func() { _ = redisClient.Del(context.Background(), key).Err() })
	if err := redisClient.Set(ctx, key, runID, time.Minute).Err(); err != nil {
		t.Fatalf("Redis unavailable at FIBERHOUSE_REDIS_ADDR: %v", err)
	}
	got, err := redisClient.Get(ctx, key).Result()
	if err != nil || got != runID {
		t.Fatalf("Redis round trip = %q, %v", got, err)
	}
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("FIBERHOUSE_INTEGRATION") != "1" {
		t.Skip("set FIBERHOUSE_INTEGRATION=1 to run real-service integration tests")
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
