package core

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// LazyTracerProvider 延迟初始化的TracerProvider代理
// 该Provider作为代理，在库被导入时立即设置。
// 它最初将所有调用委托给一个NoOp（无操作）Provider。
// 当 InitObservability 被调用时，它会将内部的NoOp Provider
// 替换为一个完全配置好的真实SDK Provider。
type LazyTracerProvider struct {
	mu           sync.RWMutex
	realProvider trace.TracerProvider
}

// NewLazyTracerProvider 创建新的懒加载TracerProvider
func NewLazyTracerProvider() *LazyTracerProvider {
	return &LazyTracerProvider{
		realProvider: noop.NewTracerProvider(),
	}
}

// Tracer 获取一个Tracer，委托给真实的Provider
func (p *LazyTracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.realProvider.Tracer(name, options...)
}

// SetProvider 设置真实的SDK Provider，完成初始化
func (p *LazyTracerProvider) SetProvider(provider *sdktrace.TracerProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.realProvider = provider
}

// Shutdown 关闭真实的Provider
func (p *LazyTracerProvider) Shutdown(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if sdkProvider, ok := p.realProvider.(*sdktrace.TracerProvider); ok {
		return sdkProvider.Shutdown(ctx)
	}
	return nil
}

// ForceFlush 强制刷新真实的Provider
func (p *LazyTracerProvider) ForceFlush(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if sdkProvider, ok := p.realProvider.(*sdktrace.TracerProvider); ok {
		return sdkProvider.ForceFlush(ctx)
	}
	return nil
}

// LazyMeterProvider 延迟初始化的MeterProvider代理
type LazyMeterProvider struct {
	mu           sync.RWMutex
	realProvider metric.MeterProvider
}

// NewLazyMeterProvider 创建新的懒加载MeterProvider
func NewLazyMeterProvider() *LazyMeterProvider {
	return &LazyMeterProvider{
		realProvider: noopmetric.NewMeterProvider(),
	}
}

// Meter 获取一个Meter，委托给真实的Provider
func (p *LazyMeterProvider) Meter(name string, options ...metric.MeterOption) metric.Meter {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.realProvider.Meter(name, options...)
}

// SetProvider 设置真实的SDK Provider，完成初始化
func (p *LazyMeterProvider) SetProvider(provider *sdkmetric.MeterProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.realProvider = provider
}

// Shutdown 关闭真实的Provider
func (p *LazyMeterProvider) Shutdown(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if sdkProvider, ok := p.realProvider.(*sdkmetric.MeterProvider); ok {
		return sdkProvider.Shutdown(ctx)
	}
	return nil
}

// ForceFlush 强制刷新真实的Provider
func (p *LazyMeterProvider) ForceFlush(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if sdkProvider, ok := p.realProvider.(*sdkmetric.MeterProvider); ok {
		return sdkProvider.ForceFlush(ctx)
	}
	return nil
}

// 全局的懒加载Providers
var (
	globalLazyTracerProvider *LazyTracerProvider
	globalLazyMeterProvider  *LazyMeterProvider
	initOnce                 sync.Once
)

// initGlobalProviders 初始化全局的懒加载Providers
func initGlobalProviders() {
	initOnce.Do(func() {
		globalLazyTracerProvider = NewLazyTracerProvider()
		globalLazyMeterProvider = NewLazyMeterProvider()
	})
}

// GetGlobalTracerProvider 获取全局的懒加载TracerProvider
func GetGlobalTracerProvider() *LazyTracerProvider {
	initGlobalProviders()
	return globalLazyTracerProvider
}

// GetGlobalMeterProvider 获取全局的懒加载MeterProvider
func GetGlobalMeterProvider() *LazyMeterProvider {
	initGlobalProviders()
	return globalLazyMeterProvider
}

// GetTracer 获取全局Tracer
func GetTracer(name string, options ...trace.TracerOption) trace.Tracer {
	return GetGlobalTracerProvider().Tracer(name, options...)
}

// GetMeter 获取全局Meter
func GetMeter(name string, options ...metric.MeterOption) metric.Meter {
	return GetGlobalMeterProvider().Meter(name, options...)
}
