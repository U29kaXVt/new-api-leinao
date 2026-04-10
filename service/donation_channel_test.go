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
		if r.URL.Path != "/api/v1/models" {
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
		BaseURL:     server.URL,
		ChannelName: "我的渠道",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 321, resp.RewardQuota)
	assert.Equal(t, []string{"gpt-4o-mini"}, resp.FetchedModels)
	assert.Equal(t, "我的渠道", resp.Channel.Name)

	channel, err := model.GetChannelById(resp.Channel.Id, true)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, model.DonationChannelTag(user.Id), channel.GetTag())
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
	assert.Equal(t, "template-key", channel.Key)
	assert.Equal(t, "我的渠道", channel.Name)
	assert.Equal(t, server.URL+"/api", channel.GetBaseURL())

	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.NotEmpty(t, abilities)
	for _, ability := range abilities {
		assert.True(t, ability.Enabled)
	}

	var refreshedUser model.User
	require.NoError(t, model.DB.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, 1321, refreshedUser.Quota)

	_, err = CreateDonationChannel(user.Id, user.Username, dto.CreateDonationChannelRequest{
		BaseURL: server.URL + "/",
	})
	require.ErrorIs(t, err, ErrDonationChannelAlreadySubmitted)

	var channelCount int64
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("tag = ?", model.DonationChannelTag(user.Id)).Count(&channelCount).Error)
	assert.Equal(t, int64(1), channelCount)

	otherUser := &model.User{
		Id:       102,
		Username: "another_user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "another-user-aff",
	}
	require.NoError(t, model.DB.Create(otherUser).Error)

	_, err = CreateDonationChannel(otherUser.Id, otherUser.Username, dto.CreateDonationChannelRequest{
		BaseURL: server.URL,
	})
	require.ErrorIs(t, err, ErrDonationChannelAlreadySubmitted)
}

func TestCreateDonationChannel_AllowsDisabledSameDomainWithoutDeletingOld(t *testing.T) {
	truncate(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
	}))
	defer server.Close()

	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})
	operation_setting.GetDonationSetting().Enabled = true
	operation_setting.GetDonationSetting().TemplateChannelID = 801
	operation_setting.GetDonationSetting().RewardQuota = 100

	oldUser := &model.User{
		Id:       111,
		Username: "old_user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "old-user-aff",
	}
	newUser := &model.User{
		Id:       112,
		Username: "new_user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "new-user-aff",
	}
	require.NoError(t, model.DB.Create(oldUser).Error)
	require.NoError(t, model.DB.Create(newUser).Error)

	template := &model.Channel{
		Id:          801,
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

	oldChannel := &model.Channel{
		Id:          802,
		Type:        constant.ChannelTypeCustom,
		Key:         "template-key",
		Name:        "old-disabled",
		Status:      common.ChannelStatusManuallyDisabled,
		Models:      "gpt-4o-mini",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
		BaseURL:     common.GetPointer[string](server.URL + "/api"),
		Tag:         common.GetPointer[string](model.DonationChannelTag(oldUser.Id)),
	}
	require.NoError(t, oldChannel.Insert())

	resp, err := CreateDonationChannel(newUser.Id, newUser.Username, dto.CreateDonationChannelRequest{
		BaseURL: server.URL + "/foo/bar",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	var oldOwnerCount int64
	require.NoError(t, model.DB.Model(&model.Channel{}).
		Where("tag = ?", model.DonationChannelTag(oldUser.Id)).
		Where("base_url = ?", server.URL+"/api").
		Count(&oldOwnerCount).Error)
	assert.Equal(t, int64(1), oldOwnerCount)

	newChannel, err := model.GetChannelById(resp.Channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, model.DonationChannelTag(newUser.Id), newChannel.GetTag())
	assert.Equal(t, server.URL+"/api", newChannel.GetBaseURL())

	var reloadedNewUser model.User
	require.NoError(t, model.DB.First(&reloadedNewUser, newUser.Id).Error)
	assert.Equal(t, 1100, reloadedNewUser.Quota)

	var domainCount int64
	require.NoError(t, model.DB.Model(&model.Channel{}).
		Where("base_url = ?", server.URL+"/api").
		Where("tag LIKE ?", "donor_user_%").
		Count(&domainCount).Error)
	assert.Equal(t, int64(2), domainCount)
}

func TestEnsureUserHasDonationChannel(t *testing.T) {
	truncate(t)

	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})
	operation_setting.GetDonationSetting().Enabled = true

	const userID = 202
	require.ErrorIs(t, EnsureUserHasDonationChannel(userID, "default", false), ErrDonationChannelUnavailable)
	require.NoError(t, EnsureUserHasDonationChannel(userID, "default", true))

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

	require.NoError(t, EnsureUserHasDonationChannel(userID, "default", false))
}

