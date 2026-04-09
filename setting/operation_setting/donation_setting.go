package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type DonationSetting struct {
	Enabled           bool `json:"enabled"`
	TemplateChannelID int  `json:"template_channel_id"`
	RewardQuota       int  `json:"reward_quota"`
}

var donationSetting = DonationSetting{
	Enabled:           false,
	TemplateChannelID: 0,
	RewardQuota:       0,
}

func init() {
	config.GlobalConfig.Register("donation_setting", &donationSetting)
}

func GetDonationSetting() *DonationSetting {
	return &donationSetting
}
