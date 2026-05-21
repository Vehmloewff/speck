package speck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	report "github.com/vehmloewff/report"
)

func TestMountExposesOpenAPIUnderPrefix(t *testing.T) {
	type helloOutput struct {
		Message string `json:"message"`
	}

	makeRouter := func() http.Handler {
		return New("Test API").
			Description("Test API description").
			Version("1.0.0").
			Routes(
				Get("/hello", func(_ RouteCx) (helloOutput, report.Err) {
					return helloOutput{Message: "Hello, world!"}, nil
				}).Description("Returns a friendly greeting."),
			)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	response := httptest.NewRecorder()

	mux := http.NewServeMux()
	Mount(mux, "/v1", makeRouter())
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var schema map[string]any
	if err := json.NewDecoder(response.Body).Decode(&schema); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	paths, ok := schema["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths, got %#v", schema["paths"])
	}
	if _, ok := paths["/hello"]; !ok {
		t.Fatalf("expected mounted schema to keep handler-relative /hello path, got %#v", paths)
	}
}

func TestObscureIfObscuresOpenAPISchema(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()

	New("Test API").
		ObscureIf(func(_ RouteCx) bool { return true }).
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestOpenAPIJSONRoute(t *testing.T) {
	type helloOutput struct {
		Message string `json:"message"`
	}

	request := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()

	New("Test API").
		Description("Test API description").
		Version("1.0.0").
		Routes(
			Get("/hello", func(_ RouteCx) (helloOutput, report.Err) {
				return helloOutput{Message: "Hello, world!"}, nil
			}).Description("Returns a friendly greeting."),
		).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var schema map[string]any
	if err := json.NewDecoder(response.Body).Decode(&schema); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	info, ok := schema["info"].(map[string]any)
	if !ok {
		t.Fatalf("expected info, got %#v", schema["info"])
	}
	if info["title"] != "Test API" {
		t.Fatalf("expected title Test API, got %#v", info["title"])
	}
	if info["description"] != "Test API description" {
		t.Fatalf("expected description Test API description, got %#v", info["description"])
	}
	if info["version"] != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %#v", info["version"])
	}

	paths, ok := schema["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths, got %#v", schema["paths"])
	}
	helloPath, ok := paths["/hello"].(map[string]any)
	if !ok {
		t.Fatalf("expected /hello path, got %#v", paths["/hello"])
	}
	getOperation, ok := helloPath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected get operation, got %#v", helloPath["get"])
	}
	if getOperation["description"] != "Returns a friendly greeting." {
		t.Fatalf("expected route description, got %#v", getOperation["description"])
	}
	responses, ok := getOperation["responses"].(map[string]any)
	if !ok {
		t.Fatalf("expected responses, got %#v", getOperation["responses"])
	}
	okResponse, ok := responses["200"].(map[string]any)
	if !ok {
		t.Fatalf("expected 200 response, got %#v", responses["200"])
	}
	content, ok := okResponse["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected content, got %#v", okResponse["content"])
	}
	jsonContent, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON content, got %#v", content["application/json"])
	}
	outputSchema, ok := jsonContent["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected output schema, got %#v", jsonContent["schema"])
	}
	properties, ok := outputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected output properties schema, got %#v", outputSchema["properties"])
	}
	message, ok := properties["message"].(map[string]any)
	if !ok {
		t.Fatalf("expected message property schema, got %#v", properties["message"])
	}
	if message["type"] != "string" {
		t.Fatalf("expected message type string, got %#v", message["type"])
	}
}

func TestOpenAPIJSONIncludesPrefixedRoute(t *testing.T) {
	type helloOutput struct {
		Message string `json:"message"`
	}

	request := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()

	New("Test API").
		Description("Test API description").
		Version("1.0.0").
		PrefixRoutes("v1",
			Get("/hello", func(_ RouteCx) (helloOutput, report.Err) {
				return helloOutput{Message: "Hello, world!"}, nil
			}).Description("Returns a friendly greeting."),
		).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var schema map[string]any
	if err := json.NewDecoder(response.Body).Decode(&schema); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	paths, ok := schema["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths, got %#v", schema["paths"])
	}
	if _, ok := paths["/v1/hello"]; !ok {
		t.Fatalf("expected /v1/hello path, got %#v", paths["/v1/hello"])
	}
}

func TestOpenAPIYAMLRoute(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	response := httptest.NewRecorder()

	New("Test API").
		Description("Test API description").
		Version("1.0.0").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/yaml" {
		t.Fatalf("expected content type application/yaml, got %q", contentType)
	}
}

func TestRemovedPerRouteSchemaRoute(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/hello/schema", nil)
	response := httptest.NewRecorder()

	New("Test API").
		Description("Test API description").
		Version("1.0.0").
		Routes(Get("/hello", func(_ RouteCx) (struct{}, report.Err) { return struct{}{}, nil })).
		ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}
