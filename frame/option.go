package frame

// ApplicationStarterOption 定义应用启动器选项函数类型
type ApplicationStarterOption func(starter ApplicationStarter)

// CommandStarterOption 定义命令启动器选项函数类型
type CommandStarterOption func(starter CommandStarter)
