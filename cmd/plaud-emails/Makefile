.PHONY: build run test clean help local-debug

# 默认目标
.DEFAULT_GOAL := help

# 变量
APP_DIR := .
APP_BIN := $(APP_DIR)/plaud-emails
DOCKER_IMAGE ?= plaud-emails:latest
DOCKER_SVC ?= plaud-emails
LOCAL_FLAGS ?= -config_mode=local -app_config_name=dev.yaml -env=dev -public_ip=127.0.0.1 -public_port=8080
LOCAL_FLAGS_RPC ?= -config_mode=local -app_config_name=dev.yaml -env=dev -public_port=0 -service_addr=127.0.0.1 -grpc_ip=127.0.0.1 -grpc_port=8081
LOCAL_DEBUG_FLAGS ?= -config_mode=local -app_config_name=dev.template.yaml -env=dev -public_ip=127.0.0.1 -public_port=7080

# 构建项目
build:
	@echo "构建项目..."
	cd $(APP_DIR) && go build -mod=vendor -o $(APP_BIN) .

# 本地运行
local-run: build
	@echo "本地运行: $(APP_BIN) $(LOCAL_FLAGS)"
	cd $(APP_DIR) && go run . $(LOCAL_FLAGS)

local-run-rpc: build
	@echo "本地单独RPC: $(APP_BIN) $(LOCAL_FLAGS_RPC)"
	cd $(APP_DIR) && go run . $(LOCAL_FLAGS_RPC)

# 本地调试运行（连接 dev 环境数据库，禁用 etcd）
local-debug: build
	@echo "本地调试运行: $(APP_BIN) $(LOCAL_DEBUG_FLAGS)"
	cd $(APP_DIR) && go run . $(LOCAL_DEBUG_FLAGS)

# Docker 构建镜像
docker-build:
	@echo "构建 Docker 镜像: $(DOCKER_IMAGE) ..."
	docker build -f $(APP_DIR)/Dockerfile -t $(DOCKER_IMAGE) $(APP_DIR)/../..

# Docker 运行（基于 docker-compose）
docker-start: docker-build
	@echo "以 Docker 方式运行（docker compose up -d）..."
	docker-compose -f $(APP_DIR)/docker-compose.yml up -d
	@echo "查看日志: make docker-logs"

# Docker 停止
docker-stop:
	@echo "停止并清理容器..."
	docker-compose -f $(APP_DIR)/docker-compose.yml stop

# Docker 重启服务
docker-restart:
	@echo "重启 $(DOCKER_SVC) ..."
	docker-compose -f $(APP_DIR)/docker-compose.yml restart $(DOCKER_SVC)

# Docker 查看日志
docker-logs:
	@echo "跟随查看 $(DOCKER_SVC) 日志..."
	docker-compose -f $(APP_DIR)/docker-compose.yml logs -f $(DOCKER_SVC)


# 测试项目
test:
	@echo "运行测试..."
	go test ./...

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -f $(APP_BIN)
	go clean

# 安装依赖
deps:
	@echo "安装依赖..."
	go mod tidy
	go mod download

# 生成gRPC代码
proto:
	@echo "生成gRPC代码..."
	mkdir -p ../../external/helloservice
	@which protoc > /dev/null || (echo "protoc not found. Please install protobuf: brew install protobuf" && exit 1)
	@which protoc-gen-go > /dev/null || (echo "protoc-gen-go not found. Please install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest" && exit 1)
	@which protoc-gen-go-grpc > /dev/null || (echo "protoc-gen-go-grpc not found. Please install: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" && exit 1)
	cd ../../ && protoc -I external/proto \
		--go_opt=paths=source_relative --go_out=./external/helloservice \
		--go-grpc_out=./external/helloservice --go-grpc_opt=paths=source_relative \
		external/proto/hello_service.proto

# 格式化代码
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 代码检查
lint:
	@echo "代码检查..."
	golangci-lint run

# 生成文档
docs:
	@echo "生成文档..."
	godoc -http=:6060

# 帮助信息
help:
	@echo "可用的命令:"
	@echo "  build     - 构建项目"
	@echo "  local-run - 本地运行 cmd/plaud-emails http服务 （可通过 LOCAL_FLAGS 覆盖启动参数）"
	@echo "  local-run-rpc - 本地运行 cmd/plaud-emails grpc服务 （可通过 LOCAL_FLAGS_RPC 覆盖启动参数）"
	@echo "  local-debug - 本地调试运行（连接 dev 环境数据库，禁用 etcd）"
	@echo "  docker-build - 构建 Docker 镜像（DOCKER_IMAGE 可覆盖）"
	@echo "  docker-start  - 使用 docker compose 以 Docker 方式运行"
	@echo "  docker-stop - 停止并清理容器"
	@echo "  docker-restart - 重启应用容器"
	@echo "  docker-logs - 查看应用容器日志"
	@echo "  test      - 运行测试"
	@echo "  clean     - 清理构建文件"
	@echo "  deps      - 安装依赖"
	@echo "  proto     - 生成gRPC代码"
	@echo "  fmt       - 格式化代码"
	@echo "  lint      - 代码检查"
	@echo "  docs      - 生成文档"
	@echo "  help      - 显示帮助信息" 