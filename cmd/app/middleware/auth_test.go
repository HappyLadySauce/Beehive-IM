package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	goredis "github.com/redis/go-redis/v9"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/config"
	"github.com/HappyLadySauce/Beehive-IM/pkg/options"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/session"
)

func TestAuthMiddlewareAcceptsLowercaseBearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx, _ := newAuthMiddlewareTestContext(t)
	sessionID := mustTestSessionID(t)
	token := newAuthTestToken(t, svcCtx, sessionID)

	router := gin.New()
	router.GET("/protected", AuthMiddleware(svcCtx), func(c *gin.Context) {
		c.String(http.StatusOK, "%s:%s:%s:%s:%s", c.GetString("userID"), c.GetString("username"), c.GetString("sessionID"), c.GetString("deviceID"), c.GetString("platform"))
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q, want 200", resp.Code, resp.Body.String())
	}
	if got, want := resp.Body.String(), "1:alice:"+sessionID+":device-1:web"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestAuthMiddlewareDoesNotRequireRedisSessionForAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx, _ := newAuthMiddlewareTestContext(t)
	sessionID := mustTestSessionID(t)
	token := newAuthTestToken(t, svcCtx, sessionID)

	router := gin.New()
	router.GET("/protected", AuthMiddleware(svcCtx), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q, want 200", resp.Code, resp.Body.String())
	}
}

func TestAuthMiddlewareRejectsIncompleteServiceContextWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", AuthMiddleware(&svc.ServiceContext{}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %q, want 500", resp.Code, resp.Body.String())
	}
}

func newAuthMiddlewareTestContext(t *testing.T) (*svc.ServiceContext, *miniredis.Miniredis) {
	t.Helper()

	redisServer := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	return &svc.ServiceContext{
		Config: &config.Config{
			Cache: &options.RedisOptions{
				CommandTimeout: 100 * time.Millisecond,
			},
			JWT: &options.JWTOptions{
				Issuer:    "Beehive-IM",
				Secret:    "12345678901234567890123456789012",
				AccessTTL: time.Hour,
			},
		},
		Cache: redisClient,
	}, redisServer
}

func mustTestSessionID(t *testing.T) string {
	t.Helper()
	id, err := session.GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID() error = %v", err)
	}
	return id
}

func newAuthTestToken(t *testing.T, svcCtx *svc.ServiceContext, sessionID string) string {
	t.Helper()

	token, err := jwt.GenerateToken(
		jwt.TokenClaims{
			SessionID: sessionID,
			UserID:    "1",
			Username:  "alice",
			DeviceID:  "device-1",
			Platform:  "web",
		},
		svcCtx.Config.JWT.Issuer,
		svcCtx.Config.JWT.Secret,
		jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
	)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	return token
}
