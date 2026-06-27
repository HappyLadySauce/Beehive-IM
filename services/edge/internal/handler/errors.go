package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/logic"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type errorResponse struct {
	Success   bool   `json:"success"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func installHTTPErrorHandler() {
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		statusCode, code, message := errorDetails(err, http.StatusBadRequest)
		return statusCode, errorResponse{
			Success:   false,
			ErrorCode: code,
			Message:   message,
		}
	})
}

func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	statusCode, code, message := errorDetails(err, http.StatusInternalServerError)
	writeJSONError(w, r, statusCode, code, message)
}

func writeJSONError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	if code == "" {
		code = "REQUEST_FAILED"
	}
	if message == "" {
		message = "Request failed"
	}
	httpx.WriteJsonCtx(r.Context(), w, statusCode, errorResponse{
		Success:   false,
		ErrorCode: code,
		Message:   message,
	})
}

func errorDetails(err error, fallbackStatus int) (int, string, string) {
	switch {
	case errors.Is(err, logic.ErrUnauthorized), errors.Is(err, ticket.ErrMissingUserID):
		return http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized"
	case errors.Is(err, logic.ErrMissingDeviceID):
		return http.StatusUnauthorized, "MISSING_DEVICE_ID", "Missing device id"
	case errors.Is(err, ticket.ErrMissingOrigin), errors.Is(err, ticket.ErrOriginMismatch):
		return http.StatusForbidden, "INVALID_ORIGIN", "Origin is not allowed"
	case status.Code(err) == codes.InvalidArgument:
		return http.StatusBadRequest, "INVALID_ARGUMENT", status.Convert(err).Message()
	case status.Code(err) == codes.Unauthenticated:
		return http.StatusUnauthorized, "UNAUTHENTICATED", status.Convert(err).Message()
	case status.Code(err) == codes.AlreadyExists:
		return http.StatusConflict, "ALREADY_EXISTS", status.Convert(err).Message()
	case status.Code(err) == codes.PermissionDenied:
		return http.StatusForbidden, "PERMISSION_DENIED", status.Convert(err).Message()
	default:
		return fallbackStatus, "REQUEST_FAILED", err.Error()
	}
}
