package speck

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"

	report "github.com/vehmloewff/report"
)

func schemaForType(reflector *jsonschema.Reflector, valueType reflect.Type) *jsonschema.Schema {
	value := reflect.New(valueType).Interface()
	return reflector.Reflect(value)
}

type routeParamDetail struct {
	name        string
	description string
}

type routeDefinition struct {
	method            string
	path              string
	description       string
	pathParamDetails  []routeParamDetail
	queryParamDetails []routeParamDetail
	inputType         reflect.Type
	outputType        reflect.Type
	run               func(RouteCx, any) (any, report.Err)
}

// Route builds generic route definition.
func Route[Input any, Output any](method string, path string, run func(RouteCx, Input) (Output, report.Err)) routeDefinition {
	return routeDefinition{
		method:     method,
		path:       path,
		inputType:  reflect.TypeFor[Input](),
		outputType: reflect.TypeFor[Output](),
		run: func(cx RouteCx, input any) (any, report.Err) {
			return run(cx, input.(Input))
		},
	}
}

// Get builds GET route.
func Get[Output any](path string, run func(RouteCx) (Output, report.Err)) routeDefinition {
	return Route(http.MethodGet, path, func(cx RouteCx, _ NoRouteInput) (Output, report.Err) {
		return run(cx)
	})
}

// Head builds HEAD route.
func Head[Output any](path string, run func(RouteCx) (Output, report.Err)) routeDefinition {
	return Route(http.MethodHead, path, func(cx RouteCx, _ NoRouteInput) (Output, report.Err) {
		return run(cx)
	})
}

// Post builds POST route.
func Post[Input any, Output any](path string, run func(RouteCx, Input) (Output, report.Err)) routeDefinition {
	return Route(http.MethodPost, path, run)
}

// Put builds PUT route.
func Put[Input any, Output any](path string, run func(RouteCx, Input) (Output, report.Err)) routeDefinition {
	return Route(http.MethodPut, path, run)
}

// Patch builds PATCH route.
func Patch[Input any, Output any](path string, run func(RouteCx, Input) (Output, report.Err)) routeDefinition {
	return Route(http.MethodPatch, path, run)
}

// Delete builds DELETE route.
func Delete[Input any, Output any](path string, run func(RouteCx, Input) (Output, report.Err)) routeDefinition {
	return Route(http.MethodDelete, path, run)
}

// Prefix applies shared prefix to route list.
func Prefix(prefix string, routeDefinitions ...routeDefinition) []routeDefinition {
	prefixedRoutes := make([]routeDefinition, 0, len(routeDefinitions))
	for _, route := range routeDefinitions {
		prefixedRoutes = append(prefixedRoutes, route.withPrefix(prefix))
	}
	return prefixedRoutes
}

// Description sets route description for OpenAPI output.
func (r routeDefinition) Description(description string) routeDefinition {
	r.description = description
	return r
}

// DetailPathParam adds path param docs.
func (r routeDefinition) DetailPathParam(name string, description string) routeDefinition {
	r.pathParamDetails = append(r.pathParamDetails, routeParamDetail{name: name, description: description})
	return r
}

// DetailQueryParam adds query param docs.
func (r routeDefinition) DetailQueryParam(name string, description string) routeDefinition {
	r.queryParamDetails = append(r.queryParamDetails, routeParamDetail{name: name, description: description})
	return r
}

func (r routeDefinition) withPrefix(prefix string) routeDefinition {
	r.path = joinRoutePaths(prefix, r.path)
	return r
}

func joinRoutePaths(prefix string, path string) string {
	prefix = strings.Trim(prefix, "/")
	path = strings.TrimLeft(path, "/")

	if prefix == "" {
		if path == "" {
			return "/"
		}
		return "/" + path
	}
	if path == "" {
		return "/" + prefix
	}
	return "/" + prefix + "/" + path
}

func (r routeDefinition) handler(reflector *jsonschema.Reflector, getJWTAuth func() *jwtAuthenticator) routeHandler {
	return routeHandler{
		method:            r.method,
		path:              r.path,
		description:       r.description,
		pathParamDetails:  r.pathParamDetails,
		queryParamDetails: r.queryParamDetails,
		inputType:         r.inputType,
		inputSchema:       schemaForType(reflector, r.inputType),
		outputSchema:      schemaForType(reflector, r.outputType),
		run:               r.run,
		getJWTAuth:        getJWTAuth,
	}
}

type routeHandler struct {
	method            string
	path              string
	description       string
	pathParamDetails  []routeParamDetail
	queryParamDetails []routeParamDetail
	inputType         reflect.Type
	inputSchema       *jsonschema.Schema
	outputSchema      *jsonschema.Schema
	run               func(RouteCx, any) (any, report.Err)
	getJWTAuth        func() *jwtAuthenticator
}

