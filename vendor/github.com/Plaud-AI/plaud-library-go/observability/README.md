# Plaud Library Go - 可观测性模块

基于 OpenTelemetry 的可观测性解决方案，提供分布式追踪、指标监控和日志增强功能。

## 快速开始

### 基础使用

```go
package main

import (
    "log"
    "github.com/Plaud-AI/plaud-library-go/observability"
)

func main() {
    // 最简单的初始化 - 仅生成trace ID用于日志关联
    err := observability.InitObservability("my-service")
    if err != nil {
        log.Fatal("初始化失败:", err)
    }
    defer observability.ShutdownObservability()
    
    // 你的应用代码...
}
```

### 生产环境配置

```go
// 完整功能配置
err := observability.InitObservability("plaud-api",
    observability.WithOTLPEndpoint("http://otel-collector:4317"),
    observability.WithServiceVersion("1.2.0"),
    observability.WithEnvironment("production"),
    observability.WithTraceSamplingRate(0.1), // 10%采样率
    observability.WithEnhanceLogging(true),
)
```

### 开发调试配置

```go
// 开发环境 - 控制台输出所有数据
err := observability.InitObservability("my-service",
    observability.WithConsoleExport(true),
    observability.WithEnvironment("development"),
)
```

## 主要功能

### 1. 手动链路追踪

```go
import "github.com/Plaud-AI/plaud-library-go/observability/utils"

func businessFunction(ctx context.Context) error {
    // 创建自定义span
    ctx, span := utils.CreateCustomSpan(ctx, "business_operation", 
        map[string]interface{}{"user_id": "123"}, 0)
    defer span.End()
    
    // 在span中执行操作
    err := utils.WithSpan(ctx, "database_query", func(ctx context.Context) error {
        // 数据库操作
        return nil
    })
    
    // 获取trace信息
    traceInfo := utils.GetCurrentTraceInfo(ctx)
    return err
}
```

### 2. 业务指标

```go
import "github.com/Plaud-AI/plaud-library-go/observability/utils"

// 简化API
utils.Inc("api_requests", map[string]string{"endpoint": "/users"})
utils.SetGauge("active_connections", 42)
utils.Record("request_duration", 0.123, map[string]string{"endpoint": "/users"})

// 常用业务指标
utils.APIRequestCount("/users", "GET", 200)
utils.APIRequestDuration("/users", "GET", 0.123)
utils.ErrorCount("validation_error", "user-service")
```

### 3. 日志增强

```go
import "github.com/Plaud-AI/plaud-library-go/observability/core"

func handleRequest(ctx context.Context) {
    // 带trace信息的日志
    core.InfoWithTrace(ctx, "处理用户请求", logrus.Fields{
        "user_id": "123",
    })
    
    // 标准logrus也会自动注入trace信息
    logrus.WithContext(ctx).Info("自动包含trace信息")
}
```

## 框架集成

### Gin框架

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/Plaud-AI/plaud-library-go/observability/instrumentation"
)

func main() {
    // 初始化可观测性
    observability.InitObservability("gin-api", 
        observability.WithEnhanceLogging(true))
    defer observability.ShutdownObservability()

    // 创建Gin应用
    r := gin.New()
    r.Use(gin.Recovery())
    
    // 添加可观测性中间件 - 包含链路追踪、指标和日志
    r.Use(instrumentation.GinCombinedMiddleware("gin-api"))
    
    // 路由处理器
    r.GET("/users/:id", func(c *gin.Context) {
        // 获取trace信息
        traceID, spanID := instrumentation.GetTraceInfoFromGinContext(c)
        
        // 创建自定义span
        ctx, span := instrumentation.CreateSpanFromGinContext(c, "user-query")
        defer span.End()
        
        c.JSON(200, gin.H{
            "user_id":  c.Param("id"),
            "trace_id": traceID,
        })
    })
    
    r.Run(":8080")
}
```

### 原生HTTP

```go
import "github.com/Plaud-AI/plaud-library-go/observability/instrumentation"

// 启用HTTP客户端自动插桩
instrumentation.EnableHTTPClientAutoTracing()

