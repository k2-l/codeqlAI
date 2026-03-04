package v1

import (
	"codeqlAI/configs"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// ===== Auth Handler =====

type AuthHandler struct {
	cfg configs.AuthConfig
	rdb *redis.Client
}

func NewAuthHandler(cfg configs.AuthConfig, rdb *redis.Client) *AuthHandler {
	return &AuthHandler{cfg: cfg, rdb: rdb}
}

func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/auth/captcha", h.GetCaptcha)
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/logout", h.Logout)
}

// ---------- 验证码 ----------

const captchaPrefix = "captcha:"
const captchaTTL    = 5 * time.Minute

// GetCaptcha GET /api/v1/auth/captcha
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	// 生成 ID（8位hex）
	idBytes := make([]byte, 4)
	rand.Read(idBytes)
	captchaID := fmt.Sprintf("%x", idBytes)

	// 4 位随机数字
	code := ""
	for i := 0; i < 4; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		code += n.String()
	}

	// 存 Redis，5 分钟有效
	if err := h.rdb.Set(c.Request.Context(), captchaPrefix+captchaID, code, captchaTTL).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate captcha"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"captcha_id":   captchaID,
		"captcha_code": code,
	})
}

// verifyCaptcha 校验并立即删除（一次性）
func (h *AuthHandler) verifyCaptcha(ctx context.Context, id, input string) bool {
	stored, err := h.rdb.GetDel(ctx, captchaPrefix+id).Result()
	if err != nil {
		return false
	}
	return strings.TrimSpace(stored) == strings.TrimSpace(input)
}

// ---------- 登录 ----------

type loginRequest struct {
	Username    string `json:"username"     binding:"required"`
	Password    string `json:"password"     binding:"required"`
	CaptchaID   string `json:"captcha_id"   binding:"required"`
	CaptchaCode string `json:"captcha_code" binding:"required"`
}

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所有字段均为必填"})
		return
	}

	// 先消耗验证码（不管账密是否正确，都消耗，防止暴力枚举）
	if !h.verifyCaptcha(c.Request.Context(), req.CaptchaID, req.CaptchaCode) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误或已过期"})
		return
	}

	if req.Username != h.cfg.Username || req.Password != h.cfg.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, expiresAt, err := generateToken(h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": expiresAt.Unix(),
		"username":   h.cfg.Username,
	})
}

// Logout POST /api/v1/auth/logout（无状态，前端清 Token 即可）
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

// ===== JWT 工具函数 =====

type jwtClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateToken(cfg configs.AuthConfig) (string, time.Time, error) {
	ttl := cfg.TokenTTLH
	if ttl <= 0 {
		ttl = 24
	}
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Hour)

	claims := jwtClaims{
		Username: cfg.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "codeql-ai-scanner",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(cfg.JWTSecret))
	return signed, expiresAt, err
}

// ===== JWT 鉴权中间件 =====

func JWTMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		// 格式：Bearer <token>
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		tokenStr := parts[1]
		claims := &jwtClaims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// 把用户名注入 context，后续 handler 可以取用
		c.Set("username", claims.Username)
		c.Next()
	}
}
