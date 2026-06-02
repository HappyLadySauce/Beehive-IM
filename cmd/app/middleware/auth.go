package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
)

func AuthMiddleware(s *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || s.Config == nil || s.Config.JWT == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "auth service is not fully initialized",
			})
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			return
		}

		tokenString := authHeader
		if len(authHeader) > len("Bearer ") && strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
			tokenString = authHeader[len("Bearer "):]
		}

		claims, err := jwt.ParseToken(tokenString, s.Config.JWT.Secret, s.Config.JWT.Issuer)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		if claims.SessionID == "" || claims.UserID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid session",
			})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("sessionID", claims.SessionID)
		c.Set("deviceID", claims.DeviceID)
		c.Set("platform", claims.Platform)
		c.Next()
	}
}
