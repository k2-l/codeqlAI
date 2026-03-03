package v1

import (
	"codeqlAI/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RuleHandler 自定义 QL 规则接口
type RuleHandler struct {
	ruleService *service.CustomRuleService
}

func NewRuleHandler(svc *service.CustomRuleService) *RuleHandler {
	return &RuleHandler{ruleService: svc}
}

func (h *RuleHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/rules", h.CreateRule)
	rg.GET("/rules", h.ListRules)
	rg.GET("/rules/:id", h.GetRule)
	rg.PUT("/rules/:id", h.UpdateRule)
	rg.DELETE("/rules/:id", h.DeleteRule)
}

// CreateRule POST /api/v1/rules
func (h *RuleHandler) CreateRule(c *gin.Context) {
	var req service.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := h.ruleService.CreateRule(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// ListRules GET /api/v1/rules?language=java
func (h *RuleHandler) ListRules(c *gin.Context) {
	language := c.Query("language")
	rules, err := h.ruleService.ListRules(language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": len(rules), "items": rules})
}

// GetRule GET /api/v1/rules/:id
func (h *RuleHandler) GetRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	rule, err := h.ruleService.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// UpdateRule PUT /api/v1/rules/:id
func (h *RuleHandler) UpdateRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	var req service.UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := h.ruleService.UpdateRule(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// DeleteRule DELETE /api/v1/rules/:id
func (h *RuleHandler) DeleteRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	if err := h.ruleService.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "rule deleted", "rule_id": id})
}
