# 构建阶段
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件（使用 vendor 构建，无需下载依赖）
COPY go.mod go.sum ./

# 复制源代码（构建上下文为仓库根目录）
COPY . .

# 构建应用（使用 vendor）
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo \
    -o /app/plaud-emails ./cmd/plaud-emails

# 运行阶段
FROM alpine:latest

# 安装必要的系统依赖
RUN apk --no-cache add ca-certificates tzdata wget

# 创建非root用户
RUN addgroup -g 1001 -S plaud && \
    adduser -u 1001 -S plaud -G plaud

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/plaud-emails .

# 复制配置（本地默认模式会从 ./conf 读取）
COPY cmd/plaud-emails/conf ./conf

# 创建日志目录
RUN mkdir -p ./logs

# 设置文件权限
RUN chown -R plaud:plaud /app

# 切换到非root用户
USER plaud

# 暴露端口
EXPOSE 8080
EXPOSE 8081

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动应用
CMD ["./plaud-emails", "-app_config_name=docker.yaml","-public_ip=0.0.0.0","-public_port=8080", "-service_addr=127.0.0.1","-grpc_ip=0.0.0.0","-grpc_port=8081"]