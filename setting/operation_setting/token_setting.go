package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// TokenSetting 令牌相关配置
type TokenSetting struct {
	MaxUserTokens                  int    `json:"max_user_tokens"`                    // 每用户最大令牌数量
	SingleInputTokensLimit         int    `json:"single_input_tokens_limit"`          // 单次输入 tokens 上限，<= 0 表示不限制
	SingleInputTokensWarnThreshold int    `json:"single_input_tokens_warn_threshold"` // 达到多少次警告后禁用账号
	SingleInputTokensExemptGroups  string `json:"single_input_tokens_exempt_groups"`  // 单次输入 tokens 限制豁免分组
}

// 默认配置
var tokenSetting = TokenSetting{
	MaxUserTokens:                  1000, // 默认每用户最多 1000 个令牌
	SingleInputTokensLimit:         0,
	SingleInputTokensWarnThreshold: 3,
	SingleInputTokensExemptGroups:  "",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("token_setting", &tokenSetting)
}

// GetTokenSetting 获取令牌配置
func GetTokenSetting() *TokenSetting {
	return &tokenSetting
}

// GetMaxUserTokens 获取每用户最大令牌数量
func GetMaxUserTokens() int {
	return GetTokenSetting().MaxUserTokens
}

func GetSingleInputTokensLimit() int {
	return GetTokenSetting().SingleInputTokensLimit
}

func GetSingleInputTokensWarnThreshold() int {
	threshold := GetTokenSetting().SingleInputTokensWarnThreshold
	if threshold <= 0 {
		return 1
	}
	return threshold
}

func IsSingleInputTokensLimitEnabled() bool {
	return GetSingleInputTokensLimit() > 0
}

func GetSingleInputTokensExemptGroups() []string {
	raw := strings.TrimSpace(GetTokenSetting().SingleInputTokensExemptGroups)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	groups := make([]string, 0, len(parts))
	for _, part := range parts {
		group := strings.TrimSpace(part)
		if group == "" {
			continue
		}
		groups = append(groups, group)
	}
	return groups
}

func IsSingleInputTokensExemptGroup(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		return false
	}
	for _, exemptGroup := range GetSingleInputTokensExemptGroups() {
		if exemptGroup == group {
			return true
		}
	}
	return false
}
