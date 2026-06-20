package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// --- Local account / 本地账号登录 ---
func (l *RegisterLogic) Register(in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.RegisterResponse{}, nil
}
