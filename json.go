package speck

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"gopkg.in/yaml.v3"
)

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)

	if err := json.NewEncoder(response).Encode(payload); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}

func (api *API) openapiJSON(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, api.spec)
}

func (api *API) openapiYAML(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/yaml")
	response.WriteHeader(http.StatusOK)

	if err := yaml.NewEncoder(response).Encode(api.spec); err != nil {
		slog.Error("failed to write OpenAPI YAML response", "error", err)
	}
}
