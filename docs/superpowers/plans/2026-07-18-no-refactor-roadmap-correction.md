# No-Refactor Roadmap Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove broad/refactor-driven future work from the active optimization checklist and replace it with an explicit local-fix-only admission policy while preserving completed history verbatim.

**Architecture:** This is a documentation-only correction. One active checklist remains the source of future task selection; completed specs, plans, execution records, feature-status facts, user-owned analysis, source code, tests, CI, and public APIs remain untouched.

**Tech Stack:** Markdown, Git, `rg`, `awk`, `sha256sum`, `git diff`

## Global Constraints

- Do not add `Run(ctx context.Context) error` or an equivalent entry point.
- Do not modify existing public interfaces, method signatures, or call chains to unify Fiber, Gin, CLI, or task lifecycle behavior.
- Do not add a shutdown registry, closer stack, lifecycle coordinator, or equivalent infrastructure.
- Do not split GlobalManager owner/locator responsibilities or plan a complete state-machine rewrite.
- Do not move public packages into `internal/`, add replacement default-assembly APIs, or start cross-package API governance work.
- Do not refactor, rename, move, abstract, consolidate, or clean up adjacent code or documentation.
- Preserve completed historical specs, plans, and execution records verbatim.
- Do not modify `.codegraph-qa-out/readme-current-status-analysis-2026-07-17.md`; it is user-owned and untracked on `main`.
- Modify no Go source, test, workflow, README, feature-status, guide, or historical plan/spec file.

---

### Task 1: Replace the active broad roadmap with a local-fix-only policy

**Files:**
- Modify: `.codegraph-qa-out/readme-current-status-optimization-todo.md:3-260`
- Reference only: `docs/superpowers/specs/2026-07-18-no-refactor-roadmap-correction-design.md`

**Interfaces:**
- Consumes: the approved no-refactor constraints and the completed P0, P1-A, and P1-B execution records already embedded in the checklist.
- Produces: the sole active optimization roadmap, limited to evidence-backed changes inside existing functions, interfaces, and call paths.

- [ ] **Step 1: Capture the protected execution-record hashes**

Run these commands before editing:

```bash
git show 1384918:.codegraph-qa-out/readme-current-status-optimization-todo.md \
  | awk '$0=="## P0 执行记录（2026-07-18）" {inside=1; next} inside && /^## / {exit} inside' \
  | sha256sum

git show 1384918:.codegraph-qa-out/readme-current-status-optimization-todo.md \
  | awk '$0=="## P1-A 执行记录（2026-07-18）" {inside=1; next} inside && /^## / {exit} inside' \
  | sha256sum

git show 1384918:.codegraph-qa-out/readme-current-status-optimization-todo.md \
  | awk '$0=="## P1-B 执行记录（2026-07-18）" {inside=1; next} inside && /^## / {exit} inside' \
  | sha256sum
```

Expected: three non-empty SHA-256 values. Record them in the ignored task report so the same values can be asserted after editing.

- [ ] **Step 2: Prove the active checklist still contains the withdrawn directions**

Run:

```bash
rg -n '^- \[ \].*(Run\(ctx|shutdown registry|owner.*locator|迁入 `internal`|统一关闭链|完整.*状态机|新增.*入口)' \
  .codegraph-qa-out/readme-current-status-optimization-todo.md
```

Expected: exit 0 with matches in the current P1 lifecycle/API sections. This is the documentation RED condition: the active checklist still authorizes work that the approved design withdraws.

- [ ] **Step 3: Add the authoritative no-refactor notice and narrow the priority overview**

Immediately after the checklist metadata, add this exact block:

```markdown
> 后续优化约束（2026-07-18）：禁止重构和大范围修改。历史分析、spec、plan 与执行记录只作为审计材料，不构成后续实施授权；未来任务以本文当前活动项为准。
```

Add this exact section after `## 执行原则` and its existing checklist:

```markdown
## 无重构硬约束（2026-07-18）

- 不新增 `Run(ctx context.Context) error` 或等价入口，不修改现有公共接口、方法签名和调用链。
- 不新增 shutdown registry、closer stack、生命周期协调器或其他统一关闭基础设施。
- 不拆分 GlobalManager owner/locator，不实施完整状态机重写，不自动扩大旧实例退役语义。
- 不迁移公开 package 到 `internal/`，不新增替代性的默认装配 API。
- 不顺带抽象、改名、移动文件、统一相似实现或清理邻近代码。
- 只能处理有明确当前证据、位于一个函数或一条短调用路径、能在原接口内修复的单一缺陷。
- 无法局部修复的问题保留为限制，不进入实施计划。
```

Replace the two broad P1 rows and the P2/P3 dependency language in `## 优先级总览` with:

```markdown
| P1 | 已证实的局部缺陷 | 小 | 修复当前错误行为 | 必须先有复现或静态证据 |
| P2 | 单能力局部验证与文档纠偏 | 小到中 | 补足单项证据 | 不依赖跨组件重构 |
| P3 | 示例与占位事实修正 | 小 | 避免能力叙事漂移 | 只改明确失实内容 |
```

- [ ] **Step 4: Replace active broad sections with cancellation, limitations, and admission gates**

Delete the active `## P1：统一运行、错误与关闭生命周期` section, including P1.1–P1.3 and its completion criteria. Do not alter the following `## P1-A 执行记录（2026-07-18）` or `## P1-B 执行记录（2026-07-18）` bodies.

