package service

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
)

type TestService struct {
	fiberhouse.ServiceLocator
	KeyExampleRepository string
}

func NewTestService(ctx fiberhouse.IApplicationContext) *TestService {
	return &TestService{
		ServiceLocator:       fiberhouse.NewService(ctx).SetName(GetKeyTestService()),
		KeyExampleRepository: repository.RegisterKeyExampleRepository(ctx),
	}
}

// GetKeyTestService 获取 TestService 注册键名
func GetKeyTestService(ns ...string) string {
	return fiberhouse.RegisterKeyName("TestService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyTestService 注册 TestService 实例初始化器到全局管理器，并返回注册实例key；该方法可由依赖方法调用，或由组件使用，无需wire或其他依赖注入工具
func RegisterKeyTestService(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyTestService(ns...), func() (interface{}, error) {
		return NewTestService(ctx), nil
	})
}

// HelloWorld 示例方法
func (s *TestService) HelloWorld() string {
	s.GetContext().GetLogger().InfoWith(s.GetContext().GetConfig().LogOriginWeb()).Msg("TestService HelloWorld()")
	return "Hello World!"
}

// DoAgeDoubleCreateForTaskHandle 示例任务处理函数
func (s *TestService) DoAgeDoubleCreateForTaskHandle(age int8) (int, error) {
	s.GetContext().GetLogger().InfoWith(s.GetContext().GetConfig().LogOriginTask()).Msgf("DoAgeDoubleCreateForTaskHandle result: %d", age*2)

	// 通过注册key获取ExampleRepository实例
	exampleRepo, err := fiberhouse.GetInstance[*repository.ExampleRepository](s.KeyExampleRepository)
	if err != nil {
		s.GetContext().GetLogger().ErrorWith(s.GetContext().GetConfig().LogOriginTask()).Err(err).Msg("Get ExampleRepository instance error")
		return 0, err
	}

	// 新建一个样例记录，将任务参数age的两倍作为年龄保存
	example := requestvo.ExampleReqVo{
		ExamName: "task_created-by-age-double",
		ExamAge:  int(age * 2),
		Courses:  []string{"c1", "c2"},
		Profile:  map[string]interface{}{"pf1": "value1", "pf2": "value2"},
	}

	id, err := exampleRepo.CreateExample(&example)

	if err != nil {
		s.GetContext().GetLogger().ErrorWith(s.GetContext().GetConfig().LogOriginTask()).Err(err).Msg("Create Example record error")
	}
	s.GetContext().GetLogger().InfoWith(s.GetContext().GetConfig().LogOriginTask()).Msgf("Create Example record success, id: %s", id)

	return int(age * 2), nil
}
