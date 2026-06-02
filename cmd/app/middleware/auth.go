package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
)

func AuthMiddleware(s *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || s.Config == nil || s.Config.JWT == nil || s.Config.Cache == nil || s.Cache == nil || s.Config.Cache.CommandTimeout <= 0 {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "auth service is not fully initialized",
			})
			return
		}

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

		as := auth.NewAuthService(s.Cache)

		ctx, cancel := context.WithTimeout(c.Request.Context(), s.Config.Cache.CommandTimeout)
		defer cancel()

		currentVersion, err := as.GetUserTokenVersion(ctx, claims.UserID)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "token has been revoked",
				})
				return
			}

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
