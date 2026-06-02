package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims is the application payload embedded in access JWTs.
// TokenClaims 是写入访问 JWT 的业务载荷。
type TokenClaims struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	DeviceID  string `json:"device_id"`
	Platform  string `json:"platform"`
}

// Claims is the signed access-token payload.
// Claims 为签名访问令牌载荷。
type Claims struct {
	TokenClaims
	jwt.RegisteredClaims
}

// GenerateToken creates a signed HS256 access JWT bound to the given session and user claims.
// GenerateToken 签发与会话和用户声明绑定的 HS256 访问 JWT。
func GenerateToken(tokenClaims TokenClaims, issuer, secretKey string, expiresAt *jwt.NumericDate) (string, error) {
	claims := Claims{
		TokenClaims: tokenClaims,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expiresAt,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Subject:   tokenClaims.SessionID,
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
