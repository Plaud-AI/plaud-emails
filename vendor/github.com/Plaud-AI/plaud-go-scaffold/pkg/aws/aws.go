package aws

import "os"

// GetAWSRegion 从环境变量获取AWS区域
func GetAWSRegion() string {
	return os.Getenv("AWS_REGION")
}

// GetAWSAccessKeyID 从环境变量获取AWS访问密钥ID
func GetAWSAccessKeyID() string {
	return os.Getenv("AWS_ACCESS_KEY_ID")
}

// GetAWSSecretAccessKey 从环境变量获取AWS秘密访问密钥
func GetAWSSecretAccessKey() string {
	return os.Getenv("AWS_SECRET_ACCESS_KEY")
}

// GetAWSAppConfigName 从环境变量获取AWS应用配置名称(AppConfig的ApplicationName)
func GetAWSAppConfigName() string {
	return os.Getenv("AWS_APPCONFIG_APPLICATION_NAME")
}
