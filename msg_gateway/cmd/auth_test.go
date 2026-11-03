package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const testKey = "0123456789abcdef0123456789abcdef01234567" // 40 字节

func signHS256(t *testing.T, key string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(key))
	if err != nil {
		t.Fatalf("签发测试 token 失败: %v", err)
	}
	return signed
}

func TestAuthenticateTokenAcceptsStringIDClaim(t *testing.T) {
	token := signHS256(t, testKey, jwt.MapClaims{
		"id":        "9223372036854775807",
		"account":   "alice",
		"tenant_id": "1001",
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
	identity, err := authenticateToken(token, []byte(testKey))
	if err != nil {
		t.Fatalf("期望校验通过, 实际: %v", err)
	}
	if identity.UserID != 9223372036854775807 || identity.Account != "alice" || identity.TenantID != 1001 {
		t.Fatalf("身份解析错误: %+v", identity)
	}
}

func TestAuthenticateTokenRejectsMissingTenant(t *testing.T) {
	token := signHS256(t, testKey, jwt.MapClaims{
		"id":      "1",
		"account": "alice",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	if _, err := authenticateToken(token, []byte(testKey)); err == nil {
		t.Fatal("缺少 tenant_id 的令牌必须被拒绝")
	}
}

func TestAuthenticateTokenRejectsNoneAlgorithm(t *testing.T) {
	// 手工构造 alg=none 的令牌，验证不会被接受
	unsigned := jwt.NewWithClaims(jwt.GetSigningMethod("none"), jwt.MapClaims{
		"id":      "1",
		"account": "alice",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	token, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("构造 alg=none 令牌失败: %v", err)
	}
	if _, err := authenticateToken(token, []byte(testKey)); err == nil {
		t.Fatal("alg=none 令牌必须被拒绝")
	}
}

func TestAuthenticateTokenRejectsExpiredAndEmptyKey(t *testing.T) {
	expired := signHS256(t, testKey, jwt.MapClaims{
		"id":        "1",
		"account":   "alice",
		"tenant_id": "1001",
		"exp":       time.Now().Add(-time.Minute).Unix(),
	})
	if _, err := authenticateToken(expired, []byte(testKey)); err == nil {
		t.Fatal("过期令牌必须被拒绝")
	}

	valid := signHS256(t, testKey, jwt.MapClaims{
		"id":        "1",
		"account":   "alice",
		"tenant_id": "1001",
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
	if _, err := authenticateToken(valid, nil); err == nil {
		t.Fatal("缺少签名密钥时必须拒绝校验")
	}
}

func TestTokenFromRequestPrefersHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/ws?token=from-query", nil)
	r.Header.Set("Authorization", "Bearer from-header")
	if got := tokenFromRequest(r); got != "from-header" {
		t.Fatalf("应优先使用 Authorization 头, 实际: %q", got)
	}

	r2 := httptest.NewRequest(http.MethodGet, "/ws?token=from-query", nil)
	if got := tokenFromRequest(r2); got != "from-query" {
		t.Fatalf("应回退到 query token, 实际: %q", got)
	}
}

func TestOriginChecker(t *testing.T) {
	check := originChecker([]string{"http://localhost:3000"})

	allow := httptest.NewRequest(http.MethodGet, "/ws", nil)
	allow.Host = "example.com"
	allow.Header.Set("Origin", "http://localhost:3000")
	if !check(allow) {
		t.Fatal("白名单内的来源应放行")
	}

	deny := httptest.NewRequest(http.MethodGet, "/ws", nil)
	deny.Host = "example.com"
	deny.Header.Set("Origin", "http://evil.example")
	if check(deny) {
		t.Fatal("白名单外的来源应拒绝")
	}

	// 非浏览器客户端（无 Origin）应放行
	noOrigin := httptest.NewRequest(http.MethodGet, "/ws", nil)
	if !check(noOrigin) {
		t.Fatal("缺少 Origin 的请求应放行")
	}

	// 同源请求应放行
	same := httptest.NewRequest(http.MethodGet, "/ws", nil)
	same.Host = "example.com"
	same.Header.Set("Origin", "http://example.com")
	if !check(same) {
		t.Fatal("同源请求应放行")
	}
}

