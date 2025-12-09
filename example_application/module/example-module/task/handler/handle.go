package handler

import (
	"context"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
	"time"
)

// todo start monitor: podman run --rm --name asynqmon -p 8181:8181 --env "PORT=8181" --env "REDIS_ADDR=10.89.1.8:6379" --env "ENABLE_METRICS_EXPORTER=true" hibiken/asynqmon

// HandleExampleCreateTask 样例任务创建处理器
func HandleExampleCreateTask(ctx context.Context, t *asynq.Task) error {
	// 从 context 中获取 appCtx 全局应用上下文，获取包括配置、日志、注册实例等组件
	appCtx, _ := ctx.Value(fiberhouse.ContextKeyAppCtx).(fiberhouse.ContextFramer)

	// 声明任务负载对象
	var p task.PayloadExampleCreate

	// 解析任务负载
	if err := p.GetMustJsonHandler(appCtx).Unmarshal(t.Payload(), &p); err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginWeb()).Str("From", "HandleStatisticsUserTradeCancelCountTask").Err(err).Msg("[Asynq]: Unmarshal error")
		return err
	}

	// 获取处理任务的实例
	instance, err := fiberhouse.GetInstance[*service.TestService](service.GetKeyTestService())
	if err != nil {
		return err
	}

	// 将负参数传入实例的处理函数
	result, err := instance.DoAgeDoubleCreateForTaskHandle(p.Age)
	if err != nil {
		return err
	}

	// 记录结果
	fmt.Println("======> Task: ", t.Type(), "result: ", result, "time: ", time.Now())
	appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginTask()).Msgf("HandleExampleCountTask 执行成功，结果 Age double: %d", result)
	return nil
}
