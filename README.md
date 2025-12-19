## 运行项目

### 本地开发模式

```bash
# 进入项目目录

# 安装依赖
make deps

# 本地运行（开发模式）
make local-run
```

项目将在 `http://localhost:8080` 启动，访问健康检查：`http://localhost:8080/health`

### Docker 容器模式

```bash
# 一键启动（包括 Redis、MySQL、应用）
make docker-start

# 查看应用日志
make docker-logs

# 停止服务
make docker-stop

# 完全清理（包括数据卷）
docker-compose down -v
```

### 可用的 Makefile 命令

```bash
make help          # 查看所有可用命令
make deps           # 安装项目依赖
make local-run      # 本地运行应用
make build          # 构建二进制文件
make test           # 运行测试
make lint           # 代码检查
make proto          # 生成 gRPC 代码
make docker-build   # 构建 Docker 镜像
make docker-start   # 启动 Docker 环境
make docker-stop    # 停止 Docker 环境
make clean          # 清理构建文件
```

## ⚙️ 配置管理

### 配置文件结构
`cmd/plaud-emails/conf/` 目录下包含配置文件：
- **dev.yaml**  本地开发环境配置
- **docker.yaml** docker运行环境配置

### 配置模式
- **本地模式**: 从本地 YAML 文件读取配置
- **AWS模式**: 从 AWS AppConfig 读取配置（支持热更新）