# GlitchTip 端到端验证套件

用框架真正的 `DefaultStackTraceHandler` 产生错误日志，经 sentryzerolog 送到本机 GlitchTip，
检查 issue 标题、Exception 堆栈、tag 与分组。结论见 `../glitchtip-error-field-verification.md`。

> 仅用于本机验证：compose 开启了 `DEBUG=true` 与 `bootstrap_dev` 的固定账号，切勿用于正式环境。

## 组成

| 文件 | 作用 |
|---|---|
| `compose.yml` | postgres:18 + valkey:9 + glitchtip:6.2.6，监听 `127.0.0.1:18000` |
| `harness/main.go` | Fiber 应用：`/panic`（nil 指针 panic）与 `/error`（handler 返回 error），走框架的恢复中间件与 ErrorHandler |
| `harness/appside.go` | 应用端 `BeforeSend` 方案（`APPSIDE=1` 时启用），说明见 `../glitchtip-app-side-before-send.md` |
| `harness/go.mod` | 独立模块，`replace` 指向仓库根目录（`../../..`），即验证当前工作区的框架代码 |

目录以 `.` 开头，`go build ./...`、`go test ./...` 不会进入这里，不影响仓库构建。

## 运行

```sh
cd .codegraph-qa-out/glitchtip-verify
docker compose up -d
docker compose logs web | grep -A8 'Dev Environment Ready'   # 取 DSN；账号 test@example.com / admin_pass

cd harness
SENTRY_DSN=<上面的 DSN> VARIANT=<环境名> DUMP_DIR=/tmp/gt-dump go run .
# 加 APPSIDE=1 启用应用端 BeforeSend；ERRMSG 改 /error 的错误文本
```

- `VARIANT` 写入事件的 environment，在 GlitchTip 里按 Environment 筛选对照。
- `DUMP_DIR` 保存 SDK 送出前的事件 JSON（`<VARIANT>-<scenario>.json`），需事先存在。
- API 读取（token 为 `bootstrap_dev` 固定值，64 个 `d`）：
  `curl -H "Authorization: Bearer dddd…" http://localhost:18000/api/0/projects/org/project/issues/`

清理：`docker compose down -v`。
