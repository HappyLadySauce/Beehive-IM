package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the minimal access-token payload; user/device fields live inside session_id.
// Claims 为最小访问令牌载荷；用户与设备信息编码在 session_id 中。
type Claims struct {
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed HS256 access JWT bound to sessionID.
// GenerateToken 签发与 sessionID 绑定的 HS256 访问 JWT。
func GenerateToken(sessionID, issuer, secretKey string, expiresAt *jwt.NumericDate) (string, error) {
	claims := Claims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expiresAt,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Subject:   sessionID,
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
