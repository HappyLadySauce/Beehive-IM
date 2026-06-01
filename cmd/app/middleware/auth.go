package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
)

func AuthMiddleware(s *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Intercept request headers
		// 请求头拦截
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			return
		}

		// Support both Bearer token and token without Bearer prefix
		// 支持 Bearer token 和未带 Bearer 的 token
		var tokenString string
		if len(tokenString) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		} else {
			tokenString = authHeader
		}

		claims, err := jwt.ParseToken(tokenString, s.Config.JWT.Secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		as := auth.NewAuthService(s.Cache)

		ctx, cancel := context.WithTimeout(c.Request.Context(), s.Config.Cache.DefaultExpiration)
		defer cancel()

		currentVersion, err := as.GetUserTokenVersion(ctx, claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "failed to get user token version",
			})
			return
		}

		if currentVersion == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token has been revoked",
			})
            return
		}

		// Token version mismatch indicates the token has been revoked or is outdated.
		// 令牌版本不匹配表示令牌已被撤销或过时。
		if claims.Version != currentVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token version mismatch, please login again",
			})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}
