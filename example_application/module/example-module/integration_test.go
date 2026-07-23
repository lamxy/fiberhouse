package examplemodule_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/model"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

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

	collectionName := envOr("FIBERHOUSE_MONGODB_COLLECTION", "example") + "_integration_" + runID
	collection := mongoClient.Database(envOr("FIBERHOUSE_MONGODB_DATABASE", "test")).
		Collection(collectionName)
	productionModel := model.NewExampleModelWithCollection(collection)
	productionRepository := repository.NewExampleRepository(nil, productionModel)
	app := service.NewExampleService(nil, productionRepository)
	var createdID bson.ObjectID
	t.Cleanup(func() {
		if !createdID.IsZero() {
			_, _ = collection.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: createdID}})
		}
		_ = collection.Drop(context.Background())
	})

	created, err := app.Create(ctx, requestvo.CreateExampleReqVo{Name: "integration_" + runID, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	createdID, err = bson.ObjectIDFromHex(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	indexCursor, err := collection.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list production indexes: %v", err)
	}
	var indexes []bson.M
	if err := indexCursor.All(ctx, &indexes); err != nil {
		t.Fatalf("decode production indexes: %v", err)
	}
	indexNames := make(map[string]bool, len(indexes))
	for _, index := range indexes {
		if name, ok := index["name"].(string); ok {
			indexNames[name] = true
		}
	}
	for _, name := range []string{"example_name_unique", "example_status_created_id"} {
		if !indexNames[name] {
			t.Fatalf("production index %q was not created: %#v", name, indexNames)
		}
	}
	if _, err := app.Get(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := app.List(ctx, requestvo.ListExamplesReqVo{Page: 1, PageSize: 100, Status: "active"})
	if err != nil || listed.Total < 1 {
		t.Fatalf("list result = %#v, err = %v", listed, err)
	}
	archived := "archived"
	updated, err := app.Update(ctx, created.ID, requestvo.UpdateExampleReqVo{Status: &archived})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != archived {
		t.Fatalf("updated status = %q, want %q", updated.Status, archived)
	}
	if err := app.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Get(ctx, created.ID); err == nil {
		t.Fatal("Get() after Delete() error = nil, want not found")
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
