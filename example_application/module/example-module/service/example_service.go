package service

import (
	"context"
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/cache"
	"github.com/lamxy/fiberhouse/example_application/api-vo/commonvo"
	"github.com/lamxy/fiberhouse/example_application/api-vo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/api-vo/example/responsevo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
	"strconv"
	"time"
)

// ExampleService 样例服务，继承 fiberhouse.ServiceLocator 服务定位器接口，具备获取上下文、配置、日志、注册实例等功能
type ExampleService struct {
	fiberhouse.ServiceLocator                               // 继承服务定位器接口
	Repo                      *repository.ExampleRepository // 依赖的组件: 样例仓库，构造参数注入。由wire自动注入
}

func NewExampleService(ctx fiberhouse.IApplicationContext, repo *repository.ExampleRepository) *ExampleService {
	name := GetKeyExampleService()
	return &ExampleService{
		ServiceLocator: fiberhouse.NewService(ctx).SetName(name),
		Repo:           repo,
	}
}

// GetKeyExampleService 获取 ExampleService 注册键名
func GetKeyExampleService(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExampleWithTaskDispatcher 示例方法，演示如何在服务方法中使用任务调度器异步执行任务
func (s *ExampleService) GetExampleWithTaskDispatcher(id string) (*responsevo.ExampleRespVo, error) {
	resp := responsevo.ExampleRespVo{}
	example, err := s.Repo.GetExampleById(id)
	if err != nil {
		return nil, err
	}

	// 获取带任务标记的日志器，从全局管理器获取已附加了日志源标记的日志器
	log := s.GetContext().GetMustLoggerWithOrigin(s.GetContext().GetConfig().LogOriginTask())

	// 获取样例数据成功，推送延迟任务异步执行
	dispatcher, err := s.GetContext().(fiberhouse.IApplicationContext).GetStarterApp().GetTask().GetTaskDispatcher()
	if err != nil {
		log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher GetTaskDispatcher failed")
	}
	task1, err := task.NewExampleCreateTask(s.GetContext(), int8(example.Age))
	if err != nil {
		log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher NewExampleCountTask failed")
	}
	tInfo, err := dispatcher.Enqueue(task1, asynq.MaxRetry(constant.TaskMaxRetryDefault), asynq.ProcessIn(1*time.Minute))

	if err != nil {
		log.Warn().Err(err).Msg("GetExampleWithTaskDispatcher Enqueue failed")
	} else if tInfo != nil {
		log.Warn().Msgf("GetExampleWithTaskDispatcher Enqueue task info: %v", tInfo)
	}

	// 正常的业务逻辑
	resp.ExamName = example.Name
	resp.ExamAge = example.Age
	resp.Courses = example.Courses
	resp.Profile = example.Profile
	resp.CreatedAt = example.CreatedAt
	resp.UpdatedAt = example.UpdatedAt
	return &resp, nil
}

func (s *ExampleService) GetExample(id string) (*responsevo.ExampleRespVo, error) {
	resp := responsevo.ExampleRespVo{}
	example, err := s.Repo.GetExampleById(id)
	if err != nil {
		return nil, err
	}
	resp.ExamName = example.Name
	resp.ExamAge = example.Age
	resp.Courses = example.Courses
	resp.Profile = example.Profile
	resp.CreatedAt = example.CreatedAt
	resp.UpdatedAt = example.UpdatedAt
	return &resp, nil
}

func (s *ExampleService) CreateExample(vo *requestvo.ExampleReqVo) (string, error) {
	id, err := s.Repo.CreateExample(vo)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *ExampleService) GetExamples(page, size int) ([]responsevo.ExampleRespVo, error) {
	// TODO cached： 1 proxy代理Repo对象、 2 GetCached泛型方法

	// 获取缓存选项对象
	co := cache.OptionPoolGet(s.GetContext())
	defer cache.OptionPoolPut(co)

	// 设置缓存参数: 二级缓存、启用本地缓存、设置缓存key、设置本地缓存随机过期时间(10秒±10%)、设置远程缓存随机过期时间(3分钟±1分钟)、写远程缓存同步策略、设置上下文、启用缓存全部的保护措施
	co.Level2().EnableCache().
		SetCacheKey("key:example:list:page:"+strconv.Itoa(page)+":size:"+strconv.Itoa(size)).
		SetLocalTTLRandomPercent(10*time.Second, 0.1).
		SetRemoteTTLWithRandom(3*time.Minute, 1*time.Minute).
		SetSyncStrategyWriteRemoteOnly().
		SetContextCtx(context.TODO()).
		EnableProtectionAll()

	// 获取缓存数据
	return cache.GetCached[[]responsevo.ExampleRespVo](co, func(ctx context.Context) ([]responsevo.ExampleRespVo, error) {
		list, err := s.Repo.GetExamples(page, size)

		if err != nil {
			return nil, err
		}
		examples := make([]responsevo.ExampleRespVo, 0, len(list))
		for i := range list {
			example := responsevo.ExampleRespVo{
				ID:       list[i].ID.Hex(),
				ExamName: list[i].Name,
				ExamAge:  list[i].Age,
				Courses:  list[i].Courses,
				Profile:  list[i].Profile,
				Timestamps: commonvo.Timestamps{
					CreatedAt: list[i].CreatedAt,
					UpdatedAt: list[i].UpdatedAt,
				},
			}
			examples = append(examples, example)
		}
		return examples, nil
	})
}
