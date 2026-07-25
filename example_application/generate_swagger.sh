#!/usr/bin/env bash
#
# generate_swagger.sh —— 从 swaggo 注解重新生成 example_application/docs。
# 不作为本仓库任何 build/test/CI 步骤执行；当你想刷新生成的 OpenAPI 规范时，
# 手动、有意地运行它。
#
# 前置条件（本脚本不安装）：
#   go install github.com/swaggo/swag/cmd/swag@latest
#
# 下方各标志背后的完整理由见 example_application/docs/README.md（为何 -g 指向
# example_main/main.go、为何只注解 Fiber handler，以及 doc.go 在运行前后是什么样）。

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if ! command -v swag >/dev/null 2>&1; then
  echo "错误：PATH 中未找到 swag CLI。" >&2
  echo "请用以下命令安装：go install github.com/swaggo/swag/cmd/swag@latest" >&2
  exit 1
fi

swag init \
  -g example_main/main.go \
  -d . \
  -o example_application/docs \
  --parseDependency \
  --parseInternal \
  --parseDepth 2

echo "已从 swaggo 注解生成 example_application/docs。"
