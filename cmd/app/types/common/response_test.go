package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponseMapsAppErrorToHTTPStatusAndSafeMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	Response(ctx, NewUnauthorized("invalid credentials", errors.New("database timeout")), nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("HTTP status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); body != `{"code":401,"message":"invalid credentials","data":null}` {
		t.Fatalf("body = %s", body)
	}
}

func TestResponseHidesUnclassifiedInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	Response(ctx, errors.New(`pq: duplicate key value violates unique constraint "secret_index"`), nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("HTTP status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if body := rec.Body.String(); body != `{"code":500,"message":"internal server error","data":null}` {
		t.Fatalf("body = %s", body)
	}
}
