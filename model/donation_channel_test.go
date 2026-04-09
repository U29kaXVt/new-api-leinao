package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteDisabledChannel_KeepDonationTemplate(t *testing.T) {
	truncateTables(t)

	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})
	operation_setting.GetDonationSetting().TemplateChannelID = 2

	channels := []Channel{
		{Id: 1, Name: "disabled", Key: "key-1", Status: common.ChannelStatusAutoDisabled, Models: "gpt-4o-mini", Group: "default"},
		{Id: 2, Name: "template", Key: "key-2", Status: common.ChannelStatusManuallyDisabled, Models: "gpt-4o-mini", Group: "default"},
		{Id: 3, Name: "enabled", Key: "key-3", Status: common.ChannelStatusEnabled, Models: "gpt-4o-mini", Group: "default"},
	}
	require.NoError(t, DB.Create(&channels).Error)
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "gpt-4o-mini", ChannelId: 1, Enabled: false}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "gpt-4o-mini", ChannelId: 2, Enabled: false}).Error)

	rows, err := DeleteDisabledChannel()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	var count int64
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 2).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ?", 2).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
