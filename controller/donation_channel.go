package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func CreateDonationChannel(c *gin.Context) {
	var req dto.CreateDonationChannelRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	userID := c.GetInt("id")
	username := c.GetString("username")
	resp, err := service.CreateDonationChannel(userID, username, req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

func GetDonationChannels(c *gin.Context) {
	items, err := service.GetDonationChannelItems(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}

func GetDonationLeaderboard(c *gin.Context) {
	items, err := service.GetDonationLeaderboardItems(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}
