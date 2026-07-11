package service

import (
	"fmt"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
)

type MongodbService struct {
	fiberhouse.ServiceLocator
	MongoModel *model.MongodbModel
}

func NewMongodbService(ctx fiberhouse.ICommandContext, mongodbModel *model.MongodbModel) *MongodbService {
	return &MongodbService{
		ServiceLocator: fiberhouse.NewService(ctx).SetName("MongodbService").(*fiberhouse.Service),
		MongoModel:     mongodbModel,
	}
}

func (s *MongodbService) Test() error {
	fmt.Println("MongodbService Test OK")
	return nil
}
