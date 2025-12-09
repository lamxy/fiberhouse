# Package globalmanager 提供全局对象管理功能，用于管理应用程序中的单例对象。

该包提供了一个线程安全的全局对象管理器，支持延迟初始化、健康检查、重建和资源释放等功能。
适用于读多写少的场景。

基本用法:

	// 创建全局管理器
	gm := globalmanager.NewGlobalManagerOnce() 或 gm := globalmanager.NewGlobalManager()

	// 注册对象初始化器
	gm.Register("database", func() (interface{}, error) {
		return &Database{}, nil
	})

	// 获取对象实例
	db, err := gm.Get("database")
	if err != nil {
		log.Fatal(err)
	}

	// 类型断言使用
	database := db.(*Database)