package config

import (
	"context"
	"fmt"
	"os"
)

// LocalConfigLoader 本地配置加载器
type LocalConfigLoader[T Configurer] struct {
	appName    string
	envName    string
	configName string
	cfgp       T

	configVarResolver ConfigVarResolver
}

func NewLocalConfigLoader[T Configurer](appName, envName, configName string) *LocalConfigLoader[T] {
	return &LocalConfigLoader[T]{
		appName:    appName,
		envName:    envName,
		configName: configName,
	}
}

func (l *LocalConfigLoader[T]) Init() error {
	data, err := os.ReadFile(l.configName)
	if err != nil {
		return err
	}

	if l.configVarResolver != nil {
		replaced, err := l.configVarResolver.ResolveConfigVars(context.Background(), string(data), LoadModeLocal, l.appName, l.envName)
		if err != nil {
			err = fmt.Errorf("failed to resolve config vars, err:%w", err)
			return err
		}
		data = []byte(replaced)
	}

	if err := LoadYAMl(data, &l.cfgp); err != nil {
		return err
	}
	if err := l.cfgp.Parse(); err != nil {
		return err
	}

	if awsAppNameSetter, ok := any(l.cfgp).(AppNameSetter); ok {
		awsAppNameSetter.SetAppName(l.appName)
	}
	if envSetter, ok := any(l.cfgp).(EnvSetter); ok {
		envSetter.SetEnv(l.envName)
	}
	if configPathSetter, ok := any(l.cfgp).(ConfigPathSetter); ok {
		configPathSetter.SetConfigPath(l.configName)
	}
	if loadModeSetter, ok := any(l.cfgp).(LoadModeSetter); ok {
		loadModeSetter.SetLoadMode(LoadModeLocal)
	}
	return nil
}

func (l *LocalConfigLoader[T]) GetConfig() T {
	return l.cfgp
}

func (l *LocalConfigLoader[T]) GetEnv() string {
	return l.envName
}

func (l *LocalConfigLoader[T]) GetConfigPath() string {
	return l.configName
}

func (l *LocalConfigLoader[T]) GetLoadMode() LoadMode {
	return LoadModeLocal
}

func (l *LocalConfigLoader[T]) GetAppName() string {
	return l.appName
}

func (l *LocalConfigLoader[T]) SetConfigVarResolver(configVarResolver ConfigVarResolver) {
	l.configVarResolver = configVarResolver
}
