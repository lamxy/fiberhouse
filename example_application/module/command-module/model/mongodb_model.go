package model

import (
	"context"
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/database/dbmongo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
)

type MongodbModel struct {
	dbmongo.MongoLocator
	ctx context.Context
}

func NewMongodbModel(ctx fiberhouse.ContextCommander) *MongodbModel {
	return &MongodbModel{
		MongoLocator: dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).SetDbName(constant.DefaultMongoDatabase).
			SetName("MongodbModel").(dbmongo.MongoLocator),
		ctx: context.Background(),
	}
}

func (m *MongodbModel) Test() error {
	fmt.Println("MongodbModel Test OK")
	return nil
}
