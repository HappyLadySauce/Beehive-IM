package v1

import "time"

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	DeviceID string `json:"device_id" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

type RefreshRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AuthResponse returns access and refresh credentials only; session/expiry live in the JWT.
// AuthResponse 仅返回访问与刷新凭证；会话与过期时间由 JWT 承载。
type AuthResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	SessionID    string    `json:"session_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}
