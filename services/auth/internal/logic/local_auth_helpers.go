package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/pkg/authjwt"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	minPasswordBytes       = 8
	maxPasswordBytes       = 72
	defaultRefreshTTL      = 30 * 24 * time.Hour
	bcryptCost             = 12
	invalidCredentialMsg   = "invalid account or password"
	invalidRefreshTokenMsg = "invalid refresh token"
)

// issueLoginResponse signs access and refresh tokens for one user.
// issueLoginResponse 为单个用户签发访问令牌和刷新令牌。
func issueLoginResponse(ctx context.Context, svcCtx *svc.ServiceContext, user repository.User) (*pb.LoginResponse, error) {
	accessToken, expiresIn, err := svcCtx.JWT.Sign(strconv.FormatInt(user.ID, 10), user.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sign access token failed")
	}
	refreshToken, err := authjwt.RandomToken(32)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate refresh token failed")
	}
	if err := svcCtx.Auth.CreateRefreshToken(ctx, user.ID, repository.HashRefreshToken(refreshToken), time.Now().UTC().Add(refreshTTL(svcCtx))); err != nil {
		return nil, status.Errorf(codes.Internal, "persist refresh token failed")
	}
	return loginResponse(accessToken, refreshToken, expiresIn, user), nil
}

func loginResponse(accessToken, refreshToken string, expiresIn int64, user repository.User) *pb.LoginResponse {
	return &pb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		TokenType:    "Bearer",
		User: &pb.UserInfo{
			Id:       strconv.FormatInt(user.ID, 10),
			Username: user.Username,
			Email:    user.Email,
			Avatar:   user.Avatar,
		},
	}
}

func userIDString(user repository.User) string {
	return strconv.FormatInt(user.ID, 10)
}

func refreshTTL(svcCtx *svc.ServiceContext) time.Duration {
	if svcCtx == nil || svcCtx.Config.RefreshTokenTTLSeconds <= 0 {
		return defaultRefreshTTL
	}
	return time.Duration(svcCtx.Config.RefreshTokenTTLSeconds) * time.Second
}

func validateRegisterInput(in *pb.RegisterRequest) (username, email, phone, password string, err error) {
	if in == nil {
		return "", "", "", "", status.Error(codes.InvalidArgument, "request is required")
	}
	username = strings.TrimSpace(in.GetUsername())
	email = strings.TrimSpace(in.GetEmail())
	phone = strings.TrimSpace(in.GetPhone())
	password = in.GetPassword()
	switch {
	case len(username) < 3 || len(username) > 50:
		err = status.Error(codes.InvalidArgument, "username must be 3-50 characters")
	case len([]byte(password)) < minPasswordBytes:
		err = status.Error(codes.InvalidArgument, "password is too short")
	case len([]byte(password)) > maxPasswordBytes:
		err = status.Error(codes.InvalidArgument, "password is too long")
	case email != "" && (!strings.Contains(email, "@") || len(email) > 255):
		err = status.Error(codes.InvalidArgument, "invalid email")
	case len(phone) > 20:
		err = status.Error(codes.InvalidArgument, "invalid phone")
	}
	return username, email, phone, password, err
}

func validateLoginInput(in *pb.LoginRequest) (account, password string, err error) {
	if in == nil {
		return "", "", status.Error(codes.InvalidArgument, "request is required")
	}
	account = strings.TrimSpace(in.GetAccount())
	password = in.GetPassword()
	if account == "" || password == "" {
		return "", "", status.Error(codes.InvalidArgument, "account and password are required")
	}
	return account, password, nil
}

func authStatusError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, repository.ErrAccountExists):
		return status.Error(codes.AlreadyExists, "account already exists")
	case errors.Is(err, repository.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, invalidCredentialMsg)
	case errors.Is(err, repository.ErrInvalidRefreshToken):
		return status.Error(codes.Unauthenticated, invalidRefreshTokenMsg)
	default:
		return status.Error(codes.Internal, "auth service internal error")
	}
}

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func bcryptVerify(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return repository.ErrInvalidCredentials
	}
	return nil
}
