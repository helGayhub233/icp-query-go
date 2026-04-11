.PHONY: help build build-all build-darwin build-linux build-windows clean lint format test run docker

# 项目配置
BINARY := icpcli
DIST   := dist

# 自动检测系统和架构
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# 版本信息（优先从 git tag 获取）
VERSION    ?= $(shell git describe --tags --always --dirty="-dev" 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# 链接器标志
LDFLAGS := -s -w \
    -X github.com/imxw/icp-query-go/cmd.Version=$(VERSION) \
    -X github.com/imxw/icp-query-go/cmd.GitCommit=$(COMMIT) \
    -X github.com/imxw/icp-query-go/cmd.BuildDate=$(BUILD_DATE)

# 跨平台目标
PLATFORMS := \
    darwin/amd64 \
    darwin/arm64 \
    linux/amd64 \
    linux/arm64 \
    windows/amd64 \
    windows/arm64

# 输出文件名
ifeq ($(GOOS),windows)
    BINARY_OUT := $(BINARY).exe
else
    BINARY_OUT := $(BINARY)
endif

.DEFAULT_GOAL := help

## help:           显示帮助信息
help:
	@echo "icpcli - ICP备案查询工具"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk -F': +' '/^## /{sub(/^## +/,"");printf "  %-15s %s\n",$$1,$$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Current: $(GOOS)/$(GOARCH), version: $(VERSION)"

## build:          编译当前平台
build:
	@echo "=> Building $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY_OUT) .

## build-all:      编译所有平台
build-all: $(foreach p,$(PLATFORMS),cross-$(p))

define CROSS_RULE
cross-$(1):
	@echo "=> Building $(1)..."
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=$(word 1,$(subst /, ,$(1))) GOARCH=$(word 2,$(subst /, ,$(1))) \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(DIST)/$(BINARY)-$(VERSION)-$(subst /,-,$(1))$(if $(findstring windows,$(1)),.exe,) .
endef

$(foreach p,$(PLATFORMS),$(eval $(call CROSS_RULE,$(p))))

## build-darwin:   编译 macOS (amd64 + arm64)
build-darwin: cross-darwin/amd64 cross-darwin/arm64

## build-linux:    编译 Linux (amd64 + arm64)
build-linux: cross-linux/amd64 cross-linux/arm64

## build-windows:  编译 Windows (amd64 + arm64)
build-windows: cross-windows/amd64 cross-windows/arm64

## run:            编译并运行 Web 服务
run: build
	./$(BINARY_OUT) serve

## lint:           代码检查
lint:
	go vet ./...

## format:         格式化代码 + 整理 import
format:
	goimports -w .

## test:           运行测试
test:
	go test -race ./...

## docker:         构建 Docker 镜像
docker:
	docker build -t icp-query:$(VERSION) .

## clean:          清理构建产物
clean:
	rm -rf $(DIST) $(BINARY) $(BINARY).exe
