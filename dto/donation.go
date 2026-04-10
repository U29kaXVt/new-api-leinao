package dto

type CreateDonationChannelRequest struct {
	BaseURL     string `json:"base_url"`
	ChannelName string `json:"channel_name"`
}

type DonationChannelItem struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	IsMine      bool   `json:"is_mine"`
	Status      int    `json:"status"`
	CreatedTime int64  `json:"created_time"`
}

type CreateDonationChannelResponse struct {
	Channel       DonationChannelItem `json:"channel"`
	RewardQuota   int                 `json:"reward_quota"`
	FetchedModels []string            `json:"fetched_models"`
}

type DonationLeaderboardItem struct {
	UserID         int    `json:"user_id"`
	Username       string `json:"username"`
	DisplayName    string `json:"display_name"`
	DonationCount  int    `json:"donation_count"`
	ActiveChannels int    `json:"active_channels"`
	IsMine         bool   `json:"is_mine"`
}
