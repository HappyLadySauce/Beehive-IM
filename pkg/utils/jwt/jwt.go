package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the access-token payload; refresh tokens are opaque and not JWTs.
// Claims 为访问令牌载荷；刷新令牌为不透明串，不是 JWT。
type Claims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
	Platform  string `json:"platform"`
	Version   string `json:"ver"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed HS256 access JWT.
// GenerateToken 签发 HS256 访问 JWT。
func GenerateToken(
	userID, username, sessionID, deviceID, platform, version,
	issuer, secretKey string,
	expiresAt *jwt.NumericDate,
) (string, error) {
	claims := Claims{
		UserID:    userID,
		Username:  username,
		SessionID: sessionID,
		DeviceID:  deviceID,
		Platform:  platform,
		Version:   version,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expiresAt,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ParseToken validates the access JWT and returns claims when valid.
// ParseToken 校验访问 JWT，有效时返回声明。
func ParseToken(tokenStr, secretKey, issuer string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	}, jwt.WithIssuer(issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}
