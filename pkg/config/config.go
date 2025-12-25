package config

import (
	"os"

	scaffoldconfig "github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
)

// ExternalServicesConfig 外部服务配置
type ExternalServicesConfig struct {
	PlaudAPI *PlaudAPIConfig `yaml:"plaud_api"`
}

// PlaudAPIConfig plaud-api 服务配置
type PlaudAPIConfig struct {
	BaseURL string `yaml:"base_url"`
}

// AppConfig 应用配置，扩展了 scaffold 的 AppConfig
type AppConfig struct {
	scaffoldconfig.AppConfig `yaml:",inline"`
	Services                 *ExternalServicesConfig `yaml:"services"`
}

// Parse 解析配置
func (p *AppConfig) Parse() error {
	return p.AppConfig.Parse()
}

// GetConfig 获取配置
func (p *AppConfig) GetConfig() *AppConfig {
	return p
}

// GetPlaudAPIBaseURL 获取 plaud-api 服务的 base URL
// 优先从配置文件读取，若未配置则从环境变量 PLAUD_API_URL 兜底
func (p *AppConfig) GetPlaudAPIBaseURL() string {
	if p.Services != nil && p.Services.PlaudAPI != nil && p.Services.PlaudAPI.BaseURL != "" {
		return p.Services.PlaudAPI.BaseURL
	}
	return os.Getenv("PLAUD_API_URL")
}

// PlaudAPIConfigGetter 用于获取 plaud-api 配置的接口
type PlaudAPIConfigGetter interface {
	GetPlaudAPIBaseURL() string
}
