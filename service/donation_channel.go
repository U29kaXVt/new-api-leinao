package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var (
	ErrDonationChannelDisabled         = errors.New("捐赠渠道功能未开启")
	ErrDonationTemplateNotConfigured   = errors.New("管理员尚未配置捐赠模板渠道")
	ErrDonationTemplateNotFound        = errors.New("捐赠模板渠道不存在")
	ErrDonationChannelAlreadySubmitted = errors.New("该捐赠渠道已提交过")
	ErrDonationChannelUnavailable      = errors.New("你捐赠的渠道已经都没了/都不可用了，请重新捐赠")
)

var donationChannelLocks sync.Map

func getDonationChannelLock(userID int) *sync.Mutex {
	if lock, ok := donationChannelLocks.Load(userID); ok {
		return lock.(*sync.Mutex)
	}
	lock := &sync.Mutex{}
	actual, _ := donationChannelLocks.LoadOrStore(userID, lock)
	return actual.(*sync.Mutex)
}

func normalizeDonationBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("base_url 不能为空")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errors.New("base_url 格式不正确")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("base_url 只支持 http 或 https")
	}
	if parsed.Host == "" {
		return "", errors.New("base_url 缺少主机名")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.RawQuery != "" {
		values, err := url.ParseQuery(parsed.RawQuery)
		if err == nil {
			parsed.RawQuery = values.Encode()
		}
	}
	if parsed.Path == "/" {
		parsed.Path = ""
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	return parsed.String(), nil
}

func normalizeDonationKey(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("key 不能为空")
	}
	return trimmed, nil
}

func buildDonationFingerprint(userID int, normalizedBaseURL string, normalizedKey string) string {
	payload := fmt.Sprintf("%d\n%s\n%s", userID, normalizedBaseURL, normalizedKey)
	return hex.EncodeToString(common.Sha256Raw([]byte(payload)))
}

func buildDonationChannelName(username string) string {
	safeUsername := strings.TrimSpace(username)
	if safeUsername == "" {
		safeUsername = "user"
	}
	suffixBytes := common.Sha256Raw([]byte(fmt.Sprintf("%s:%d", safeUsername, common.GetTimestamp())))
	return fmt.Sprintf("%s#%s", safeUsername, hex.EncodeToString(suffixBytes[:4]))
}

func cloneDonationTemplateChannel(template *model.Channel, username string, baseURL string, key string, userID int) *model.Channel {
	clone := *template
	clone.Id = 0
	clone.BaseURL = common.GetPointer[string](baseURL)
	clone.Key = key
	clone.Name = buildDonationChannelName(username)
	clone.Status = common.ChannelStatusEnabled
	clone.SetTag(model.DonationChannelTag(userID))
	clone.CreatedTime = common.GetTimestamp()
	clone.TestTime = 0
	clone.ResponseTime = 0
	clone.Balance = 0
	clone.BalanceUpdatedTime = 0
	clone.UsedQuota = 0
	clone.OtherInfo = ""
	clone.ChannelInfo = model.ChannelInfo{}
	return &clone
}

func donationChannelToItem(channel *model.Channel) dto.DonationChannelItem {
	return dto.DonationChannelItem{
		Id:          channel.Id,
		Name:        channel.Name,
		BaseURL:     channel.GetBaseURL(),
		Models:      channel.GetModels(),
		Status:      channel.Status,
		CreatedTime: channel.CreatedTime,
	}
}

func persistDonationReward(userID int, channelID int, fingerprint string, rewardQuota int) error {
	submission := model.DonationChannelSubmission{
		UserID:      userID,
		Fingerprint: fingerprint,
		ChannelID:   channelID,
		RewardQuota: rewardQuota,
		CreatedTime: common.GetTimestamp(),
	}

	txErr := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&submission).Error; err != nil {
			return err
		}
		if rewardQuota <= 0 {
			return nil
		}
		return tx.Model(&model.User{}).
			Where("id = ?", userID).
			Update("quota", gorm.Expr("quota + ?", rewardQuota)).
			Error
	})
	if txErr != nil {
		return txErr
	}

	if rewardQuota > 0 {
		if err := model.RefreshUserCache(userID); err != nil {
			common.SysLog("failed to refresh donation reward user cache: " + err.Error())
		}
	}
	return nil
}

func CreateDonationChannel(userID int, username string, req dto.CreateDonationChannelRequest) (*dto.CreateDonationChannelResponse, error) {
	lock := getDonationChannelLock(userID)
	lock.Lock()
	defer lock.Unlock()

	setting := operation_setting.GetDonationSetting()
	if !setting.Enabled {
		return nil, ErrDonationChannelDisabled
	}
	if setting.TemplateChannelID <= 0 {
		return nil, ErrDonationTemplateNotConfigured
	}

	normalizedBaseURL, err := normalizeDonationBaseURL(req.BaseURL)
	if err != nil {
		return nil, err
	}
	normalizedKey, err := normalizeDonationKey(req.Key)
	if err != nil {
		return nil, err
	}
	fingerprint := buildDonationFingerprint(userID, normalizedBaseURL, normalizedKey)
	exists, err := model.HasUserDonationFingerprint(userID, fingerprint)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDonationChannelAlreadySubmitted
	}

	template, err := model.GetChannelById(setting.TemplateChannelID, true)
	if err != nil {
		return nil, ErrDonationTemplateNotFound
	}
	if template.ChannelInfo.IsMultiKey {
		return nil, errors.New("捐赠模板渠道不能使用多密钥模式")
	}

	channel := cloneDonationTemplateChannel(template, username, normalizedBaseURL, normalizedKey, userID)
	if err := channel.Insert(); err != nil {
		return nil, err
	}

	if err := model.UpdateAbilityStatus(channel.Id, false); err != nil {
		_ = channel.Delete()
		return nil, err
	}

	fetchedModels, err := FetchChannelUpstreamModelIDs(channel)
	if err != nil {
		_ = channel.Delete()
		return nil, fmt.Errorf("校验捐赠渠道失败：%w", err)
	}

	if err := persistDonationReward(userID, channel.Id, fingerprint, setting.RewardQuota); err != nil {
		_ = channel.Delete()
		recheckExists, recheckErr := model.HasUserDonationFingerprint(userID, fingerprint)
		if recheckErr == nil && recheckExists {
			return nil, ErrDonationChannelAlreadySubmitted
		}
		return nil, err
	}

	model.RecordLog(userID, model.LogTypeSystem, fmt.Sprintf("捐赠渠道验证成功，渠道 ID %d，奖励额度 %s", channel.Id, logger.LogQuota(setting.RewardQuota)))
	if common.MemoryCacheEnabled {
		model.InitChannelCache()
	}
	ResetProxyClientCache()

	return &dto.CreateDonationChannelResponse{
		Channel:       donationChannelToItem(channel),
		RewardQuota:   setting.RewardQuota,
		FetchedModels: fetchedModels,
	}, nil
}

func GetUserDonationChannelItems(userID int) ([]dto.DonationChannelItem, error) {
	channels, err := model.GetUserDonationChannels(userID, true)
	if err != nil {
		return nil, err
	}
	items := make([]dto.DonationChannelItem, 0, len(channels))
	for _, channel := range channels {
		items = append(items, donationChannelToItem(channel))
	}
	return items, nil
}

func EnsureUserHasDonationChannel(userID int, isAdmin bool) error {
	if isAdmin {
		return nil
	}
	if !operation_setting.GetDonationSetting().Enabled {
		return nil
	}
	count, err := model.CountUserActiveDonationChannels(userID)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrDonationChannelUnavailable
	}
	return nil
}
