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
)

func TestRegisterCreatesUserStoresTokenVersionAndReturnsJWT(t *testing.T) {
	service, redisServer, mock := newRegisterTestService(t)
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*username.*email.* LIMIT .*`).
		WillReturnRows(userRows())
	mock.ExpectQuery(`INSERT INTO "users" .* RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	token, err := service.Register(context.Background(), v1.RegisterRequest{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if token == "" {
		t.Fatal("Register() returned empty token")
	}

	storedVersion, err := redisServer.Get(cache.UserTokenVersionPrefix + "1")
	if err != nil {
		t.Fatalf("token version was not stored in redis: %v", err)
	}
	if storedVersion == "" {
		t.Fatal("stored token version is empty")
	}

	claims, err := jwt.ParseToken(token, service.Config.JWT.Secret, service.Config.JWT.Issuer)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != "1" || claims.Username != "alice" || claims.Version != storedVersion {
		t.Fatalf("claims = {userID:%q username:%q version:%q}, want userID=1 username=alice version=%q", claims.UserID, claims.Username, claims.Version, storedVersion)
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
			JWT: &options.JWTOptions{
				Issuer:    "Beehive-IM",
				Secret:    "12345678901234567890123456789012",
				AccessTTL: time.Hour,
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
