package context

// ICoreContext 统一核心的上下文接口
type ICoreContext interface {
	JSON(statusCode int, data interface{}) error
	GetCtx() interface{}
	// TODO 支持更多的方法： JSONP、XML、SendString...
	//  其他rpc传输协议...
}
