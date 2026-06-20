package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLiveRoutesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLiveRoutesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLiveRoutesLogic {
	return &GetLiveRoutesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetLiveRoutes returns live routes for one user.
// GetLiveRoutes 返回单个用户的在线路由。
func (l *GetLiveRoutesLogic) GetLiveRoutes(in *pb.GetLiveRoutesRequest) (*pb.GetLiveRoutesResponse, error) {
	routes, err := l.svcCtx.Store.GetLiveRoutes(l.ctx, in.GetUserId())
	if err != nil {
		l.Errorf("presence get live routes failed: user_id=%s code=%s", in.GetUserId(), store.CodeForError(err))
		return nil, err
	}

	resp := &pb.GetLiveRoutesResponse{
		Routes: make([]*pb.ConnectionMeta, 0, len(routes)),
	}
	for _, route := range routes {
		resp.Routes = append(resp.Routes, pbConnection(route))
	}
	return resp, nil
}
