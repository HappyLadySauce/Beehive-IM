// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"errors"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/logic"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ackMessagesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AckMessagesRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewAckMessagesLogic(r.Context(), svcCtx)
		resp, err := l.AckMessages(&req, r)
		if err != nil {
			if errors.Is(err, logic.ErrMissingDebugUserID) {
				http.Error(w, "Missing X-Debug-User-Id header", http.StatusUnauthorized)
				return
			}
			if errors.Is(err, logic.ErrMissingDebugDeviceID) {
				http.Error(w, "Missing X-Debug-Device-Id header", http.StatusUnauthorized)
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
