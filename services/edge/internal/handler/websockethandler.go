// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/logic"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func websocketHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewWebsocketLogic(r.Context(), svcCtx)
		if err := l.Websocket(w, r); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		}
	}
}
