package handler

import (
	"errors"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/logic"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, logic.ErrUnauthorized), errors.Is(err, ticket.ErrMissingUserID):
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	case errors.Is(err, logic.ErrMissingDeviceID):
		http.Error(w, "Missing device id", http.StatusUnauthorized)
	case status.Code(err) == codes.InvalidArgument:
		http.Error(w, status.Convert(err).Message(), http.StatusBadRequest)
	case status.Code(err) == codes.Unauthenticated:
		http.Error(w, status.Convert(err).Message(), http.StatusUnauthorized)
	case status.Code(err) == codes.AlreadyExists:
		http.Error(w, status.Convert(err).Message(), http.StatusConflict)
	case status.Code(err) == codes.PermissionDenied:
		http.Error(w, status.Convert(err).Message(), http.StatusForbidden)
	default:
		httpx.ErrorCtx(r.Context(), w, err)
	}
}
