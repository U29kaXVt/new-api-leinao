package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

type DonationChannelSubmission struct {
	Id          int    `json:"id"`
	UserID      int    `json:"user_id" gorm:"not null;uniqueIndex:idx_donation_user_fingerprint,priority:1"`
	Fingerprint string `json:"fingerprint" gorm:"type:varchar(64);not null;uniqueIndex:idx_donation_user_fingerprint,priority:2"`
	ChannelID   int    `json:"channel_id" gorm:"not null"`
	RewardQuota int    `json:"reward_quota" gorm:"not null;default:0"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

func DonationChannelTag(userID int) string {
	return fmt.Sprintf("donor_user_%d", userID)
}

func GetUserDonationChannels(userID int, onlyEnabled bool) ([]*Channel, error) {
	var channels []*Channel
	query := DB.Where("tag = ?", DonationChannelTag(userID)).Order("id desc").Omit("key")
	if onlyEnabled {
		query = query.Where("status = ?", common.ChannelStatusEnabled)
	}
	err := query.Find(&channels).Error
	return channels, err
}

func CountUserActiveDonationChannels(userID int) (int64, error) {
	var count int64
	err := DB.Model(&Channel{}).
		Where("tag = ?", DonationChannelTag(userID)).
		Where("status = ?", common.ChannelStatusEnabled).
		Count(&count).Error
	return count, err
}

func HasUserDonationFingerprint(userID int, fingerprint string) (bool, error) {
	var count int64
	err := DB.Model(&DonationChannelSubmission{}).
		Where("user_id = ? AND fingerprint = ?", userID, fingerprint).
		Count(&count).Error
	return count > 0, err
}
