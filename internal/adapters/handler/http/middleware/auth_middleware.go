package middleware

import (
	"net/http"
	"strings"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/gin-gonic/gin"
)

const (
	authorizationHeader = "Authorization"
	authorizationType   = "Bearer"
	ContextUserIDKey    = "userID"
)

func AuthMiddleware(tokenService *services.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorizationHeader)
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		fields := strings.Fields(authHeader)
		if len(fields) < 2 || fields[0] != authorizationType {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := fields[1]

		userID, err := tokenService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, userID)

		c.Next()
	}
}

func GetUserID(c *gin.Context) (string, bool) {
	id, exists := c.Get(ContextUserIDKey)
	if !exists {
		return "", false
	}
	idStr, ok := id.(string)
	return idStr, ok
}