Insert this exact section before the P1-A record:

```markdown
## P1：已证实的局部缺陷

### 已撤销方向

以下方向不再是活动任务，也不替换为另一套架构方案：新增 Web/CLI 运行入口、统一关闭链或生命周期协调器、Fiber/Gin/CLI 生命周期统一、GlobalManager owner/locator 拆分、完整状态机重写、公开 package 迁移和默认装配 API 扩展。

### 保留的已知限制

- 现有入口的错误记录、panic/fatal 和返回行为并不完全一致。
- 部分资源缺少关闭验证或可重复外部集成测试。
- GlobalManager 的引用存活期、旧实例退役和 deletion-only 清理边界仍有限制。
- 这些事实只用于说明当前边界，不授权跨组件或公共接口改造。

### 局部任务准入门槛

- 指向一个具体函数或一条短调用路径。
- 有失败测试、可重复现象或当前静态证据。
- 保持公开签名、现有抽象和调用结构不变。
- 开始前能列出全部预期修改文件和直接影响。
- 采用聚焦红绿测试，并运行相关包测试、race 和必要的 vet。
- 一次只处理一个缺陷；不能夹带整理、抽象或相邻修复。
- 任一条件不满足时停止实施，只更新限制记录。
```

Delete the active `## P1：v1 API 治理`, `## P2：分能力专题`, `## P3：示例与占位治理`, and `## 能力晋级门槛` sections. Replace them with this exact candidate pool:

```markdown
## 局部调查候选

以下内容只允许调查，不自动进入实施：

- Gin TLS 配置与实际 serve 路径是否不一致；调查必须确认能否在现有 `AppCoreRun` 和配置结构内局部修复。
- 某个现有外部依赖路径是否缺少可重复测试；每次只选择一个 package 和一种失败行为。
- 状态文档是否与当前源码、测试或 CI 证据不一致；只修正已确认的事实差异。

调查结果必须给出“不修改”选项。若局部修复需要新增入口、改变公共签名、引入协调器、跨组件迁移或结构调整，则结论只能是保留限制。
```

Replace `## 推荐执行顺序` with:

```markdown
## 推荐执行顺序

1. [x] P0：修正文档事实、状态模型和质量门禁。
2. [x] P1-A/P1-B：完成已审核的局部生命周期与 maintenance gate 修复。
3. [ ] 一次只调查一个局部候选，先确认当前证据和直接影响范围。
4. [ ] 给出“不修改”和最多两个原实现内的局部方案，提交用户审批。
5. [ ] 获批后在独立 worktree 中按 TDD 实施；独立审查无重构、无顺带修改后再交用户审核。

不存在自动承接的下一项架构任务；每个候选都必须重新取得实施批准。
```

- [ ] **Step 5: Verify withdrawn work is no longer active and protected history is unchanged**

Run the prohibited-active-item scan again:

```bash
rg -n '^- \[ \].*(Run\(ctx|shutdown registry|owner.*locator|迁入 `internal`|统一关闭链|完整.*状态机|新增.*入口)' \
  .codegraph-qa-out/readme-current-status-optimization-todo.md
```

Expected: exit 1 with no matches.

Run:

```bash
rg -n '无重构硬约束|已撤销方向|局部任务准入门槛|局部调查候选|不存在自动承接' \
  .codegraph-qa-out/readme-current-status-optimization-todo.md
```

Expected: exit 0 with all five concepts present.

Re-run the three `awk ... | sha256sum` commands from Step 1 against the working file instead of `git show`:

```bash
awk '$0=="## P0 执行记录（2026-07-18）" {inside=1; next} inside && /^## / {exit} inside' \
  .codegraph-qa-out/readme-current-status-optimization-todo.md | sha256sum

awk '$0=="## P1-A 执行记录（2026-07-18）" {inside=1; next} inside && /^## / {exit} inside' \
  .codegraph-qa-out/readme-current-status-optimization-todo.md | sha256sum

awk '$0=="## P1-B 执行记录（2026-07-18）" {inside=1; next} inside && /^## / {exit} inside' \
  .codegraph-qa-out/readme-current-status-optimization-todo.md | sha256sum
```

Expected: each hash exactly equals its Step 1 baseline value.

- [ ] **Step 6: Verify scope and repository preservation**

Run:

```bash
git diff --check
git diff --name-only 1384918..HEAD
git diff --name-only
git status --short --branch
git -C ../.. rev-parse --short HEAD
git -C ../.. status --short --branch
test -f ../../.codegraph-qa-out/readme-current-status-analysis-2026-07-17.md
```

Expected before the Task 1 commit:

- `git diff --check` exits 0;
- the union of committed and working-tree changes is limited to the approved design, this plan, and the active checklist;
- no Go, test, workflow, README, feature-status, guide, or historical spec/plan path appears;
- the isolated worktree is on `docs/no-refactor-roadmap`;
- `main` remains at `1384918` with the user-owned analysis document present and untracked.

No Go test command is required for Task 1 because the implementation modifies Markdown only and the isolated worktree baseline `go test ./... -count=1` already passed before documentation work began.

- [ ] **Step 7: Commit the active-checklist correction**

```bash
git add .codegraph-qa-out/readme-current-status-optimization-todo.md
git commit -m "docs: replace broad roadmap with local fixes"
```

Expected: the commit contains only the active checklist. The approved design and this implementation plan remain in their own preceding documentation commits.
