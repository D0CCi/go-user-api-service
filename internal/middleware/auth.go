package middleware

import (
	"net/http"
	"pr-reviewer-service/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	AdminTokenHeader = "X-Admin-Token"
	UserTokenHeader  = "X-User-Token"
)

// Простая проверка токенов (для тестового задания)
// В реальном проекте здесь была бы проверка JWT или других токенов
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminToken := c.GetHeader(AdminTokenHeader)
		userToken := c.GetHeader(UserTokenHeader)

		// Для тестового задания принимаем любой непустой токен
		if adminToken == "" && userToken == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorNotFound,
					Message: "authentication required",
				},
			})
			c.Abort()
			return
		}

		c.Set("isAdmin", adminToken != "")
		c.Next()
	}
}

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminToken := c.GetHeader(AdminTokenHeader)
		if adminToken == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorNotFound,
					Message: "admin token required",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}


