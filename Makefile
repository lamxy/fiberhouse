APP_NAME := fiberhouse
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | awk '{print $$3}')

# 构建参数
LDFLAGS := -ldflags="-X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GoVersion=$(GO_VERSION)'"

# 目录定义
CMD_DIR := example_main
TARGET_DIR := $(CMD_DIR)/target
BIN := $(TARGET_DIR)/$(APP_NAME)

.PHONY: help deps build test lint clean run pre-commit build-all version

# 默认目标
help: ## 显示帮助信息
	@echo "可用命令："
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## 安装依赖
	go mod download
	go mod tidy

build: deps ## 构建应用
	@mkdir -p $(TARGET_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BIN) ./$(CMD_DIR)/main.go
	@echo "构建完成: $(BIN)"

# 测试相关命令
test: ## 运行所有测试
	@echo "正在运行单元测试..."
	set CGO_ENABLED=1 && go test ./frame/... -race -cover -coverprofile=coverage.out
	@echo "测试完成"

test-verbose: ## 详细测试输出
	set CGO_ENABLED=1 && go test -v ./frame/... -race -cover

test-coverage: ## 生成测试覆盖率报告
	set CGO_ENABLED=1 && go test ./frame/... -race -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

test-bench: ## 运行基准测试
	go test -bench=. ./frame/... -benchmem

lint: ## 代码静态检查
	golangci-lint run --timeout=5m

clean: ## 清理构建产物
	@rm -rf $(TARGET_DIR)
	@rm -f coverage.out coverage.html
	@echo "清理完成"

# Docker 相关

# 运行应用
run: build ## 运行应用
	cd $(CMD_DIR) && ../$(BIN)

# CI 环境测试

# 发布前检查
pre-commit: lint test
	@echo "预发布检查通过"

# 交叉编译
build-all: ## 交叉编译多平台
	@mkdir -p $(TARGET_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(TARGET_DIR)/$(APP_NAME)-linux-amd64 ./$(CMD_DIR)/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(TARGET_DIR)/$(APP_NAME)-windows-amd64.exe ./$(CMD_DIR)/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(TARGET_DIR)/$(APP_NAME)-darwin-amd64 ./$(CMD_DIR)/main.go
	@echo "交叉编译完成"

# 显示版本信息
version:
	@go version