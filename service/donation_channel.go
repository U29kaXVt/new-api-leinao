package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
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

	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return "", errors.New("base_url 缺少主机名")
	}
	if isBlockedDonationHostname(hostname) {
		return "", errors.New("该域名后缀不允许用于捐赠渠道")
	}

	host := hostname
	if port := strings.TrimSpace(parsed.Port()); port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	return buildDonationBaseURL("https", host), nil
}

func parseDonationBlockedDomainSuffixes() []string {
	raw := strings.TrimSpace(operation_setting.GetDonationSetting().BlockedDomainSuffixes)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	suffixes := make([]string, 0, len(parts))
	for _, part := range parts {
		suffix := strings.ToLower(strings.TrimSpace(part))
		if suffix == "" {
			continue
		}
		suffixes = append(suffixes, suffix)
	}
	return suffixes
}

func hostnameMatchesBlockedSuffix(hostname string, suffix string) bool {
	if hostname == "" || suffix == "" {
		return false
	}
	if strings.HasPrefix(suffix, ".") {
		return strings.HasSuffix(hostname, suffix)
	}
	return hostname == suffix || strings.HasSuffix(hostname, "."+suffix)
}

func isBlockedDonationHostname(hostname string) bool {
	for _, suffix := range parseDonationBlockedDomainSuffixes() {
		if hostnameMatchesBlockedSuffix(hostname, suffix) {
			return true
		}
	}
	return false
}

func buildDonationFingerprint(normalizedBaseURL string) string {
	parsed, err := url.Parse(normalizedBaseURL)
	if err != nil {
		return hex.EncodeToString(common.Sha256Raw([]byte(normalizedBaseURL)))
	}
	payload := strings.ToLower(parsed.Host) + "/api"
	return hex.EncodeToString(common.Sha256Raw([]byte(payload)))
}

func buildDonationBaseURL(scheme, host string) string {
	return (&url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   "/api",
	}).String()
}

func buildDonationHTTPFallbackURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", errors.New("base_url 缺少主机名")
	}
	return buildDonationBaseURL("http", parsed.Host), nil
}

func validateDonationChannel(channel *model.Channel) ([]string, error) {
	models, err := FetchChannelUpstreamModelIDs(channel)
	if err == nil {
		return models, nil
	}
	if !strings.HasPrefix(channel.GetBaseURL(), "https://") {
		return nil, err
	}

	fallbackBaseURL, fallbackErr := buildDonationHTTPFallbackURL(channel.GetBaseURL())
	if fallbackErr != nil {
		return nil, err
	}
	channel.BaseURL = common.GetPointer[string](fallbackBaseURL)
	if updateErr := model.DB.Model(channel).Update("base_url", fallbackBaseURL).Error; updateErr != nil {
		return nil, err
	}
	models, fallbackErr = FetchChannelUpstreamModelIDs(channel)
	if fallbackErr == nil {
		return models, nil
	}
	return nil, fallbackErr
}

func buildDonationChannelName(username string) string {
	safeUsername := strings.TrimSpace(username)
	if safeUsername == "" {
		safeUsername = "user"
	}
	suffixBytes := common.Sha256Raw([]byte(fmt.Sprintf("%s:%d", safeUsername, common.GetTimestamp())))
	return fmt.Sprintf("%s#%s", safeUsername, hex.EncodeToString(suffixBytes[:4]))
}

func cloneDonationTemplateChannel(template *model.Channel, username string, baseURL string, userID int) *model.Channel {
	clone := *template
	clone.Id = 0
	clone.BaseURL = common.GetPointer[string](baseURL)
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
		IsMine:      false,
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

	template, err := model.GetChannelById(setting.TemplateChannelID, true)
	if err != nil {
		return nil, ErrDonationTemplateNotFound
	}
	if template.ChannelInfo.IsMultiKey {
		return nil, errors.New("捐赠模板渠道不能使用多密钥模式")
	}
	fingerprint := buildDonationFingerprint(normalizedBaseURL)
	exists, err := model.HasDonationFingerprint(fingerprint)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDonationChannelAlreadySubmitted
	}

	channel := cloneDonationTemplateChannel(template, username, normalizedBaseURL, userID)
	if err := channel.Insert(); err != nil {
		return nil, err
	}

	if err := model.UpdateAbilityStatus(channel.Id, false); err != nil {
		_ = channel.Delete()
		return nil, err
	}

	fetchedModels, err := validateDonationChannel(channel)
	if err != nil {
		_ = channel.Delete()
		return nil, fmt.Errorf("校验捐赠渠道失败：%w", err)
	}

	if err := persistDonationReward(userID, channel.Id, fingerprint, setting.RewardQuota); err != nil {
		_ = channel.Delete()
		recheckExists, recheckErr := model.HasDonationFingerprint(fingerprint)
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

func GetDonationChannelItems(userID int) ([]dto.DonationChannelItem, error) {
	channels, err := model.GetActiveDonationChannels()
	if err != nil {
		return nil, err
	}
	items := make([]dto.DonationChannelItem, 0, len(channels))
	userTag := model.DonationChannelTag(userID)
	for _, channel := range channels {
		item := donationChannelToItem(channel)
		item.IsMine = channel.GetTag() == userTag
		items = append(items, item)
	}
	return items, nil
}

func EnsureUserHasDonationChannel(userID int, userGroup string, isAdmin bool) error {
	_ = userGroup
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
