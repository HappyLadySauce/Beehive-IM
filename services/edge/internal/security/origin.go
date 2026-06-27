package security

import "strings"

// OriginChecker validates browser origins against an explicit allowlist.
// OriginChecker 使用显式白名单校验浏览器 Origin。
type OriginChecker struct {
	allowed map[string]struct{}
}

func NewOriginChecker(origins []string) *OriginChecker {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		origin = normalizeOrigin(origin)
		if origin == "" {
			continue
		}
		allowed[origin] = struct{}{}
	}
	return &OriginChecker{allowed: allowed}
}

func (c *OriginChecker) Configured() bool {
	return c != nil && len(c.allowed) > 0
}

func (c *OriginChecker) Allowed(origin string) bool {
	if c == nil || len(c.allowed) == 0 {
		return false
	}
	_, ok := c.allowed[normalizeOrigin(origin)]
	return ok
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	return strings.TrimRight(origin, "/")
}
