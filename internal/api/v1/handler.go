package v1

import (
	"codeqlAI/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler 持有业务逻辑层依赖
type Handler struct {
	scanService *service.ScanService
}

// NewHandler 初始化 Handler
func NewHandler(scanService *service.ScanService) *Handler {
	return &Handler{scanService: scanService}
}

// RegisterRoutes 注册所有 v1 路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/scan", h.SubmitScan)
	rg.GET("/task/:id", h.GetTask)
	rg.GET("/task/:id/results", h.GetFindings)
	rg.GET("/task/name/:name", h.GetTaskByName)
	rg.DELETE("/task/:id", h.DeleteTask)        // 彻底删除任务
	rg.POST("/finding/:id/audit", h.TriggerAIAudit)
}

// SubmitScan POST /api/v1/scan
func (h *Handler) SubmitScan(c *gin.Context) {
	var req service.SubmitScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.scanService.SubmitScan(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "scan task submitted",
		"task_id":      task.ID,
		"display_name": task.DisplayName,
		"status":       task.Status,
	})
}

// GetTask GET /api/v1/task/:id
// 查询任务状态和进度
func (h *Handler) GetTask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
		return
	}

	task, err := h.scanService.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetFindings GET /api/v1/task/:id/results
// 查询任务下所有漏洞（含 AI 审计结论）
func (h *Handler) GetFindings(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
		return
	}

	findings, err := h.scanService.GetFindings(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"total":   len(findings),
		"items":   findings,
	})
}

// DeleteTask DELETE /api/v1/task/:id
// 彻底删除任务：取消队列中的待执行任务 + 清理数据库所有关联记录
func (h *Handler) DeleteTask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
		return
	}

	if err := h.scanService.DeleteTask(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "task deleted successfully",
		"task_id": taskID,
	})
}

// GetTaskByName GET /api/v1/task/name/:name
// 按自定义任务名查询任务（支持模糊匹配）
func (h *Handler) GetTaskByName(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task name is required"})
		return
	}

	task, err := h.scanService.GetTaskByName(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}
// 手动触发单条 Finding 的 AI 审计
func (h *Handler) TriggerAIAudit(c *gin.Context) {
	findingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid finding_id"})
		return
	}

	if err := h.scanService.TriggerAIAudit(findingID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "AI audit task enqueued",
		"finding_id": findingID,
	})
}