package v1

import (
	"codeqlAI/internal/analyzer"
	"codeqlAI/internal/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// VulnMapHandler 漏洞地图接口
type VulnMapHandler struct {
	db *gorm.DB
}

func NewVulnMapHandler(db *gorm.DB) *VulnMapHandler {
	return &VulnMapHandler{db: db}
}

func (h *VulnMapHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/task/:id/vulnmap", h.GetVulnMap)
	rg.GET("/tasks", h.ListTasks) // 任务列表（仪表盘 + 漏洞地图选择器用）
}

// GetVulnMap GET /api/v1/task/:id/vulnmap
// 返回该任务 SARIF 里所有带 codeFlows 的 finding
func (h *VulnMapHandler) GetVulnMap(c *gin.Context) {
	taskID := c.Param("id")

	var task model.Task
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	if task.Status != model.TaskStatusCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task is not completed yet"})
		return
	}

	if task.SarifPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sarif file not available"})
		return
	}

	flows, err := analyzer.ParseCodeFlows(task.SarifPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"total":   len(flows),
		"items":   flows,
	})
}

// ListTasks GET /api/v1/tasks?status=completed
func (h *VulnMapHandler) ListTasks(c *gin.Context) {
	statusFilter := c.Query("status")

	var tasks []model.Task
	q := h.db.Preload("Project").Order("created_at DESC")
	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	}
	if err := q.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total": len(tasks), "items": tasks})
}
