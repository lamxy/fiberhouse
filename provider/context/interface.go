package context

// ContextProvider 统一的上下文接口
type ContextProvider interface {
	JSON(statusCode int, data interface{}) error
	GetCtx() interface{}
	// TODO 支持更多的方法： JSONP、XML、SendString...
}
