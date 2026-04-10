package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("discord", &DiscordProvider{})
}

// DiscordProvider implements OAuth for Discord
type DiscordProvider struct{}

type discordOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type discordUser struct {
	UID  string `json:"id"`
	ID   string `json:"username"`
	Name string `json:"global_name"`
}

type discordGuildMember struct {
	Roles []string `json:"roles"`
}

func (p *DiscordProvider) GetName() string {
	return "Discord"
}

func (p *DiscordProvider) IsEnabled() bool {
	return system_setting.GetDiscordSettings().Enabled
}

func (p *DiscordProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	settings := system_setting.GetDiscordSettings()
	redirectUri := fmt.Sprintf("%s/oauth/discord", system_setting.ServerAddress)
	values := url.Values{}
	values.Set("client_id", settings.ClientId)
	values.Set("client_secret", settings.ClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectUri)

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken: redirect_uri=%s", redirectUri)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://discord.com/api/v10/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken response status: %d", res.StatusCode)

	var discordResponse discordOAuthResponse
	err = common.DecodeJson(res.Body, &discordResponse)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if discordResponse.AccessToken == "" {
		logger.LogError(ctx, "[OAuth-Discord] ExchangeToken failed: empty access token")
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "Discord"})
	}

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken success: scope=%s", discordResponse.Scope)

	return &OAuthToken{
		AccessToken:  discordResponse.AccessToken,
		TokenType:    discordResponse.TokenType,
		RefreshToken: discordResponse.RefreshToken,
		ExpiresIn:    discordResponse.ExpiresIn,
		Scope:        discordResponse.Scope,
		IDToken:      discordResponse.IDToken,
	}, nil
}

func (p *DiscordProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo: fetching user info")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo response status: %d", res.StatusCode)

	if res.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetUserInfo failed: status=%d", res.StatusCode))
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, nil)
	}

	var discordUser discordUser
	err = common.DecodeJson(res.Body, &discordUser)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	if discordUser.UID == "" || discordUser.ID == "" {
		logger.LogError(ctx, "[OAuth-Discord] GetUserInfo failed: empty user fields")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "Discord"})
	}

	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo success: uid=%s, username=%s, name=%s", discordUser.UID, discordUser.ID, discordUser.Name)

	return &OAuthUser{
		ProviderUserID: discordUser.UID,
		Username:       discordUser.ID,
		DisplayName:    discordUser.Name,
	}, nil
}

func (p *DiscordProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsDiscordIdAlreadyTaken(providerUserID)
}

func (p *DiscordProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.DiscordId = providerUserID
	return user.FillUserByDiscordId()
}

func (p *DiscordProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.DiscordId = providerUserID
}

func (p *DiscordProvider) GetProviderPrefix() string {
	return "discord_"
}

func (p *DiscordProvider) ValidateAccess(ctx context.Context, token *OAuthToken, oauthUser *OAuthUser, flow OAuthAccessFlow) error {
	settings := system_setting.GetDiscordSettings()
	if !settings.ShouldVerifyAccess(string(flow)) {
		return nil
	}
	rules := settings.GetAccessRules()
	if len(rules) == 0 {
		return nil
	}

	client := http.Client{Timeout: 5 * time.Second}
	memberMatched := false
	userID := oauthUser.ProviderUserID

	for _, rule := range rules {
		member, err := p.getCurrentGuildMember(ctx, &client, token.AccessToken, rule.GuildID)
		if err != nil {
			if _, ok := err.(*discordGuildNotFoundError); ok {
				logger.LogDebug(ctx, fmt.Sprintf("[OAuth-Discord] user %s is not in guild %s", userID, rule.GuildID))
				continue
			}
			return err
		}

		memberMatched = true
		if len(rule.RoleIDs) == 0 {
			logger.LogDebug(ctx, fmt.Sprintf("[OAuth-Discord] access granted by guild membership: user=%s guild=%s", userID, rule.GuildID))
			return nil
		}

		if hasAnyDiscordRole(member.Roles, rule.RoleIDs) {
			logger.LogDebug(ctx, fmt.Sprintf("[OAuth-Discord] access granted by role match: user=%s guild=%s", userID, rule.GuildID))
			return nil
		}
	}

	if !memberMatched {
		logger.LogWarn(ctx, fmt.Sprintf("[OAuth-Discord] access denied: user=%s is not in any allowed guild", userID))
		return NewOAuthError(i18n.MsgOAuthDiscordGuildRequired, nil)
	}

	logger.LogWarn(ctx, fmt.Sprintf("[OAuth-Discord] access denied: user=%s missing all required roles", userID))
	return NewOAuthError(i18n.MsgOAuthDiscordRoleRequired, nil)
}

type discordGuildNotFoundError struct{}

func (e *discordGuildNotFoundError) Error() string {
	return "discord guild member not found"
}

func hasAnyDiscordRole(userRoleIDs []string, allowedRoleIDs []string) bool {
	roleSet := make(map[string]struct{}, len(userRoleIDs))
	for _, roleID := range userRoleIDs {
		trimmed := strings.TrimSpace(roleID)
		if trimmed == "" {
			continue
		}
		roleSet[trimmed] = struct{}{}
	}
	for _, roleID := range allowedRoleIDs {
		if _, ok := roleSet[strings.TrimSpace(roleID)]; ok {
			return true
		}
	}
	return false
}

func (p *DiscordProvider) getCurrentGuildMember(ctx context.Context, client *http.Client, accessToken string, guildID string) (*discordGuildMember, error) {
	memberEndpoint := fmt.Sprintf("https://discord.com/api/v10/users/@me/guilds/%s/member", guildID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, memberEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetCurrentGuildMember error: guild=%s err=%s", guildID, err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, fmt.Sprintf("[OAuth-Discord] GetCurrentGuildMember response: guild=%s status=%d", guildID, res.StatusCode))

	switch res.StatusCode {
	case http.StatusOK:
		var member discordGuildMember
		if err := common.DecodeJson(res.Body, &member); err != nil {
			logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetCurrentGuildMember decode error: guild=%s err=%s", guildID, err.Error()))
			return nil, err
		}
		return &member, nil
	case http.StatusNotFound:
		return nil, &discordGuildNotFoundError{}
	case http.StatusForbidden:
		logger.LogWarn(ctx, fmt.Sprintf("[OAuth-Discord] GetCurrentGuildMember forbidden: guild=%s", guildID))
		return nil, NewOAuthError(i18n.MsgOAuthDiscordMemberScopeRequired, nil)
	case http.StatusUnauthorized:
		logger.LogWarn(ctx, fmt.Sprintf("[OAuth-Discord] GetCurrentGuildMember unauthorized: guild=%s", guildID))
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "Discord"})
	default:
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetCurrentGuildMember failed: guild=%s status=%d", guildID, res.StatusCode))
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "Discord"})
	}
}
