# Example application 模块

各示例共享领域行为，而非按框架重复实现：

```text
Fiber adapter ----\
                   -> canonical MongoDB example module
Gin adapter ------/

CLI adapter --------> independent MySQL command module
```

- [`example-module`](example-module/README.md) 拥有 HTTP CRUD 行为、MongoDB 持久化、
  列表缓存，以及 `example:changed` 任务契约。
- [`example-ginapi-module`](example-ginapi-module/README.md) 把 Gin 适配到同一个规范
  用例上。
- [`command-module`](command-module/README.md) 通过 `urfave/cli` 演示完整的 MySQL CRUD。

本地验证：

```bash
go test ./example_application/... -count=1
FIBERHOUSE_INTEGRATION=1 go test \
  ./example_application/module/example-module \
  ./example_application/module/command-module -count=1
```

集成测试默认值为：MongoDB `127.0.0.1:27037`、MySQL `127.0.0.1:3306`、
Redis `127.0.0.1:6379`。环境变量覆盖在各模块 README 中说明。
