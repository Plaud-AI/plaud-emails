package telemetry

import (
	"fmt"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/Plaud-AI/plaud-library-go/env"
	"github.com/Plaud-AI/plaud-library-go/observability"
)

// InitObservability 初始化 observability，返回 shutdown 函数
// 类似于 context.WithCancel() 的模式，确保初始化和清理成对出现
func InitObservability(appName string, cfg *config.ObservabilityConfig, cfgEnv string) (shutdownFunc func(), err error) {
	// 如果配置为空或者禁用，则跳过初始化
	if cfg == nil || !cfg.Enabled || (cfgEnv == env.DevelopEnv || cfgEnv == env.LocalEnv) {
		logger.Infof("Observability is disabled for %s, env:%s", appName, cfgEnv)
		return func() {}, nil
	}

	// 从配置构建observability选项
	observabilityOptions := []observability.InitOption{
		observability.WithEnvironment(env.GetEnv()),
		observability.WithConsoleExport(cfg.ConsoleExport),
		observability.WithTraceSamplingRate(cfg.TraceSamplingRate),
		observability.WithEnhanceLogging(cfg.EnhanceLogging),
		observability.WithEnablePerformanceMonitoring(cfg.EnablePerformanceMonitoring),
	}

	// 配置OTLP端点（如果启用的话）
	if cfg.OTLP != nil && cfg.OTLP.Enabled && cfg.OTLP.Endpoint != "" {
		observabilityOptions = append(observabilityOptions, observability.WithOTLPEndpoint(cfg.OTLP.Endpoint))
		logger.Infof("OTLP endpoint configured for %s: %s", appName, cfg.OTLP.Endpoint)
	} else {
		logger.Infof("OTLP export disabled for %s - using console export only", appName)
	}

	// 初始化 observability
	if err := observability.InitObservability(appName, observabilityOptions...); err != nil {
		return nil, fmt.Errorf("init observability failed: %w", err)
	}

	logger.Infof("Enhanced observability initialized for %s with metrics and logging", appName)

	// 返回 shutdown 函数，包含错误处理和日志
	shutdownFunc = func() {
		if err := observability.ShutdownObservability(); err != nil {
			logger.Errorf("shutdown observability failed for %s: %v", appName, err)
		} else {
			logger.Infof("Enhanced observability shutdown completed for %s", appName)
		}
	}

	return shutdownFunc, nil
}
