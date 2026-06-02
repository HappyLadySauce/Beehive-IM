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
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/session"
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
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatalf("Login() returned incomplete auth response: %+v", resp)
	}
	if _, err := jwt.ParseToken(resp.RefreshToken, service.Config.JWT.Secret, service.Config.JWT.Issuer); err == nil {
		t.Fatal("refresh token must not be a valid access JWT")
	}
	claims, err := jwt.ParseToken(resp.Token, service.Config.JWT.Secret, service.Config.JWT.Issuer)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if !redisServer.Exists(cache.SessionIDPrefix + claims.SessionID) {
		t.Fatal("session was not stored in redis")
	}
	refreshHash := jwt.HashRefreshToken(resp.RefreshToken)
	if storedHash, err := redisServer.Get(cache.SessionIDPrefix + claims.SessionID); err != nil || storedHash != refreshHash {
		t.Fatalf("session value = %q err = %v, want refresh hash %q", storedHash, err, refreshHash)
	}
	if err := service.ValidateRefreshToken(context.Background(), claims.SessionID, resp.RefreshToken); err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}
	parsed, err := session.ParseSessionID(claims.SessionID)
	if err != nil {
		t.Fatalf("ParseSessionID() error = %v", err)
	}
	if parsed.UserID != "1" || parsed.Username != "alice" || parsed.DeviceID != "device-1" || parsed.Platform != "web" {
		t.Fatalf("unexpected session claims: %+v", parsed)
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
	firstClaims, err := jwt.ParseToken(first.Token, service.Config.JWT.Secret, service.Config.JWT.Issuer)
	if err != nil {
		t.Fatalf("ParseToken(first) error = %v", err)
	}
	secondClaims, err := jwt.ParseToken(second.Token, service.Config.JWT.Secret, service.Config.JWT.Issuer)
	if err != nil {
		t.Fatalf("ParseToken(second) error = %v", err)
	}
	if firstClaims.SessionID == secondClaims.SessionID {
		t.Fatalf("session IDs are equal: %q", firstClaims.SessionID)
	}

	if !redisServer.Exists(cache.SessionIDPrefix + firstClaims.SessionID) {
		t.Fatalf("first session missing from redis")
	}
	if !redisServer.Exists(cache.SessionIDPrefix + secondClaims.SessionID) {
		t.Fatalf("second session missing from redis")
	}
	if err := service.DeleteSession(context.Background(), firstClaims.SessionID); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if redisServer.Exists(cache.SessionIDPrefix + firstClaims.SessionID) {
		t.Fatal("first session still exists after delete")
	}
	if !redisServer.Exists(cache.SessionIDPrefix + secondClaims.SessionID) {
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
