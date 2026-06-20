package logic

import (
	"context"
	"errors"
	"strconv"

	"github.com/HappyLadySauce/Beehive-IM/services/user/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/user/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/user/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserLogic {
	return &GetUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserLogic) GetUser(in *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := l.svcCtx.Users.GetByID(l.ctx, in.GetId())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			l.Infof("user not found: user_id=%s", in.GetId())
		} else {
			l.Errorf("query user failed: user_id=%s error=%v", in.GetId(), err)
		}
		return nil, err
	}

	return &pb.GetUserResponse{
		Id:        strconv.FormatInt(user.ID, 10),
		Name:      user.Name,
		Email:     user.Email,
		Phone:     user.Phone,
		CreatedAt: user.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt: user.UpdatedAt.UTC().Format(timeFormat),
	}, nil
}

const timeFormat = "2006-01-02T15:04:05Z07:00"
