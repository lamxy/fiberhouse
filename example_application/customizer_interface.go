package example_application

import "github.com/lamxy/fiberhouse/globalmanager"

// IApplicationCustomizer 定制项目应用获取实例key的方法;
// module业务层Api、Service、Repository、Model等层通过ctx获取application，转成IApplicationCustomizer即可调用这里定制的获取实例key方法
type IApplicationCustomizer interface {
	// 示例：自定义xxx全局对象key的获取方法
	GetXxxCustomKey() globalmanager.KeyName
}
