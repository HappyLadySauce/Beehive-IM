package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/session"
)

func AuthMiddleware(s *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || s.Config == nil || s.Config.JWT == nil || s.Config.Cache == nil || s.Cache == nil || s.Config.Cache.CommandTimeout <= 0 {
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

		sessionClaims, err := session.ParseSessionID(claims.SessionID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid session",
			})
			return
		}

		as := auth.NewAuthService(s)

		ctx, cancel := context.WithTimeout(c.Request.Context(), s.Config.Cache.CommandTimeout)
		defer cancel()

		active, err := as.SessionIsActive(ctx, claims.SessionID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "failed to verify session",
			})
			return
		}
		if !active {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "session expired or revoked",
			})
			return
		}

		c.Set("userID", sessionClaims.UserID)
		c.Set("username", sessionClaims.Username)
		c.Set("sessionID", claims.SessionID)
		c.Set("deviceID", sessionClaims.DeviceID)
		c.Set("platform", sessionClaims.Platform)
		c.Next()
	}
}
