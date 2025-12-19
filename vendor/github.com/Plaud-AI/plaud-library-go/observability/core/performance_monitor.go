package core

import (
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	enabled           bool
	goroutineBaseline int
	mutex             sync.RWMutex
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

// GlobalPerformanceMonitor 全局性能监控器实例
var (
	globalMonitor *PerformanceMonitor
	monitorOnce   sync.Once
)

// GetPerformanceMonitor 获取全局性能监控器
func GetPerformanceMonitor() *PerformanceMonitor {
	monitorOnce.Do(func() {
		globalMonitor = &PerformanceMonitor{
			enabled:           false,
			goroutineBaseline: runtime.NumGoroutine(),
			stopChan:          make(chan struct{}),
		}
	})
	return globalMonitor
}

// Start 启动性能监控
func (pm *PerformanceMonitor) Start() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.enabled {
		return
	}

	pm.enabled = true
	pm.goroutineBaseline = runtime.NumGoroutine()

	pm.wg.Add(1)
	go pm.monitor()

	logrus.Info("性能监控已启动")
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.enabled {
		return
	}

	pm.enabled = false
	close(pm.stopChan)
	pm.wg.Wait()

	// 重新创建stopChan为下次启动准备
	pm.stopChan = make(chan struct{})

	logrus.Info("性能监控已停止")
}

// monitor 监控循环
func (pm *PerformanceMonitor) monitor() {
	defer pm.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.checkPerformance()
		case <-pm.stopChan:
			return
		}
	}
}

// checkPerformance 检查性能指标
func (pm *PerformanceMonitor) checkPerformance() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	currentGoroutines := runtime.NumGoroutine()
	goroutineIncrease := currentGoroutines - pm.goroutineBaseline

	// 内存使用（MB）
	allocMB := float64(memStats.Alloc) / 1024 / 1024
	sysMB := float64(memStats.Sys) / 1024 / 1024

	// 创建监控日志条目
	logEntry := logrus.WithFields(logrus.Fields{
		"goroutines":         currentGoroutines,
		"goroutine_increase": goroutineIncrease,
		"memory_alloc_mb":    allocMB,
		"memory_sys_mb":      sysMB,
		"gc_cycles":          memStats.NumGC,
	})

	// 检查是否有问题
	hasIssue := false

	// Goroutine泄漏检查
	if goroutineIncrease > 100 {
		logEntry = logEntry.WithField("issue", "potential_goroutine_leak")
		hasIssue = true
	}

	// 内存使用检查
	if allocMB > 500 { // 超过500MB
		logEntry = logEntry.WithField("issue", "high_memory_usage")
		hasIssue = true
	}

	// GC压力检查
	if memStats.NumGC > 0 && memStats.PauseTotalNs > 100*1000*1000 { // 超过100ms总GC时间
		logEntry = logEntry.WithField("issue", "gc_pressure")
		hasIssue = true
	}

	if hasIssue {
		logEntry.Warn("🚨 检测到潜在性能问题")
	} else {
		logEntry.Debug("✅ 性能指标正常")
	}
}

// GetCurrentStats 获取当前性能统计
func (pm *PerformanceMonitor) GetCurrentStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	currentGoroutines := runtime.NumGoroutine()

	return map[string]interface{}{
		"goroutines":         currentGoroutines,
		"goroutine_baseline": pm.goroutineBaseline,
		"goroutine_increase": currentGoroutines - pm.goroutineBaseline,
		"memory_alloc_bytes": memStats.Alloc,
		"memory_sys_bytes":   memStats.Sys,
		"memory_alloc_mb":    float64(memStats.Alloc) / 1024 / 1024,
		"memory_sys_mb":      float64(memStats.Sys) / 1024 / 1024,
		"gc_cycles":          memStats.NumGC,
		"gc_pause_total_ns":  memStats.PauseTotalNs,
		"monitoring_enabled": pm.enabled,
	}
}

// IsEnabled 检查监控是否启用
func (pm *PerformanceMonitor) IsEnabled() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.enabled
}
