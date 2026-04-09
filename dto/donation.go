package dto

type CreateDonationChannelRequest struct {
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
}

type DonationChannelItem struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	BaseURL     string   `json:"base_url"`
	Models      []string `json:"models"`
	Status      int      `json:"status"`
	CreatedTime int64    `json:"created_time"`
}

type CreateDonationChannelResponse struct {
	Channel       DonationChannelItem `json:"channel"`
	RewardQuota   int                 `json:"reward_quota"`
	FetchedModels []string            `json:"fetched_models"`
}
