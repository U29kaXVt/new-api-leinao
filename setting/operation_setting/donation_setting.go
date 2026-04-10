package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type DonationSetting struct {
	Enabled               bool   `json:"enabled"`
	TemplateChannelID     int    `json:"template_channel_id"`
	RewardQuota           int    `json:"reward_quota"`
	BlockedDomainSuffixes string `json:"blocked_domain_suffixes"`
	GuideText             string `json:"guide_text"`
	CopyButtonText        string `json:"copy_button_text"`
	CopyButtonContent     string `json:"copy_button_content"`
}

var donationSetting = DonationSetting{
	Enabled:               false,
	TemplateChannelID:     0,
	RewardQuota:           0,
	BlockedDomainSuffixes: "",
	GuideText:             "",
	CopyButtonText:        "",
	CopyButtonContent:     "",
}

func init() {
	config.GlobalConfig.Register("donation_setting", &donationSetting)
}

func GetDonationSetting() *DonationSetting {
	return &donationSetting
}
