package service

import (
	"context"

	v1 "github.com/HappyLadySauce/Beehive-IM/api/user/v1"
	"github.com/HappyLadySauce/Beehive-IM/app/user/service/internal/biz"
	"google.golang.org/protobuf/types/known/emptypb"
)

// UserService implements api.user.v1.UserServer.
// UserService 实现用户 gRPC/HTTP 接口。
type UserService struct {
	v1.UnimplementedUserServer

	uc *biz.UserUsecase
}

// NewUserService creates a UserService.
// NewUserService 创建用户服务实例。
func NewUserService(uc *biz.UserUsecase) *UserService {
	return &UserService{uc: uc}
}

// Ping implements User.Ping.
func (s *UserService) Ping(ctx context.Context, _ *emptypb.Empty) (*v1.PingReply, error) {
	return &v1.PingReply{Message: "pong"}, nil
}

// GetUser implements User.GetUser.
func (s *UserService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserReply, error) {
	user, err := s.uc.GetUser(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &v1.GetUserReply{
		User: &v1.UserInfo{
			Id:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Avatar:   user.Avatar,
		},
	}, nil
}
