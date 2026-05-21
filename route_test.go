package speck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	report "github.com/vehmloewff/report"
)

func TestRouteCxIncludesRequestContext(t *testing.T) {
	type contextKey string
	const key contextKey = "request-id"

	request := httptest.NewRequest(http.MethodGet, "/context", nil)
	request = request.WithContext(context.WithValue(request.Context(), key, "from-request"))
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	New("Test API").Routes(
		Get("/context", func(cx RouteCx) (struct {
			Message string `json:"message"`
		}, report.Err) {
			return struct {
				Message string `json:"message"`
			}{Message: cx.Context().Value(key).(string)}, nil
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var output struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if output.Message != "from-request" {
		t.Fatalf("expected request context value, got %q", output.Message)
	}
}

func TestRouteUserErrorReturnsBadRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/fail", nil)
	response := httptest.NewRecorder()

	New("Test API").Routes(
		Get("/fail", func(_ RouteCx) (struct{}, report.Err) {
			return struct{}{}, report.New("bad input")
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}

	var output map[string]string
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if output["error"] != "bad input" {
		t.Fatalf("expected user error message, got %q", output["error"])
	}
}

func TestRouteInternalErrorReturnsInternalServerError(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/fail", nil)
	response := httptest.NewRecorder()

	New("Test API").Routes(
		Get("/fail", func(_ RouteCx) (struct{}, report.Err) {
			return struct{}{}, report.From(errors.New("something broke")).Internal()
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}

	var output map[string]string
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if output["error"] != "Internal error" {
		t.Fatalf("expected internal error message, got %q", output["error"])
	}
}

func TestRouteNotFoundHintReturnsNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/fail", nil)
	response := httptest.NewRecorder()

	New("Test API").Routes(
		Get("/fail", func(_ RouteCx) (struct{}, report.Err) {
			return struct{}{}, report.New("thing missing").Hint(report.HintNotFound)
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestRouteNotPermittedHintReturnsForbidden(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/fail", nil)
	response := httptest.NewRecorder()

	New("Test API").Routes(
		Get("/fail", func(_ RouteCx) (struct{}, report.Err) {
			return struct{}{}, report.New("forbidden").Hint(report.HintNotPermitted)
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestObscureIfReturnsNotFoundWithoutRunningRoute(t *testing.T) {
	routeRan := false
	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	New("Test API").
		ObscureIf(func(cx RouteCx) bool {
			return cx.AuthToken() == "test-token"
		}).
		Routes(
			Get("/hello", func(_ RouteCx) (struct {
				Message string `json:"message"`
			}, report.Err) {
				routeRan = true
				return struct {
					Message string `json:"message"`
				}{Message: "Hello, world!"}, nil
			}),
		).
		ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
	if routeRan {
		t.Fatal("expected obscured route not to run")
	}
}

func TestObscureIfCanUseParsedToken(t *testing.T) {
	secret := []byte("test-secret")
	token := SignJWT(map[string]any{
		"sub":   "hidden-thing",
		"scope": "hide",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, secret)
	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	New("Test API").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		JWTAuth(secret).
		ObscureIf(func(cx RouteCx) bool {
			return cx.GetToken() != nil && cx.GetToken().HasScope("hide")
		}).
		ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestUnsupportedRoute(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	response := httptest.NewRecorder()

	New("Test API").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}
