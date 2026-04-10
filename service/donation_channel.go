package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var (
	ErrDonationChannelDisabled         = errors.New("捐赠渠道功能未开启")
	ErrDonationTemplateNotConfigured   = errors.New("管理员尚未配置捐赠模板渠道")
	ErrDonationTemplateNotFound        = errors.New("捐赠模板渠道不存在")
	ErrDonationChannelAlreadySubmitted = errors.New("该捐赠渠道已提交过")
	ErrDonationChannelUnavailable      = errors.New("你捐赠的渠道已经都没了/都不可用了，请重新捐赠")
)

var donationChannelLocks sync.Map
var donationDomainLocks sync.Map

func getDonationChannelLock(userID int) *sync.Mutex {
	if lock, ok := donationChannelLocks.Load(userID); ok {
		return lock.(*sync.Mutex)
	}
	lock := &sync.Mutex{}
	actual, _ := donationChannelLocks.LoadOrStore(userID, lock)
	return actual.(*sync.Mutex)
}

func getDonationDomainLock(domainKey string) *sync.Mutex {
	if lock, ok := donationDomainLocks.Load(domainKey); ok {
		return lock.(*sync.Mutex)
	}
	lock := &sync.Mutex{}
	actual, _ := donationDomainLocks.LoadOrStore(domainKey, lock)
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

func buildDonationDomainKey(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(baseURL))
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	if host == "" {
		return strings.ToLower(strings.TrimSpace(baseURL))
	}
	return host + "/api"
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

func resolveDonationChannelName(username string, customName string) string {
	trimmed := strings.TrimSpace(customName)
	if trimmed != "" {
		return trimmed
	}
	return buildDonationChannelName(username)
}

func cloneDonationTemplateChannel(template *model.Channel, username string, customName string, baseURL string, userID int) *model.Channel {
	clone := *template
	clone.Id = 0
	clone.BaseURL = common.GetPointer[string](baseURL)
	clone.Name = resolveDonationChannelName(username, customName)
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

func persistDonationReward(userID int, rewardQuota int) error {
	if rewardQuota <= 0 {
		return nil
	}
	if err := model.IncreaseUserQuota(userID, rewardQuota, true); err != nil {
		return err
	}
	if err := model.RefreshUserCache(userID); err != nil {
		common.SysLog("failed to refresh donation reward user cache: " + err.Error())
	}
	return nil
}

func hasEnabledDonationChannelByDomainKey(domainKey string, templateChannelID int) (bool, error) {
	channels, err := model.GetAllDonationChannels()
	if err != nil {
		return false, err
	}
	for _, channel := range channels {
		if channel == nil || channel.Id == templateChannelID {
			continue
		}
		if buildDonationDomainKey(channel.GetBaseURL()) != domainKey {
			continue
		}
		if channel.Status == common.ChannelStatusEnabled {
			return true, nil
		}
	}
	return false, nil
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
	domainKey := buildDonationDomainKey(normalizedBaseURL)
	domainLock := getDonationDomainLock(domainKey)
	domainLock.Lock()
	defer domainLock.Unlock()

	template, err := model.GetChannelById(setting.TemplateChannelID, true)
	if err != nil {
		return nil, ErrDonationTemplateNotFound
	}
	if template.ChannelInfo.IsMultiKey {
		return nil, errors.New("捐赠模板渠道不能使用多密钥模式")
	}
	hasEnabledChannel, err := hasEnabledDonationChannelByDomainKey(domainKey, setting.TemplateChannelID)
	if err != nil {
		return nil, err
	}
	if hasEnabledChannel {
		return nil, ErrDonationChannelAlreadySubmitted
	}

	channel := cloneDonationTemplateChannel(template, username, req.ChannelName, normalizedBaseURL, userID)
	if err := channel.Insert(); err != nil {
		return nil, err
	}

	fetchedModels, err := validateDonationChannel(channel)
	if err != nil {
		_ = channel.Delete()
		return nil, fmt.Errorf("校验捐赠渠道失败：%w", err)
	}

	if err := persistDonationReward(userID, setting.RewardQuota); err != nil {
		_ = channel.Delete()
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

func GetDonationLeaderboardItems(userID int) ([]dto.DonationLeaderboardItem, error) {
	channels, err := model.GetAllDonationChannels()
	if err != nil {
		return nil, err
	}

	type aggregate struct {
		DonationCount  int
		ActiveChannels int
	}
	userStats := make(map[int]*aggregate)
	userIDs := make([]int, 0)
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		ownerID, ok := model.ParseDonationChannelUserID(channel.GetTag())
		if !ok {
			continue
		}
		stats, exists := userStats[ownerID]
		if !exists {
			stats = &aggregate{}
			userStats[ownerID] = stats
			userIDs = append(userIDs, ownerID)
		}
		stats.DonationCount++
		if channel.Status == common.ChannelStatusEnabled {
			stats.ActiveChannels++
		}
	}
	if len(userStats) == 0 {
		return []dto.DonationLeaderboardItem{}, nil
	}

	var users []*model.User
	if err := model.DB.Select("id", "username", "display_name").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	userMap := make(map[int]*model.User, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		userMap[user.Id] = user
	}

	items := make([]dto.DonationLeaderboardItem, 0, len(userStats))
	for _, ownerID := range userIDs {
		stats := userStats[ownerID]
		user := userMap[ownerID]
		if user == nil {
			continue
		}
		items = append(items, dto.DonationLeaderboardItem{
			UserID:         ownerID,
			Username:       user.Username,
			DisplayName:    user.DisplayName,
			DonationCount:  stats.DonationCount,
			ActiveChannels: stats.ActiveChannels,
			IsMine:         ownerID == userID,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].DonationCount != items[j].DonationCount {
			return items[i].DonationCount > items[j].DonationCount
		}
		if items[i].ActiveChannels != items[j].ActiveChannels {
			return items[i].ActiveChannels > items[j].ActiveChannels
		}
		return items[i].UserID < items[j].UserID
	})
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
