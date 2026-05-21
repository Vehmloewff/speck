package speck

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	report "github.com/vehmloewff/report"
)

func TestJWTAuthParsesSignedBearerToken(t *testing.T) {
	secret := []byte("test-secret")
	token := SignJWT(map[string]any{
		"sub":   "hello-world",
		"scope": "hello:read hello:write",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, secret)
	request := httptest.NewRequest(http.MethodGet, "/token", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	New("Test API").
		Routes(
			Get("/token", func(cx RouteCx) (struct {
				Message string `json:"message"`
			}, report.Err) {
				payload := cx.GetToken()
				if payload == nil {
					return struct {
						Message string `json:"message"`
					}{Message: "missing token"}, nil
				}
				if err := payload.EnsureScope("hello:read"); err != nil {
					return struct {
						Message string `json:"message"`
					}{Message: err.Error()}, nil
				}
				if err := payload.ForbidScope("hello:admin"); err != nil {
					return struct {
						Message string `json:"message"`
					}{Message: err.Error()}, nil
				}
				return struct {
					Message string `json:"message"`
				}{Message: payload.Subject}, nil
			}),
		).
		JWTAuth(secret).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var output struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if output.Message != "hello-world" {
		t.Fatalf("expected token subject, got %q", output.Message)
	}
}

func TestJWTAuthAllowsUnsignedTokenWhenConfigured(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Authorization", "Bearer "+unsignedJWTForTest(map[string]any{"sub": "test"}))
	response := httptest.NewRecorder()

	New("Test API").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		JWTAuth([]byte("test-secret")).
		AllowUnsignedJWT().
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}

func TestJWTAuthRejectsUnsignedTokenByDefault(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Authorization", "Bearer "+unsignedJWTForTest(map[string]any{"sub": "test"}))
	response := httptest.NewRecorder()

	New("Test API").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		JWTAuth([]byte("test-secret")).
		ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestJWTAuthRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	token := SignJWT(map[string]any{
		"sub": "hello-world",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, []byte("wrong-secret"))
	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	New("Test API").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		JWTAuth([]byte("test-secret")).
		ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestTokenPayloadScopeHelpers(t *testing.T) {
	payload := &TokenPayload{Scopes: []string{"read", "write"}}
	if err := payload.EnsureScope("read"); err != nil {
		t.Fatalf("expected read scope to be allowed: %v", err)
	}
	if err := payload.EnsureScope("admin"); !errors.Is(err, ErrMissingScope) {
		t.Fatalf("expected missing scope error, got %v", err)
	}
	if err := payload.ForbidScope("admin"); err != nil {
		t.Fatalf("expected absent admin scope to be allowed: %v", err)
	}
	if err := payload.ForbidScope("write"); !errors.Is(err, ErrForbiddenScope) {
		t.Fatalf("expected forbidden scope error, got %v", err)
	}
}
