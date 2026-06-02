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
	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
	"github.com/HappyLadySauce/Beehive-IM/pkg/config"
	"github.com/HappyLadySauce/Beehive-IM/pkg/options"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
)

func TestAuthMiddlewareAcceptsLowercaseBearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx, redisServer := newAuthMiddlewareTestContext(t)
	redisServer.Set(cache.SessionPrefix+"session-1", "version-1")
	token := newAuthTestToken(t, svcCtx)

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
	if got, want := resp.Body.String(), "1:alice:session-1:device-1:web"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestAuthMiddlewareTreatsMissingSessionAsRevoked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx, _ := newAuthMiddlewareTestContext(t)
	token := newAuthTestToken(t, svcCtx)

	router := gin.New()
	router.GET("/protected", AuthMiddleware(svcCtx), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %q, want 401", resp.Code, resp.Body.String())
	}
	if got, want := resp.Body.String(), `{"error":"token has been revoked"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestAuthMiddlewareRejectsSessionVersionMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx, redisServer := newAuthMiddlewareTestContext(t)
	redisServer.Set(cache.SessionPrefix+"session-1", "version-2")
	token := newAuthTestToken(t, svcCtx)

	router := gin.New()
	router.GET("/protected", AuthMiddleware(svcCtx), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %q, want 401", resp.Code, resp.Body.String())
	}
	if got, want := resp.Body.String(), `{"error":"token version mismatch, please login again"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestAuthMiddlewareRejectsTokenAfterCurrentSessionDeleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx, redisServer := newAuthMiddlewareTestContext(t)
	redisServer.Set(cache.SessionPrefix+"session-1", "version-1")
	token := newAuthTestToken(t, svcCtx)
	redisServer.Del(cache.SessionPrefix + "session-1")

	router := gin.New()
	router.GET("/protected", AuthMiddleware(svcCtx), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %q, want 401", resp.Code, resp.Body.String())
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

func newAuthTestToken(t *testing.T, svcCtx *svc.ServiceContext) string {
	t.Helper()

	token, err := jwt.GenerateToken(
		"1",
		"alice",
		"session-1",
		"device-1",
		"web",
		"version-1",
		svcCtx.Config.JWT.Issuer,
		svcCtx.Config.JWT.Secret,
		jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
	)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	return token
}
