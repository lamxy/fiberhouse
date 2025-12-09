package task

import (
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"time"
)

/*
Task payload list 任务负载列表
*/

// PayloadExampleCreate 样例创建负载的数据
type PayloadExampleCreate struct {
	fiberhouse.PayloadBase // 继承基础负载结构体，自动具备获取json编解码器的方法
	/**
	负载的数据
	*/
	Age int8
}

// NewExampleCreateTask 生成一个 ExampleCreate 任务，从调用处获取相关参数，并返回任务
func NewExampleCreateTask(ctx fiberhouse.IContext, age int8) (*asynq.Task, error) {
	vo := PayloadExampleCreate{
		Age: age,
	}
	// 获取json编解码器，将负载数据编码为json格式的字节切片
	payload, err := vo.GetMustJsonHandler(ctx).Marshal(&vo)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExampleCreate, payload, asynq.Retention(24*time.Hour), asynq.MaxRetry(3), asynq.ProcessIn(1*time.Minute)), nil
}
