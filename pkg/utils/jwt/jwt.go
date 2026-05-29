package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    UserID   string `json:"user_id"`	// 用户 ID
    Username string `json:"username"`	// 用户名
    Version  string `json:"version"`	// 版本号
    jwt.RegisteredClaims				// 内嵌标准 JWT 声明，如 exp、iat、sub 等
}

// GenerateToken creates a JWT token with the given user information and expiration time.
// GenerateToken 使用给定的用户信息和过期时间生成 JWT 令牌。
func GenerateToken(userID, username, version, secretKey string, expiresAt *jwt.NumericDate) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Version:  version,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expiresAt, // 设置过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()), // 设置签发时间
			NotBefore: jwt.NewNumericDate(time.Now()), // 设置生效时间
			Subject:   userID, // 设置主题
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ParseToken validates the JWT token and returns the claims if valid.
// ParseToken 验证 JWT 令牌并返回有效的声明。
func ParseToken(tokenStr, secretKey string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}
