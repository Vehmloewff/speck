package speck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	report "github.com/vehmloewff/report"
)

func TestGetHelloRoute(t *testing.T) {
	type helloWorldOutput struct {
		Message string `json:"message"`
	}

	helloWorld := func(_ RouteCx) (helloWorldOutput, report.Err) {
		return helloWorldOutput{Message: "Hello, world!"}, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	New("Test API").
		Description("Test API description").
		Version("1.0.0").
		Routes(
			Get("/hello", helloWorld).
				Description("Returns a friendly greeting."),
		).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", contentType)
	}

	var output struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if output.Message != "Hello, world!" {
		t.Fatalf("expected greeting, got %q", output.Message)
	}
}

func TestPrefixedHelloRoute(t *testing.T) {
	type helloWorldOutput struct {
		Message string `json:"message"`
	}

	helloWorld := func(_ RouteCx) (helloWorldOutput, report.Err) {
		return helloWorldOutput{Message: "Hello, world!"}, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	New("Test API").
		Description("Test API description").
		Version("1.0.0").
		PrefixRoutes("v1", Get("/hello", helloWorld).Description("Returns a friendly greeting.")).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}

func TestPrefixNormalizesSlashes(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	New("Test API").
		PrefixRoutes("/v1/", Get("hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}

func TestMountRoutesPrefixToHandler(t *testing.T) {
	type helloWorldOutput struct {
		Message string `json:"message"`
	}

	helloWorld := func(_ RouteCx) (helloWorldOutput, report.Err) {
		return helloWorldOutput{Message: "Hello, world!"}, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	mux := http.NewServeMux()
	Mount(mux, "/v1", New("Test API").Routes(Get("/hello", helloWorld)))
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}
