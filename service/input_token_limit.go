package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type InputTokenLimitResult struct {
	Message      string
	WarningCount int
	UserDisabled bool
}

func HandleSingleInputTokenLimit(userID, tokenID, promptTokens int, userGroup string, isAdmin bool) (*InputTokenLimitResult, error) {
	if isAdmin || operation_setting.IsSingleInputTokensExemptGroup(userGroup) {
		return nil, nil
	}
	limit := operation_setting.GetSingleInputTokensLimit()
	if limit <= 0 || promptTokens <= 0 || promptTokens <= limit {
		return nil, nil
	}

	var result InputTokenLimitResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var token model.Token
		if err := tx.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
			return err
		}

		var user model.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}

		result.WarningCount = user.InputTokenLimitWarnCount + 1
		result.UserDisabled = result.WarningCount >= operation_setting.GetSingleInputTokensWarnThreshold()

		if err := tx.Model(&model.Token{}).
			Where("id = ? AND user_id = ?", tokenID, userID).
			Updates(map[string]interface{}{
				"status":        common.TokenStatusDisabled,
				"accessed_time": common.GetTimestamp(),
			}).Error; err != nil {
			return err
		}

		userUpdates := map[string]interface{}{
			"input_token_limit_warn_count": result.WarningCount,
		}
		if result.UserDisabled {
			userUpdates["status"] = common.UserStatusDisabled
			userUpdates["remark"] = buildInputTokenLimitRemark(user.Remark, promptTokens, limit, result.WarningCount)
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(userUpdates).Error; err != nil {
			return err
		}

		if err := model.UpdateTokenStatusCache(token.Key, common.TokenStatusDisabled); err != nil {
			common.SysLog("failed to update token status cache: " + err.Error())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := model.RefreshUserCache(userID); err != nil {
		common.SysLog("failed to refresh user cache: " + err.Error())
	}

	result.Message = fmt.Sprintf("您一次性输入了 %d tokens，超出平台限制，已将您的令牌禁用一次，请规范使用行为", promptTokens)
	if result.UserDisabled {
		result.Message += fmt.Sprintf("；由于累计警告已达 %d 次，账号已被禁用。", result.WarningCount)
		model.RecordLog(userID, model.LogTypeSystem, fmt.Sprintf("系统因单次输入 tokens 超限已禁用账号（第 %d 次警告，单次输入 %d tokens，阈值 %d）", result.WarningCount, promptTokens, limit))
	} else {
		model.RecordLog(userID, model.LogTypeSystem, fmt.Sprintf("系统因单次输入 tokens 超限已禁用令牌一次（第 %d 次警告，单次输入 %d tokens，阈值 %d）", result.WarningCount, promptTokens, limit))
	}
	return &result, nil
}

func buildInputTokenLimitRemark(existingRemark string, promptTokens, limit, warningCount int) string {
	const maxRemarkRunes = 255
	reason := fmt.Sprintf("[系统封禁] 单次输入 tokens 超限，累计警告 %d 次，最近一次 %d/%d tokens，已自动禁用账号，请管理员关注。", warningCount, promptTokens, limit)
	existingRemark = strings.TrimSpace(existingRemark)
	if existingRemark == "" {
		return truncateRunes(reason, maxRemarkRunes)
	}

	suffix := " 原备注：" + existingRemark
	remaining := maxRemarkRunes - runeLen(reason)
	if remaining <= 0 {
		return truncateRunes(reason, maxRemarkRunes)
	}
	return reason + truncateRunes(suffix, remaining)
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return string(runes[:1])
	}
	return string(runes[:limit-1]) + "…"
}

func runeLen(s string) int {
	return len([]rune(s))
}

var ErrInputTokenLimitTokenMissing = errors.New("token not found for input token limit handling")