// HTTP服务器中间件
mux := http.NewServeMux()
mux.HandleFunc("/api", handler)
wrappedHandler := instrumentation.HTTPServerMiddleware(mux)
```

## 数据库和消息队列

### Redis插桩

```go
import "github.com/Plaud-AI/plaud-library-go/observability/instrumentation"

// 原生客户端用于缓存操作 - 不产生trace，性能最优
cacheClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
cacheClient.Set(ctx, "user:123", "data", time.Hour)  // 缓存操作，无trace
cacheClient.Get(ctx, "user:123")                     // 缓存操作，无trace

// 代理客户端仅用于消息队列 - 自动处理trace传播
mqClient := instrumentation.CreateTracedRedisClient(cacheClient, "my-service")

// ========== Producer端：发送消息 ==========
// 消息队列操作：自动注入trace信息
mqClient.XAdd(ctx, &redis.XAddArgs{
    Stream: "events", 
    Values: map[string]interface{}{
        "event": "signup",
        "user_id": "12345",
    },
})  // ✅ 自动注入trace到Stream消息

mqClient.Publish(ctx, "notifications", "user signup")  // ✅ 自动注入trace到PubSub

// ========== Consumer端：消费消息 ==========
// ⚠️ 重要变化：XReadGroup和XRead现在返回包含trace信息的context和cleanup函数
traceCtx, result, cleanup := mqClient.XReadGroup(ctx, &redis.XReadGroupArgs{
    Group:    "my-group",
    Consumer: "consumer-1",
    Streams:  []string{"events", ">"},
    Count:    10,
    Block:    time.Second * 5,
})
defer cleanup() // ✅ 自动管理span生命周期

if result.Err() != nil {
    log.WithError(result.Err()).Error("failed to read from stream")
    return
}

// 🚀 智能Trace处理：
// - 如果消息包含trace信息：traceCtx继承producer的trace，实现链路关联
// - 如果消息是老格式（无trace信息）：自动为consumer创建新的trace ID
for _, stream := range result.Val() {
    for _, msg := range stream.Messages {
        // ✅ 使用包含trace信息的context进行日志记录
        // 无论是新消息还是老消息，都会有trace_id用于问题排查
        log.WithContext(traceCtx).WithFields(log.Fields{
            "message_id": msg.ID,
            "stream":     stream.Stream,
        }).Info("processing message")
        
        // ✅ msg.Values已经是清理后的原始数据，不包含trace包装信息
        err := processBusinessLogic(traceCtx, msg.Values)
        if err != nil {
            log.WithContext(traceCtx).WithError(err).Error("business logic failed")
        }
    }
}

// ✅ defer cleanup() 会自动结束consumer span，无需手动管理

// ========== 智能Trace处理详解 ==========
/*
Span属性会显示消息类型：
- message.type: "traced"  -> 消息包含producer的trace信息，实现完整链路
- message.type: "legacy"  -> 老消息无trace信息，consumer自动创建新trace ID

这样确保：
1. 新消息：完整的分布式链路追踪
2. 老消息：consumer也有trace_id便于问题排查
3. 日志关联：所有日志都包含trace_id，无论消息新老
*/

// ========== 缓存操作：直接透传，不产生trace ==========
mqClient.Get(ctx, "cache-key")  // ❌ 不会产生trace，性能与原生一致
mqClient.Set(ctx, "cache-key", "value", time.Hour)  // ❌ 不会产生trace
mqClient.Del(ctx, "cache-key")  // ❌ 不会产生trace
```

#### Redis消息队列完整示例

```go
// Producer服务
func sendMessage(ctx context.Context, mqClient *instrumentation.TracedRedisClient) {
    // 发送消息，自动注入当前span的trace信息
    _, err := mqClient.XAdd(ctx, &redis.XAddArgs{
        Stream: "user-events",
        Values: map[string]interface{}{
            "action":    "user_signup",
            "user_id":   "12345",
            "timestamp": time.Now().Unix(),
        },
    })
    if err != nil {
        log.WithContext(ctx).WithError(err).Error("failed to send message")
    }
}

