package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSingleInputTokenLimit_WarnOnly(t *testing.T) {
	truncate(t)

	originalSetting := *operation_setting.GetTokenSetting()
	t.Cleanup(func() {
		*operation_setting.GetTokenSetting() = originalSetting
	})
	operation_setting.GetTokenSetting().SingleInputTokensLimit = 100
	operation_setting.GetTokenSetting().SingleInputTokensWarnThreshold = 3

	user := &model.User{
		Id:       401,
		Username: "warn_only_user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, model.DB.Create(user).Error)

	token := &model.Token{
		Id:          501,
		UserId:      user.Id,
		Key:         "warn-only-token",
		Name:        "warn-only-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 1000,
	}
	require.NoError(t, model.DB.Create(token).Error)

	result, err := HandleSingleInputTokenLimit(user.Id, token.Id, 120, user.Group, false)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.WarningCount)
	assert.False(t, result.UserDisabled)
	assert.Equal(t, "您一次性输入了 120 tokens，超出平台限制，已将您的令牌禁用一次，请规范使用行为", result.Message)

	var refreshedToken model.Token
	require.NoError(t, model.DB.First(&refreshedToken, token.Id).Error)
	assert.Equal(t, common.TokenStatusDisabled, refreshedToken.Status)

	var refreshedUser model.User
	require.NoError(t, model.DB.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, common.UserStatusEnabled, refreshedUser.Status)
	assert.Equal(t, 1, refreshedUser.InputTokenLimitWarnCount)
	assert.Empty(t, refreshedUser.Remark)
}

func TestHandleSingleInputTokenLimit_DisableUserOnThreshold(t *testing.T) {
	truncate(t)

	originalSetting := *operation_setting.GetTokenSetting()
	t.Cleanup(func() {
		*operation_setting.GetTokenSetting() = originalSetting
	})
	operation_setting.GetTokenSetting().SingleInputTokensLimit = 100
	operation_setting.GetTokenSetting().SingleInputTokensWarnThreshold = 2

	user := &model.User{
		Id:                       402,
		Username:                 "disable_user",
		Status:                   common.UserStatusEnabled,
		Group:                    "default",
		Remark:                   "已有备注",
		InputTokenLimitWarnCount: 1,
	}
	require.NoError(t, model.DB.Create(user).Error)

	token := &model.Token{
		Id:          502,
		UserId:      user.Id,
		Key:         "disable-user-token",
		Name:        "disable-user-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 1000,
	}
	require.NoError(t, model.DB.Create(token).Error)

	result, err := HandleSingleInputTokenLimit(user.Id, token.Id, 150, user.Group, false)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.WarningCount)
	assert.True(t, result.UserDisabled)
	assert.Contains(t, result.Message, "账号已被禁用")

	var refreshedUser model.User
	require.NoError(t, model.DB.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, refreshedUser.Status)
	assert.Equal(t, 2, refreshedUser.InputTokenLimitWarnCount)
	assert.Contains(t, refreshedUser.Remark, "单次输入 tokens 超限")
	assert.Contains(t, refreshedUser.Remark, "已有备注")
}

func TestHandleSingleInputTokenLimit_ExemptGroup(t *testing.T) {
	truncate(t)

	originalSetting := *operation_setting.GetTokenSetting()
	t.Cleanup(func() {
		*operation_setting.GetTokenSetting() = originalSetting
	})
	operation_setting.GetTokenSetting().SingleInputTokensLimit = 100
	operation_setting.GetTokenSetting().SingleInputTokensWarnThreshold = 2
	operation_setting.GetTokenSetting().SingleInputTokensExemptGroups = "vip, internal"

	user := &model.User{
		Id:       403,
		Username: "vip_user",
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}
	require.NoError(t, model.DB.Create(user).Error)

	token := &model.Token{
		Id:          503,
		UserId:      user.Id,
		Key:         "vip-token",
		Name:        "vip-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 1000,
	}
	require.NoError(t, model.DB.Create(token).Error)

	result, err := HandleSingleInputTokenLimit(user.Id, token.Id, 500, user.Group, false)
	require.NoError(t, err)
	assert.Nil(t, result)

	var refreshedToken model.Token
	require.NoError(t, model.DB.First(&refreshedToken, token.Id).Error)
	assert.Equal(t, common.TokenStatusEnabled, refreshedToken.Status)
}

func TestHandleSingleInputTokenLimit_AdminExempt(t *testing.T) {
	truncate(t)

	originalSetting := *operation_setting.GetTokenSetting()
	t.Cleanup(func() {
		*operation_setting.GetTokenSetting() = originalSetting
	})
	operation_setting.GetTokenSetting().SingleInputTokensLimit = 100
	operation_setting.GetTokenSetting().SingleInputTokensWarnThreshold = 2

	user := &model.User{
		Id:       404,
		Username: "admin_user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, model.DB.Create(user).Error)

	token := &model.Token{
		Id:          504,
		UserId:      user.Id,
		Key:         "admin-token",
		Name:        "admin-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 1000,
	}
	require.NoError(t, model.DB.Create(token).Error)

	result, err := HandleSingleInputTokenLimit(user.Id, token.Id, 500, user.Group, true)
	require.NoError(t, err)
	assert.Nil(t, result)

	var refreshedToken model.Token
	require.NoError(t, model.DB.First(&refreshedToken, token.Id).Error)
	assert.Equal(t, common.TokenStatusEnabled, refreshedToken.Status)
}
