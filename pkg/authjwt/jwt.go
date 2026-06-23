package authjwt

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultIssuer           = "beehive-im"
	defaultAccessTTLSeconds = int64(900)
	defaultDevSecret        = "beehive-im-dev-secret-change-me"
)

var (
	ErrMissingSecret = errors.New("jwt secret is required")
	ErrInvalidToken  = errors.New("invalid jwt token")
)

// Config describes shared JWT signing and verification settings.
// Config 描述共享 JWT 签发与校验配置。
type Config struct {
	Secret             string `json:",optional"`
	Issuer             string `json:",default=beehive-im"`
	AccessTTLSeconds   int64  `json:",default=900"`
	PreferEnvWhenEmpty bool   `json:",default=true"`
	AllowDevDefault    bool   `json:",default=true"`
}

// Claims is the Beehive access token claim set.
// Claims 是 Beehive 访问令牌载荷。
type Claims struct {
	Username string `json:"username,omitempty"`
	jwt.RegisteredClaims
}

// Manager signs and verifies HS256 access tokens.
// Manager 签发并校验 HS256 访问令牌。
type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewManager builds a JWT manager from config.
// NewManager 根据配置创建 JWT 管理器。
func NewManager(c Config) (*Manager, error) {
	normalized := c.Normalize()
	if strings.TrimSpace(normalized.Secret) == "" {
		return nil, ErrMissingSecret
	}
	return &Manager{
		secret: []byte(normalized.Secret),
		issuer: normalized.Issuer,
		ttl:    time.Duration(normalized.AccessTTLSeconds) * time.Second,
	}, nil
}

// Normalize applies environment fallback and safe local defaults.
// Normalize 应用环境变量回退和安全的本地默认值。
func (c Config) Normalize() Config {
	if c.PreferEnvWhenEmpty || c.Secret == "" {
		if env := strings.TrimSpace(os.Getenv("JWT_SECRET")); c.Secret == "" && env != "" {
			c.Secret = env
		}
	}
	if strings.TrimSpace(c.Secret) == "" && c.AllowDevDefault {
		c.Secret = defaultDevSecret
	}
	if strings.TrimSpace(c.Issuer) == "" {
		c.Issuer = defaultIssuer
	}
	if c.AccessTTLSeconds <= 0 {
		c.AccessTTLSeconds = defaultAccessTTLSeconds
	}
	return c
}

// Sign creates a compact JWT and returns its TTL in seconds.
// Sign 创建紧凑 JWT，并返回秒级有效期。
func (m *Manager) Sign(userID, username string) (string, int64, error) {
	if m == nil || len(m.secret) == 0 {
		return "", 0, ErrMissingSecret
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", 0, errors.New("user id is required")
	}
	now := time.Now().UTC()
	jti, err := RandomToken(16)
	if err != nil {
		return "", 0, err
	}
	claims := Claims{
		Username: strings.TrimSpace(username),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign jwt: %w", err)
	}
	return token, int64(m.ttl.Seconds()), nil
}

// Verify parses and validates one compact JWT.
// Verify 解析并校验一个紧凑 JWT。
func (m *Manager) Verify(token string) (*Claims, error) {
	if m == nil || len(m.secret) == 0 {
		return nil, ErrMissingSecret
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrInvalidToken
	}
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil || !parsed.Valid || claims.Subject == "" {
		if err == nil {
			err = ErrInvalidToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return claims, nil
}

// RandomToken returns URL-safe cryptographic random token text.
// RandomToken 返回 URL 安全的密码学随机令牌文本。
func RandomToken(byteLen int) (string, error) {
	if byteLen <= 0 {
		byteLen = 32
	}
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
