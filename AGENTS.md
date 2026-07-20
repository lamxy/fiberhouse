# Codex 项目指令

此项目可以将 Claude 导向的规则来源作为共用代理规则材料。此文件是 Codex-native 的指令入口。

## 规则来源

使用第一个存在的规则来源：

1. `.agent-rules/claude/`
2. `.claude/`

Claude 规则来源是参考材料。不要盲目加载或遵循整份 `CLAUDE.md`。先使用此 `AGENTS` 文件，再只读取当前任务需要的规则文件。

除非已另行安装 Codex-native 设定，否则 Claude 专用的 `settings.json`、`hooks/`、插件与 `SendMessage` 指令不是生效中的 Codex 执行时设定。

## 核心行为

- 直接回应当前请求。
- 先给结论，再只补充必要细节。
- 保持上下文精简；不要加载无关的规则、模板、日志或文档。
- 如果不确定，说明不确定性以及缺少哪些证据。
- 只有当答案会实质改变工作时，才提出一个关键澄清问题。
- 优先采用现有项目模式，而不是新增抽象。
- 未经 approval，不要执行破坏性或对外可见的操作。

## 规则加载

使用最小且足够的指令层。

1. 先应用此 `AGENTS` 文件。
2. 对一般任务，除非需要更多细节，否则使用上方核心行为。
3. 当任务符合"场景提示"列出的场景时，只读取生效中规则来源下列出的路径。列出的路径具有权威性，可能位于 `rules/` 或 `expandable/`。
4. 当没有适用的场景提示时，只读取当前任务最相关的本地规则文件。
5. 只有在输出结构必须稳定时，才使用 `expandable/templates/` 下的模板。
6. 不要仅因规则存在就加载无关规则。

## 场景提示

- 一般任务：`rules/task/general-task-rule-min.md`
- 工具使用：`rules/task/tool-call-rule-min.md`
- 子代理任务：`rules/task/sub-agent-rule-min.md`
- 设计、规划、架构或头脑风暴：`expandable/task/design-first-rule-min.md`
- 循环、cron 或重复监控：`expandable/task/loop-cron-rule-min.md`
- Agent 团队工作流程：`expandable/task/agent-team-rule-min.md`
- 输出格式：`expandable/templates/`

当被引用的文件不存在时，使用可用的最佳本地指令继续，不要因此阻塞。

## Token 节省与 RTK 命令代理规约
- **原则**：为了节省上下文 Token 并提高推理精度，你必须优先使用 `rtk` 代理高噪声的终端命令。
- **强制使用 `rtk` 前缀的命令**：
    - 文件/目录检索：`ls`、`tree`
    - 文本搜索与读取：`cat`、`grep`、`rg`
    - Git 版本管理：`git status`、`git diff`、`git log`
    - 测试运行器：`go test`（尤其是运行大套件时）
    - 容器管理：`docker ps`、`kubectl get`
- **示例**：
    - ❌ 禁止使用：`go test ./...`
    -   应当使用：`rtk go test ./...`
    - ❌ 禁止使用：`git diff`
    -   应当使用：`rtk git diff`

- **白名单机制（何时不使用 RTK）**：
    - 如果你执行了 `rtk <command>` 发现输出被过度裁剪，且你确实需要阅读完整的原始报错或未压缩细节，请在命令前加上 `NO_RTK` 前缀来绕过 Hook（例如：`NO_RTK go test ./...`）。

@CLAUDE.md
