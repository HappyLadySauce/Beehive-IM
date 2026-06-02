package v1

import "time"

// RegisterRequest is the payload for user registration.
// RegisterRequest 为用户注册请求体。
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20" example:"alice"`
	Email    string `json:"email" binding:"required,email" example:"alice@example.com"`
	Password string `json:"password" binding:"required,min=6,max=100" example:"secret123"`
	DeviceID string `json:"device_id" binding:"required" example:"device-001"`
	Platform string `json:"platform" binding:"required" example:"web"`
}

// LoginRequest is the payload for user login.
// LoginRequest 为用户登录请求体。
type LoginRequest struct {
	Account  string `json:"account" binding:"required" example:"alice"`
	Password string `json:"password" binding:"required" example:"secret123"`
	DeviceID string `json:"device_id" binding:"required" example:"device-001"`
	Platform string `json:"platform" binding:"required" example:"web"`
}

// RefreshRequest is the payload for refreshing an access token.
// RefreshRequest 为刷新 access token 的请求体。
type RefreshRequest struct {
	SessionID    string `json:"session_id" binding:"required" example:"sess_abc123"`
	RefreshToken string `json:"refresh_token" binding:"required" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// AuthResponse is returned after successful authentication or registration.
// AuthResponse 为认证或注册成功后的响应体。
type AuthResponse struct {
	Token        string    `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string    `json:"refresh_token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	SessionID    string    `json:"session_id" example:"sess_abc123"`
	ExpiresAt    time.Time `json:"expires_at" example:"2026-06-02T12:00:00Z"`
}

// ErrorResponse is a simple JSON error payload.
// ErrorResponse 为简单 JSON 错误响应体。
type ErrorResponse struct {
	Error string `json:"error" example:"invalid request body"`
}

// MessageResponse is a simple JSON message payload.
// MessageResponse 为简单 JSON 消息响应体。
type MessageResponse struct {
	Message string `json:"message" example:"logged out"`
}