func (h routeHandler) jwtAuth() *jwtAuthenticator {
	if h.getJWTAuth == nil {
		return nil
	}
	return h.getJWTAuth()
}

func (h routeHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	cx, er := h.readRouteCx(request)
	if er != nil {
		h.writeError(response, request, report.From(er))
		return
	}

	input, er := h.readInput(request)
	if er != nil {
		h.writeError(response, request, report.From(er))
		return
	}

	var output any
	{
		var er report.Err
		output, er = h.run(cx, input)
		if er != nil {
			h.writeError(response, request, er)
			return
		}
	}

	writeJSON(response, http.StatusOK, output)
}

func (h routeHandler) writeError(response http.ResponseWriter, request *http.Request, er report.Err) {
	status := report.HTTPStatus(er)
	log := slog.Warn
	if status >= http.StatusInternalServerError {
		log = slog.Error
	}

	log("route error", "method", request.Method, "path", request.URL.Path, "status", status, "error", er.String())
	writeJSON(response, status, map[string]string{"error": er.UserMessage()})
}

func (h routeHandler) pattern() string {
	return h.method + " " + h.path
}

func (h routeHandler) schema() openapiRoute {
	return openapiRoute{
		method: strings.ToLower(h.method),
		path:   h.path,
		operation: openapiOperation{
			Description: h.description,
			Parameters:  h.openapiParameters(),
			RequestBody: &openapiRequestBody{
				Required: false,
				Content: map[string]openapiMediaType{
					"application/json": {Schema: h.inputSchema},
				},
			},
			Responses: map[string]openapiResponse{
				"200": {
					Description: "OK",
					Content: map[string]openapiMediaType{
						"application/json": {Schema: h.outputSchema},
					},
				},
			},
		},
	}
}

func (h routeHandler) openapiParameters() []openapiParameter {
	parameters := make([]openapiParameter, 0, len(h.pathParamDetails)+len(h.queryParamDetails))
	for _, detail := range h.pathParamDetails {
		parameters = append(parameters, openapiParameter{
			Name:        detail.name,
			In:          "path",
			Description: detail.description,
			Required:    true,
			Schema:      &jsonschema.Schema{Type: "string"},
		})
	}
	for _, detail := range h.queryParamDetails {
		parameters = append(parameters, openapiParameter{
			Name:        detail.name,
			In:          "query",
			Description: detail.description,
			Required:    false,
			Schema:      &jsonschema.Schema{Type: "string"},
		})
	}
	return parameters
}

func (h routeHandler) readRouteCx(request *http.Request) (RouteCx, error) {
	authToken := authTokenFromRequest(request)
	var token *TokenPayload
	jwtAuth := h.jwtAuth()
	if jwtAuth != nil {
		if authToken == "" {
			return RouteCx{}, ErrMissingToken
		}

		var er error
		token, er = jwtAuth.parse(authToken)
		if er != nil {
			return RouteCx{}, er
		}
	}

	pathParams := map[string]string{}
	for _, name := range h.pathParamNames() {
		pathParams[name] = request.PathValue(name)
	}

	return newRouteCx(request.Context(), pathParams, request.URL.Query(), authToken, token), nil
}

func (h routeHandler) pathParamNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, name := range pathParamNamesFromPattern(h.path) {
		seen[name] = true
		names = append(names, name)
	}
	for _, detail := range h.pathParamDetails {
		if !seen[detail.name] {
			seen[detail.name] = true
			names = append(names, detail.name)
		}
	}
	return names
}

func pathParamNamesFromPattern(pattern string) []string {
	var names []string
	for _, segment := range strings.Split(pattern, "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			name = strings.TrimSuffix(name, "...")
			if name != "" && name != "$" {
				names = append(names, name)
			}
		}
	}
	return names
}

func authTokenFromRequest(request *http.Request) string {
	auth := strings.TrimSpace(request.Header.Get("Authorization"))
	parts := strings.Fields(auth)
	if len(parts) > 0 && strings.EqualFold(parts[0], "bearer") {
		if len(parts) == 1 {
			return ""
		}
		return strings.Join(parts[1:], " ")
	}
	return auth
}

func (h routeHandler) readInput(request *http.Request) (any, error) {
	inputPointer := reflect.New(h.inputType).Interface()
	if request.Body == nil {
		return reflect.ValueOf(inputPointer).Elem().Interface(), nil
	}

	decoder := json.NewDecoder(request.Body)
	if er := decoder.Decode(inputPointer); er != nil && !errors.Is(er, io.EOF) {
		return nil, er
	}

	return reflect.ValueOf(inputPointer).Elem().Interface(), nil
}
