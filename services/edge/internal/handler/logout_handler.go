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

func logoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogoutRequest
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

		l := logic.NewLogoutLogic(r.Context(), svcCtx)
		resp, err := l.Logout(&req)
		if err != nil {
			writeHandlerError(w, r, err)
		} else {
			clearRefreshCookie(w, svcCtx.Config.Env, svcCtx.Config.Auth.RefreshCookie)
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
