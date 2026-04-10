package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type DiscordSettings struct {
	Enabled                bool                `json:"enabled"`
	ClientId               string              `json:"client_id"`
	ClientSecret           string              `json:"client_secret"`
	AccessRules            []DiscordAccessRule `json:"access_rules"`
	AccessRulesIgnoreLogin bool                `json:"access_rules_ignore_login"`
}

type DiscordAccessRule struct {
	GuildID string   `json:"guild_id"`
	RoleIDs []string `json:"role_ids"`
}

// 默认配置
var defaultDiscordSettings = DiscordSettings{}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("discord", &defaultDiscordSettings)
}

func GetDiscordSettings() *DiscordSettings {
	return &defaultDiscordSettings
}

func normalizeDiscordIDList(ids []string) []string {
	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))

	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func (s *DiscordSettings) GetAccessRules() []DiscordAccessRule {
	if len(s.AccessRules) == 0 {
		return nil
	}

	type mergedRule struct {
		rule     DiscordAccessRule
		roleSet  map[string]struct{}
		allowAll bool
	}

	merged := make([]*mergedRule, 0, len(s.AccessRules))
	indexByGuild := make(map[string]int, len(s.AccessRules))

	for _, rawRule := range s.AccessRules {
		guildID := strings.TrimSpace(rawRule.GuildID)
		if guildID == "" {
			continue
		}

		roleIDs := normalizeDiscordIDList(rawRule.RoleIDs)
		idx, exists := indexByGuild[guildID]
		if !exists {
			item := &mergedRule{
				rule: DiscordAccessRule{
					GuildID: guildID,
				},
				roleSet: make(map[string]struct{}),
			}
			if len(roleIDs) == 0 {
				item.allowAll = true
			} else {
				item.rule.RoleIDs = make([]string, 0, len(roleIDs))
				for _, roleID := range roleIDs {
					item.roleSet[roleID] = struct{}{}
					item.rule.RoleIDs = append(item.rule.RoleIDs, roleID)
				}
			}
			indexByGuild[guildID] = len(merged)
			merged = append(merged, item)
			continue
		}

		item := merged[idx]
		if item.allowAll || len(roleIDs) == 0 {
			item.allowAll = true
			item.rule.RoleIDs = nil
			item.roleSet = map[string]struct{}{}
			continue
		}

		for _, roleID := range roleIDs {
			if _, ok := item.roleSet[roleID]; ok {
				continue
			}
			item.roleSet[roleID] = struct{}{}
			item.rule.RoleIDs = append(item.rule.RoleIDs, roleID)
		}
	}

	normalized := make([]DiscordAccessRule, 0, len(merged))
	for _, item := range merged {
		if item.allowAll {
			item.rule.RoleIDs = nil
		}
		normalized = append(normalized, item.rule)
	}

	return normalized
}

func (s *DiscordSettings) RequiresMemberVerification() bool {
	return len(s.GetAccessRules()) > 0
}

func (s *DiscordSettings) ShouldVerifyAccess(flow string) bool {
	if !s.RequiresMemberVerification() {
		return false
	}
	if s.AccessRulesIgnoreLogin && flow == "login" {
		return false
	}
	return true
}

func (s *DiscordSettings) GetOAuthScopes() string {
	scopes := []string{"identify", "openid"}
	if s.RequiresMemberVerification() {
		scopes = append(scopes, "guilds.members.read")
	}
	return strings.Join(scopes, " ")
}