// Consumer服务
func consumeMessages(ctx context.Context, mqClient *instrumentation.TracedRedisClient) {
    for {
        // 重要：使用返回的traceCtx来继承producer的trace信息，cleanup函数自动管理span
        traceCtx, result, cleanup := mqClient.XReadGroup(ctx, &redis.XReadGroupArgs{
            Group:    "user-service",
            Consumer: "consumer-1",
            Streams:  []string{"user-events", ">"},
            Count:    1,
            Block:    time.Second * 10,
        })
        
        if result.Err() != nil {
            cleanup() // 确保清理资源
            if result.Err() == redis.Nil {
                continue // 超时，继续轮询
            }
            log.WithError(result.Err()).Error("consumer error")
            continue
        }
        
        // 处理每条消息，使用包含trace信息的context
        func() {
            defer cleanup() // 确保在处理完成后清理span
            
            for _, stream := range result.Val() {
                for _, msg := range stream.Messages {
                    // 日志会自动包含trace_id，便于问题追踪
                    log.WithContext(traceCtx).WithFields(log.Fields{
                        "message_id": msg.ID,
                        "action":     msg.Values["action"],
                        "user_id":    msg.Values["user_id"],
                    }).Info("processing user event")
                    
                    // 业务处理逻辑
                    if err := handleUserEvent(traceCtx, msg.Values); err != nil {
                        log.WithContext(traceCtx).WithError(err).Error("failed to handle user event")
                        continue
                    }
                    
                    // 确认消息处理完成
                    mqClient.XAck(traceCtx, "user-events", "user-service", msg.ID)
                }
            }
        }() // 自动调用cleanup()，结束consumer span
    }
}
```

### Kafka插桩

```go
import "github.com/Plaud-AI/plaud-library-go/observability/instrumentation"

// 启用Kafka自动插桩
instrumentation.EnableKafkaAutoTracing()

// 创建生产者和消费者
producer, _ := instrumentation.CreateTracedKafkaProducer([]string{"localhost:9092"}, nil)
consumer, _ := instrumentation.CreateTracedKafkaConsumer([]string{"localhost:9092"}, nil)
```

## 配置选项

### 关键配置参数

```go
err := observability.InitObservability("service-name",
    // 基础配置
    observability.WithServiceVersion("1.0.0"),
    observability.WithEnvironment("production"),
    
    // 采样和性能
    observability.WithTraceSamplingRate(0.1),  // 10%采样率
    
    // 导出配置
    observability.WithOTLPEndpoint("http://collector:4317"),
    observability.WithConsoleExport(false),    // 生产环境关闭
    
    // 功能开关
    observability.WithEnhanceLogging(true),
    observability.WithAutoInstrumentHTTPClient(true),
)
```

### 采样说明

- `1.0` = 100%采集（开发环境）
- `0.1` = 10%采集（生产环境推荐）
- `0.0` = 关闭采集

采样器遵循OpenTelemetry标准，保证分布式链路的完整性。

## 工具函数

```go
// 状态检查
if observability.IsInitialized() { /* 已初始化 */ }

// 获取配置和版本
config := observability.GetConfig()
version := observability.GetVersion()

// 数据刷新和关闭
observability.ForceFlushObservability()  // 强制刷新
observability.ShutdownObservability()    // 优雅关闭
```

## 架构特点

- **三层设计**: 基础功能（trace ID）→ 数据收集 → 自动插桩
- **懒加载**: Provider按需初始化，解决循环依赖
- **零开销**: 基础模式几乎无性能影响
- **配置驱动**: 所有功能可通过配置启用/禁用

## 运行示例

```bash
# 查看示例
ls observability/examples/

# 运行基础示例
go run observability/examples/basic_usage.go

# 运行Gin示例
go run observability/examples/gin_example.go
```

## 注意事项

1. **生产环境**: 建议使用较低采样率（0.1）减少性能影响
2. **优雅关闭**: 应用退出时调用`ShutdownObservability()`
3. **日志关联**: 自动注入trace信息，方便问题定位
4. **依赖管理**: 主要依赖OpenTelemetry、logrus、redis、kafka客户端

## 与Python版本功能对比

| 功能 | Python | Go |
|------|--------|-----|
| 链路追踪 | ✅ | ✅ |
| 指标监控 | ✅ | ✅ |
| 日志增强 | ✅ | ✅ |
| HTTP插桩 | ✅ | ✅ |
| Redis插桩 | ✅ | ✅ |
| Kafka插桩 | ✅ | ✅ |
| 框架集成 | ✅ FastAPI | ✅ Gin | 