func TestGetDonationChannelItems(t *testing.T) {
	truncate(t)

	channels := []*model.Channel{
		{
			Id:          701,
			Type:        constant.ChannelTypeCustom,
			Key:         "key-1",
			Name:        "mine-enabled",
			Status:      common.ChannelStatusEnabled,
			Models:      "gpt-4o-mini",
			Group:       "default",
			CreatedTime: common.GetTimestamp() - 10,
			Tag:         common.GetPointer[string](model.DonationChannelTag(301)),
		},
		{
			Id:          702,
			Type:        constant.ChannelTypeCustom,
			Key:         "key-2",
			Name:        "other-enabled",
			Status:      common.ChannelStatusEnabled,
			Models:      "gpt-4o-mini",
			Group:       "default",
			CreatedTime: common.GetTimestamp(),
			Tag:         common.GetPointer[string](model.DonationChannelTag(302)),
		},
		{
			Id:          703,
			Type:        constant.ChannelTypeCustom,
			Key:         "key-3",
			Name:        "mine-disabled",
			Status:      common.ChannelStatusManuallyDisabled,
			Models:      "gpt-4o-mini",
			Group:       "default",
			CreatedTime: common.GetTimestamp(),
			Tag:         common.GetPointer[string](model.DonationChannelTag(301)),
		},
		{
			Id:          704,
			Type:        constant.ChannelTypeCustom,
			Key:         "key-4",
			Name:        "non-donation",
			Status:      common.ChannelStatusEnabled,
			Models:      "gpt-4o-mini",
			Group:       "default",
			CreatedTime: common.GetTimestamp(),
		},
	}

	for _, channel := range channels {
		require.NoError(t, model.DB.Create(channel).Error)
	}

	items, err := GetDonationChannelItems(301)
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, 702, items[0].Id)
	assert.False(t, items[0].IsMine)
	assert.Equal(t, "other-enabled", items[0].Name)

	assert.Equal(t, 701, items[1].Id)
	assert.True(t, items[1].IsMine)
	assert.Equal(t, "mine-enabled", items[1].Name)
}

func TestCreateDonationChannel_UsesAutoNameWhenChannelNameEmpty(t *testing.T) {
	truncate(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
	}))
	defer server.Close()

	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})
	operation_setting.GetDonationSetting().Enabled = true
	operation_setting.GetDonationSetting().TemplateChannelID = 901

	user := &model.User{
		Id:       120,
		Username: "named_user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "named-user-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)

	template := &model.Channel{
		Id:          901,
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
		BaseURL:     server.URL,
		ChannelName: "   ",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Regexp(t, `^named_user#[0-9a-f]{8}$`, resp.Channel.Name)
}

func TestGetDonationLeaderboardItems(t *testing.T) {
	truncate(t)

	users := []*model.User{
		{Id: 401, Username: "alice", DisplayName: "Alice", Status: common.UserStatusEnabled, Group: "default", AffCode: "alice"},
		{Id: 402, Username: "bob", DisplayName: "", Status: common.UserStatusEnabled, Group: "default", AffCode: "bob"},
		{Id: 403, Username: "carol", DisplayName: "Carol", Status: common.UserStatusEnabled, Group: "default", AffCode: "carol"},
	}
	for _, user := range users {
		require.NoError(t, model.DB.Create(user).Error)
	}

	channels := []*model.Channel{
		{Id: 1001, Type: constant.ChannelTypeCustom, Key: "k1", Name: "alice-1", Status: common.ChannelStatusEnabled, Models: "gpt-4o-mini", Group: "default", CreatedTime: common.GetTimestamp(), Tag: common.GetPointer[string](model.DonationChannelTag(401))},
		{Id: 1002, Type: constant.ChannelTypeCustom, Key: "k2", Name: "alice-2", Status: common.ChannelStatusManuallyDisabled, Models: "gpt-4o-mini", Group: "default", CreatedTime: common.GetTimestamp(), Tag: common.GetPointer[string](model.DonationChannelTag(401))},
		{Id: 1003, Type: constant.ChannelTypeCustom, Key: "k3", Name: "bob-1", Status: common.ChannelStatusEnabled, Models: "gpt-4o-mini", Group: "default", CreatedTime: common.GetTimestamp(), Tag: common.GetPointer[string](model.DonationChannelTag(402))},
		{Id: 1004, Type: constant.ChannelTypeCustom, Key: "k4", Name: "bob-2", Status: common.ChannelStatusEnabled, Models: "gpt-4o-mini", Group: "default", CreatedTime: common.GetTimestamp(), Tag: common.GetPointer[string](model.DonationChannelTag(402))},
		{Id: 1005, Type: constant.ChannelTypeCustom, Key: "k5", Name: "carol-1", Status: common.ChannelStatusEnabled, Models: "gpt-4o-mini", Group: "default", CreatedTime: common.GetTimestamp(), Tag: common.GetPointer[string](model.DonationChannelTag(403))},
	}
	for _, channel := range channels {
		require.NoError(t, model.DB.Create(channel).Error)
	}

	items, err := GetDonationLeaderboardItems(402)
	require.NoError(t, err)
	require.Len(t, items, 3)

	assert.Equal(t, 402, items[0].UserID)
	assert.Equal(t, "bob", items[0].Username)
	assert.Equal(t, "", items[0].DisplayName)
	assert.Equal(t, 2, items[0].DonationCount)
	assert.Equal(t, 2, items[0].ActiveChannels)
	assert.True(t, items[0].IsMine)

	assert.Equal(t, 401, items[1].UserID)
	assert.Equal(t, "Alice", items[1].DisplayName)
	assert.Equal(t, 2, items[1].DonationCount)
	assert.Equal(t, 1, items[1].ActiveChannels)

	assert.Equal(t, 403, items[2].UserID)
	assert.Equal(t, 1, items[2].DonationCount)
	assert.Equal(t, 1, items[2].ActiveChannels)
}

func TestNormalizeDonationBaseURL(t *testing.T) {
	originalSetting := *operation_setting.GetDonationSetting()
	t.Cleanup(func() {
		*operation_setting.GetDonationSetting() = originalSetting
	})

	operation_setting.GetDonationSetting().BlockedDomainSuffixes = ".blocked.dev"

	normalized, err := normalizeDonationBaseURL("http://Srk.Replit.Dev/asqd/aa?foo=bar")
	require.NoError(t, err)
	assert.Equal(t, "https://srk.replit.dev/api", normalized)

	_, err = normalizeDonationBaseURL("https://demo.blocked.dev/path")
	require.EqualError(t, err, "该域名后缀不允许用于捐赠渠道")
}
