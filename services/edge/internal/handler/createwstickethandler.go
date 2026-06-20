// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"errors"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/logic"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func createWsTicketHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TicketRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewCreateWsTicketLogic(r.Context(), svcCtx)
		resp, err := l.CreateWsTicket(&req, r)
		if err != nil {
			if errors.Is(err, ticket.ErrMissingUserID) {
				http.Error(w, "Missing X-Debug-User-Id header", http.StatusUnauthorized)
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
