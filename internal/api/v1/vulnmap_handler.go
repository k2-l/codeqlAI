package v1

import (
    "net/http"

    "codeqlAI/internal/analyzer"
    "codeqlAI/internal/model"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

// VulnMapHandler 漏洞地图接口
type VulnMapHandler struct {
    db *gorm.DB
}

func NewVulnMapHandler(db *gorm.DB) *VulnMapHandler {
    return &VulnMapHandler{db: db}
}

func (h *VulnMapHandler) RegisterRoutes(rg gin.IRoutes) {
    rg.GET("/task/:id/vulnmap", h.GetVulnMap)
    rg.GET("/tasks", h.ListTasks)
}

// GetVulnMap GET /api/v1/task/:id/vulnmap
func (h *VulnMapHandler) GetVulnMap(c *gin.Context) {
    taskIDStr := c.Param("id")
    
    // 优化：校验 UUID 格式，防止无效 ID 导致的数据库错误或低效查询
    taskID, err := uuid.Parse(taskIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id format"})
        return
    }

    var task model.Task
    if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

    // 优化：确保空切片返回 [] 而非 null
    if flows == nil {
        flows = []analyzer.FindingFlow{}
    }

    c.JSON(http.StatusOK, gin.H{
        "task_id": taskIDStr,
        "total":   len(flows),
        "items":   flows,
    })
}

// ListTasks GET /api/v1/tasks?status=completed
func (h *VulnMapHandler) ListTasks(c *gin.Context) {
    statusFilter := c.Query("status")

    var tasks []model.Task
    // 注意：Preload("Project") 会额外查询关联表，如果不需要 Project 信息可移除以提升性能
    q := h.db.Preload("Project").Order("created_at DESC")
    
    if statusFilter != "" {
        q = q.Where("status = ?", statusFilter)
    }
    
    if err := q.Find(&tasks).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 优化：确保空切片返回 [] 而非 null
    if tasks == nil {
        tasks = []model.Task{}
    }

    c.JSON(http.StatusOK, gin.H{"total": len(tasks), "items": tasks})
}