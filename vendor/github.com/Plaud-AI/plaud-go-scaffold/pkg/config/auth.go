package config

// AuthConfig 认证配置
type AuthConfig struct {
	// JWT配置 有关的配置
	JWT struct {
		Secret        string   `yaml:"secret"`
		ExpireSeconds int64    `yaml:"expire_seconds"`
		TestList      []string `yaml:"test_list"`
	} `yaml:"jwt"`
	// API Key配置
	APIKey []struct {
		Name string `yaml:"name"`
		Key  string `yaml:"key"`
	} `yaml:"api_key"`
	// 白名单
	Whitelist []string `yaml:"whitelist"`
	// 黑名单
	Blacklist []string `yaml:"blacklist"`

	apiKeyMap    map[string]struct{}
	whitelistMap map[string]struct{}
	blacklistMap map[string]struct{}
}

// Parse 解析认证配置
func (p *AuthConfig) Parse() error {
	if p == nil {
		return nil
	}

	p.apiKeyMap = make(map[string]struct{})
	p.whitelistMap = make(map[string]struct{})
	p.blacklistMap = make(map[string]struct{})

	for _, apiKey := range p.APIKey {
		p.apiKeyMap[apiKey.Key] = struct{}{}
	}
	for _, whitelist := range p.Whitelist {
		p.whitelistMap[whitelist] = struct{}{}
	}
	for _, blacklist := range p.Blacklist {
		p.blacklistMap[blacklist] = struct{}{}
	}

	return nil
}

func (p *AuthConfig) IsAPIKey(apiKey string) bool {
	if p == nil {
		return false
	}
	_, ok := p.apiKeyMap[apiKey]
	return ok
}

func (p *AuthConfig) IsWhitelist(ip string) bool {
	if p == nil {
		return false
	}
	_, ok := p.whitelistMap[ip]
	return ok
}

func (p *AuthConfig) IsBlacklist(ip string) bool {
	if p == nil {
		return false
	}
	_, ok := p.blacklistMap[ip]
	return ok
}
