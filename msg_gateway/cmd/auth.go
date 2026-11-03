package main

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

const (
	maxExactlyRepresentableJSONInteger = int64(1<<53 - 1)
	// minJWTKeyLen 只是配置体检阈值：低于该长度的密钥会在启动时告警，
	// 但不会拒绝握手，避免与 api_gateway 的签名强度要求不一致。
	minJWTKeyLen = 32
)

type tokenIdentity struct {
	UserID   int64
	Account  string
	TenantID int64
}

func tokenFromRequest(r *http.Request) string {
	const bearerPrefix = "bearer "
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authorization) > len(bearerPrefix) &&
		strings.EqualFold(authorization[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(authorization[len(bearerPrefix):])
	}
	// Browser WebSocket clients cannot set arbitrary Authorization headers, so
	// keep query-token compatibility while preferring the header when present.
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func parseUserIDClaim(value interface{}) (int64, bool) {
	switch id := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(id, 10, 64)
		return parsed, err == nil && parsed > 0
	case json.Number:
		parsed, err := id.Int64()
		return parsed, err == nil && parsed > 0
	case float64:
		if id <= 0 || id > float64(maxExactlyRepresentableJSONInteger) || math.Trunc(id) != id {
			return 0, false
		}
		return int64(id), true
	default:
		return 0, false
	}
}

func authenticateToken(tokenString string, key []byte) (tokenIdentity, error) {
	if tokenString == "" {
		return tokenIdentity{}, errors.New("missing token")
	}
	if len(key) == 0 {
		return tokenIdentity{}, errors.New("missing signing key")
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing algorithm")
			}
			return key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || token == nil || !token.Valid {
		if err == nil {
			err = errors.New("invalid token")
		}
		return tokenIdentity{}, err
	}

	userID, ok := parseUserIDClaim(claims["id"])
	if !ok {
		return tokenIdentity{}, errors.New("invalid user id claim")
	}
	account, ok := claims["account"].(string)
	account = strings.TrimSpace(account)
	if !ok || account == "" {
		return tokenIdentity{}, errors.New("invalid account claim")
	}
	// 多租户：消息链路必须携带租户，缺失即拒绝，避免退化成全局可见。
	tenantID, ok := parseUserIDClaim(claims["tenant_id"])
	if !ok {
		return tokenIdentity{}, errors.New("invalid tenant id claim")
	}
	return tokenIdentity{UserID: userID, Account: account, TenantID: tenantID}, nil
}

func originChecker(allowedOrigins []string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized, ok := normalizeOrigin(origin)
		if ok {
			allowed[normalized] = struct{}{}
		}
	}

	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		normalized, ok := normalizeOrigin(origin)
		if !ok {
			return false
		}
		originURL, err := url.Parse(normalized)
		if err != nil {
			return false
		}
		if strings.EqualFold(originURL.Host, r.Host) {
			return true
		}
		_, ok = allowed[normalized]
		return ok
	}
}

func normalizeOrigin(origin string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path != "" {
		return "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), true
}
