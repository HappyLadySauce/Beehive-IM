package session

import (
	"fmt"
    "encoding/base64"
    "encoding/json"
)

type SessionClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	DeviceID  string `json:"device_id"`
	Platform  string `json:"platform"`
}

// GenerateSessionID 将 Claims 转换为一个 URL 安全的字符串密钥
func GenerateSessionID(c SessionClaims) (string, error) {
    data, err := json.Marshal(c)
    if err != nil {
        return "", fmt.Errorf("marshal claims: %w", err)
    }
    // 使用 URL 安全的 Base64 编码，无填充（可选，RawURLEncoding 去掉尾部的 '='）
    return base64.RawURLEncoding.EncodeToString(data), nil
}

// ParseSessionID 将字符串密钥解析回 Claims
func ParseSessionID(sessionID string) (SessionClaims, error) {
    data, err := base64.RawURLEncoding.DecodeString(sessionID)
    if err != nil {
        return SessionClaims{}, fmt.Errorf("decode session id: %w", err)
    }
    var c SessionClaims
    if err := json.Unmarshal(data, &c); err != nil {
        return SessionClaims{}, fmt.Errorf("unmarshal claims: %w", err)
    }
    return c, nil
}
