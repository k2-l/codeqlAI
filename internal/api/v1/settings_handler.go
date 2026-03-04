package v1

import (
	"codeqlAI/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SettingsHandler 系统设置接口
type SettingsHandler struct {
	settingsService *service.AISettingsService
}

// NewSettingsHandler 初始化
func NewSettingsHandler(settingsService *service.AISettingsService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService}
}

// RegisterSettingsRoutes 注册设置路由
func (h *SettingsHandler) RegisterSettingsRoutes(rg gin.IRoutes) {
	rg.GET("/settings/ai", h.GetAISettings)
	rg.PUT("/settings/ai", h.UpdateAISettings)
}

// GetAISettings GET /api/v1/settings/ai
func (h *SettingsHandler) GetAISettings(c *gin.Context) {
	resp, err := h.settingsService.GetAISettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateAISettings PUT /api/v1/settings/ai
func (h *SettingsHandler) UpdateAISettings(c *gin.Context) {
	var req service.UpdateAISettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.settingsService.UpdateAISettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "AI settings updated successfully. Restart the server to apply changes.",
	})
}
