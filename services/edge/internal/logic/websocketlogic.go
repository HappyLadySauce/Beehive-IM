// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type WebsocketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWebsocketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WebsocketLogic {
	return &WebsocketLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WebsocketLogic) Websocket(w http.ResponseWriter, r *http.Request) error {
	l.svcCtx.Proxy.ServeHTTP(w, r)
	return nil
}
