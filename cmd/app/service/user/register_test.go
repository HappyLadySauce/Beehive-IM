package user

import (
	"context"
	"errors"
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
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/session"
)

func TestRegisterCreatesUserStoresSessionAndReturnsAuthResponse(t *testing.T) {
	service, redisServer, mock := newRegisterTestService(t)
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*username.*email.* LIMIT .*`).
		WillReturnRows(userRows())
	mock.ExpectQuery(`INSERT INTO "users" .* RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	resp, err := service.Register(context.Background(), v1.RegisterRequest{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "secret123",
		DeviceID: "device-1",
		Platform: "web",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatalf("Register() returned incomplete auth response: %+v", resp)
	}

	claims, err := jwt.ParseToken(resp.Token, service.Config.JWT.Secret, service.Config.JWT.Issuer)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if !redisServer.Exists(cache.SessionIDPrefix + claims.SessionID) {
		t.Fatal("session was not stored in redis")
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

func TestRegisterRejectsExistingUsernameOrEmail(t *testing.T) {
	service, _, mock := newRegisterTestService(t)
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*username.*email.* LIMIT .*`).
		WillReturnRows(userRows().AddRow(1, "alice", "alice@example.com", "hashed", "active", nil, time.Now(), time.Now(), nil))

	_, err := service.Register(context.Background(), v1.RegisterRequest{
		Username: "alice",
		Email:    "other@example.com",
		Password: "secret123",
		DeviceID: "device-1",
		Platform: "web",
	})
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("Register() error = %v, want ErrUserAlreadyExists", err)
	}

	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*username.*email.* LIMIT .*`).
		WillReturnRows(userRows().AddRow(1, "alice", "alice@example.com", "hashed", "active", nil, time.Now(), time.Now(), nil))
	_, err = service.Register(context.Background(), v1.RegisterRequest{
		Username: "other",
		Email:    "alice@example.com",
		Password: "secret123",
		DeviceID: "device-1",
		Platform: "web",
	})
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("Register() error = %v, want ErrUserAlreadyExists", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func newRegisterTestService(t *testing.T) (*UserService, *miniredis.Miniredis, sqlmock.Sqlmock) {
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

	return &UserService{
		DB:    db,
		Cache: redisClient,
		Config: &config.Config{
			Cache: &options.RedisOptions{
				CommandTimeout: 100 * time.Millisecond,
			},
			JWT: &options.JWTOptions{
				Issuer:     "Beehive-IM",
				Secret:    "12345678901234567890123456789012",
				AccessTTL:  time.Hour,
				RefreshTTL: 2 * time.Hour,
			},
		},
	}, redisServer, mock
}

func userRows() *sqlmock.Rows {
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
