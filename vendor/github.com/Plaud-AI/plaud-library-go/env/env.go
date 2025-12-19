package env

import (
	"errors"
)

// env const
const (
	LocalEnv   = "local"
	DevelopEnv = "dev"
	TestEnv    = "test"
	PreviewEnv = "preview"
	ProductEnv = "prod"
)

// current
var env string

// SetEnv 设置当前环境，仅第一次能被设置成功
func SetEnv(envParam string) error {
	if env != "" {
		return nil
	}
	if envParam != LocalEnv && envParam != DevelopEnv && envParam != TestEnv && envParam != ProductEnv && envParam != PreviewEnv {
		return errors.New("envParam value is invalid")
	}
	env = envParam
	return nil
}

// GetEnv 获取当前环境， 如果没有设置返回默认值"develop"
func GetEnv() string {
	if env != "" {
		return env
	}
	return DevelopEnv
}

// IsProd
// @Description: 是否生产环境.
// @return bool
func IsProd() bool {
	return env == ProductEnv
}

// IsPreview
// @Description: 是否仿真环境.
// @return bool
func IsPreview() bool {
	return env == PreviewEnv
}

// IsTest
// @Description: 是否测试环境.
// @return bool
func IsTest() bool {
	return env == TestEnv
}

// IsDevelop
// @Description: 是否测试环境.
// @return bool
func IsDevelop() bool {
	return env == DevelopEnv
}

// IsLocal
// @Description: 是否本地环境.
// @return bool
func IsLocal() bool {
	return env == LocalEnv
}
