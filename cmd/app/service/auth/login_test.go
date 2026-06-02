package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
	"github.com/HappyLadySauce/Beehive-IM/pkg/config"
	"github.com/HappyLadySauce/Beehive-IM/pkg/options"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/passwd"
)

func TestLoginWithCorrectPasswordCreatesSession(t *testing.T) {
	service, redisServer, mock := newLoginTestService(t)
	hash, err := passwd.HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE username = .* LIMIT .*`).
		WillReturnRows(loginUserRows().AddRow(1, "alice", "alice@example.com", hash, "active", nil, time.Now(), time.Now(), nil))

	resp, err := service.Login(context.Background(), v1.LoginRequest{
		Account:  "alice",
		Password: "secret123",
		DeviceID: "device-1",
		Platform: "web",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if resp.Token == "" || resp.SessionID == "" || resp.ExpiresAt.IsZero() {
		t.Fatalf("Login() returned incomplete auth response: %+v", resp)
	}

	storedVersion, err := redisServer.Get(cache.SessionPrefix + resp.SessionID)
	if err != nil {
		t.Fatalf("session version was not stored in redis: %v", err)
	}
	if resp.RefreshToken == "" {
		t.Fatal("Login() returned empty refresh token")
	}
	if _, err := jwt.ParseToken(resp.RefreshToken, service.Config.JWT.Secret, service.Config.JWT.Issuer); err == nil {
		t.Fatal("refresh token must not be a valid access JWT")
	}
	refreshHash := jwt.HashRefreshToken(resp.RefreshToken)
	if storedSessionID, err := redisServer.Get(cache.RefreshPrefix + refreshHash); err != nil || storedSessionID != resp.SessionID {
		t.Fatalf("refresh mapping = %q err = %v, want session %q", storedSessionID, err, resp.SessionID)
	}
	claims, err := jwt.ParseToken(resp.Token, service.Config.JWT.Secret, service.Config.JWT.Issuer)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != "1" || claims.Username != "alice" || claims.SessionID != resp.SessionID || claims.DeviceID != "device-1" || claims.Platform != "web" || claims.Version != storedVersion {
		t.Fatalf("unexpected claims: %+v, storedVersion=%q response=%+v", claims, storedVersion, resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestLoginCreatesIndependentSessionsForSameUser(t *testing.T) {
	service, redisServer, mock := newLoginTestService(t)
	hash, err := passwd.HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`SELECT .* FROM "users" WHERE username = .* LIMIT .*`).
			WillReturnRows(loginUserRows().AddRow(1, "alice", "alice@example.com", hash, "active", nil, time.Now(), time.Now(), nil))
	}

	first, err := service.Login(context.Background(), v1.LoginRequest{
		Account:  "alice",
		Password: "secret123",
		DeviceID: "device-1",
		Platform: "web",
	})
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	second, err := service.Login(context.Background(), v1.LoginRequest{
		Account:  "alice",
		Password: "secret123",
		DeviceID: "device-2",
		Platform: "android",
	})
	if err != nil {
		t.Fatalf("second Login() error = %v", err)
	}
	if first.SessionID == second.SessionID {
		t.Fatalf("session IDs are equal: %q", first.SessionID)
	}

	if _, err := redisServer.Get(cache.SessionPrefix + first.SessionID); err != nil {
		t.Fatalf("first session missing from redis: %v", err)
	}
	if _, err := redisServer.Get(cache.SessionPrefix + second.SessionID); err != nil {
		t.Fatalf("second session missing from redis: %v", err)
	}
	if err := service.DeleteSession(context.Background(), first.SessionID); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if redisServer.Exists(cache.SessionPrefix + first.SessionID) {
		t.Fatal("first session still exists after delete")
	}
	if !redisServer.Exists(cache.SessionPrefix + second.SessionID) {
		t.Fatal("second session was deleted with first session")
	}
}

func newLoginTestService(t *testing.T) (*AuthService, *miniredis.Miniredis, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}

	redisServer := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	return &AuthService{
		DB:    db,
		Cache: redisClient,
		Config: &config.Config{
			Cache: &options.RedisOptions{
				CommandTimeout: 100 * time.Millisecond,
			},
			JWT: &options.JWTOptions{
				Issuer:     "Beehive-IM",
				Secret:     "12345678901234567890123456789012",
				AccessTTL:  time.Hour,
				RefreshTTL: 2 * time.Hour,
			},
		},
	}, redisServer, mock
}

func loginUserRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"username",
		"email",
		"password_hash",
		"status",
		"last_login_at",
		"created_at",
		"updated_at",
		"deleted_at",
	})
}
