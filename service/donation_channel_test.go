package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDonationChannel_SuccessAndDedup(t *testing.T) {
	truncate(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4o-mini"}]}`))
	}))
	defer server.Close()

	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})
	operation_setting.GetDonationSetting().Enabled = true
	operation_setting.GetDonationSetting().TemplateChannelID = 501
	operation_setting.GetDonationSetting().RewardQuota = 321

	user := &model.User{
		Id:       101,
		Username: "donor_user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, model.DB.Create(user).Error)

	template := &model.Channel{
		Id:          501,
		Type:        constant.ChannelTypeCustom,
		Key:         "template-key",
		Name:        "template",
		Status:      common.ChannelStatusEnabled,
		Models:      "gpt-4o-mini",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
		BaseURL:     common.GetPointer[string](server.URL),
	}
	require.NoError(t, model.DB.Create(template).Error)

	resp, err := CreateDonationChannel(user.Id, user.Username, dto.CreateDonationChannelRequest{
		BaseURL: server.URL,
		Key:     "real-key",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 321, resp.RewardQuota)
	assert.Equal(t, []string{"gpt-4o-mini"}, resp.FetchedModels)

	channel, err := model.GetChannelById(resp.Channel.Id, true)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, model.DonationChannelTag(user.Id), channel.GetTag())
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)

	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.NotEmpty(t, abilities)
	for _, ability := range abilities {
		assert.False(t, ability.Enabled)
	}

	var refreshedUser model.User
	require.NoError(t, model.DB.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, 1321, refreshedUser.Quota)

	var submissionCount int64
	require.NoError(t, model.DB.Model(&model.DonationChannelSubmission{}).Count(&submissionCount).Error)
	assert.Equal(t, int64(1), submissionCount)

	_, err = CreateDonationChannel(user.Id, user.Username, dto.CreateDonationChannelRequest{
		BaseURL: server.URL + "/",
		Key:     "real-key",
	})
	require.ErrorIs(t, err, ErrDonationChannelAlreadySubmitted)

	var channelCount int64
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("tag = ?", model.DonationChannelTag(user.Id)).Count(&channelCount).Error)
	assert.Equal(t, int64(1), channelCount)
}

func TestEnsureUserHasDonationChannel(t *testing.T) {
	truncate(t)

	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})
	operation_setting.GetDonationSetting().Enabled = true

	const userID = 202
	require.ErrorIs(t, EnsureUserHasDonationChannel(userID, false), ErrDonationChannelUnavailable)
	require.NoError(t, EnsureUserHasDonationChannel(userID, true))

	channel := &model.Channel{
		Id:          601,
		Type:        constant.ChannelTypeCustom,
		Key:         "key",
		Name:        "donation-channel",
		Status:      common.ChannelStatusEnabled,
		Models:      "gpt-4o-mini",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
		Tag:         common.GetPointer[string](model.DonationChannelTag(userID)),
	}
	require.NoError(t, model.DB.Create(channel).Error)

	require.NoError(t, EnsureUserHasDonationChannel(userID, false))
}
