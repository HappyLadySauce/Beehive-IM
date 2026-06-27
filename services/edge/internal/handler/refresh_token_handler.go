// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/logic"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func refreshTokenHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RefreshTokenRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		refreshToken, ok := refreshTokenFromCookie(r, svcCtx.Config.Auth.RefreshCookie)
		if !ok {
			writeJSONError(w, r, http.StatusUnauthorized, "MISSING_REFRESH_TOKEN", "Missing refresh token")
			return
		}
		req.RefreshToken = refreshToken

		l := logic.NewRefreshTokenLogic(r.Context(), svcCtx)
		resp, err := l.RefreshToken(&req)
		if err != nil {
			writeHandlerError(w, r, err)
		} else {
			setRefreshCookie(w, svcCtx.Config.Env, svcCtx.Config.Auth.RefreshCookie, resp.RefreshToken)
			resp.RefreshToken = ""
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
