package logic

import (
	"context"
	"strconv"

	"github.com/HappyLadySauce/Beehive-IM/services/user/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/user/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchGetUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchGetUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetUsersLogic {
	return &BatchGetUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchGetUsersLogic) BatchGetUsers(in *pb.BatchGetUsersRequest) (*pb.BatchGetUsersResponse, error) {
	users, missing, err := l.svcCtx.Users.BatchGetByIDs(l.ctx, in.GetIds())
	if err != nil {
		l.Errorf("batch query users failed: count=%d error=%v", len(in.GetIds()), err)
		return nil, err
	}

	resp := &pb.BatchGetUsersResponse{
		Users:      make([]*pb.GetUserResponse, 0, len(users)),
		MissingIds: missing,
	}
	for _, user := range users {
		resp.Users = append(resp.Users, &pb.GetUserResponse{
			Id:        strconv.FormatInt(user.ID, 10),
			Name:      user.Name,
			Email:     user.Email,
			Phone:     user.Phone,
			CreatedAt: user.CreatedAt.UTC().Format(timeFormat),
			UpdatedAt: user.UpdatedAt.UTC().Format(timeFormat),
		})
	}
	return resp, nil
}
