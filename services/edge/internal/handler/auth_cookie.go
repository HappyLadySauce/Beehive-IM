package handler

import (
	"net/http"
	"strings"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
)

const defaultRefreshCookieName = "refresh_token"

func refreshTokenFromCookie(r *http.Request, conf config.RefreshCookieConf) (string, bool) {
	name := refreshCookieName(conf)
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(cookie.Value)
	return token, token != ""
}

func setRefreshCookie(w http.ResponseWriter, env string, conf config.RefreshCookieConf, token string) {
	if token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName(conf),
		Value:    token,
		Path:     refreshCookiePath(conf),
		Domain:   strings.TrimSpace(conf.Domain),
		MaxAge:   refreshCookieMaxAge(conf),
		HttpOnly: true,
		Secure:   refreshCookieSecure(env, conf),
		SameSite: refreshCookieSameSite(conf),
	})
}

func clearRefreshCookie(w http.ResponseWriter, env string, conf config.RefreshCookieConf) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName(conf),
		Value:    "",
		Path:     refreshCookiePath(conf),
		Domain:   strings.TrimSpace(conf.Domain),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   refreshCookieSecure(env, conf),
		SameSite: refreshCookieSameSite(conf),
	})
}

func refreshCookieName(conf config.RefreshCookieConf) string {
	name := strings.TrimSpace(conf.Name)
	if name == "" {
		return defaultRefreshCookieName
	}
	return name
}

func refreshCookiePath(conf config.RefreshCookieConf) string {
	path := strings.TrimSpace(conf.Path)
	if path == "" {
		return "/v1/auth"
	}
	return path
}

func refreshCookieMaxAge(conf config.RefreshCookieConf) int {
	if conf.MaxAgeSeconds <= 0 {
		return 30 * 24 * 60 * 60
	}
	return conf.MaxAgeSeconds
}

func refreshCookieSecure(env string, conf config.RefreshCookieConf) bool {
	if conf.Secure {
		return true
	}
	if refreshCookieSameSite(conf) == http.SameSiteNoneMode {
		return true
	}
	env = strings.ToLower(strings.TrimSpace(env))
	if env == "dev" || env == "test" {
		return false
	}
	return !conf.AllowInsecureNonDev
}

func refreshCookieSameSite(conf config.RefreshCookieConf) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(conf.SameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
