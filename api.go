package speck

import (
	"net/http"
	"strings"

	"github.com/invopop/jsonschema"
)

// API groups typed routes, OpenAPI metadata, and optional JWT auth.
type API struct {
	mux       *http.ServeMux
	reflector *jsonschema.Reflector
	spec      openapiSpec
	jwtAuth   *jwtAuthenticator
	obscureIf func(RouteCx) bool
}

// New creates API with built-in OpenAPI endpoints.
func New(title string) *API {
	api := &API{
		mux:       http.NewServeMux(),
		reflector: &jsonschema.Reflector{DoNotReference: true, Anonymous: false},
		spec:      newOpenAPISpec(title),
	}

	api.mux.HandleFunc("GET /openapi.json", api.openapiJSON)
	api.mux.HandleFunc("GET /openapi.yaml", api.openapiYAML)

	return api
}

// Description sets API description used in OpenAPI output.
func (api *API) Description(description string) *API {
	api.spec.Info.Description = description
	return api
}

// Version sets API version used in OpenAPI output.
func (api *API) Version(version string) *API {
	api.spec.Info.Version = version
	return api
}

// Server appends server URL to OpenAPI output.
func (api *API) Server(url string) *API {
	api.spec.Servers = append(api.spec.Servers, openapiServer{URL: url})
	return api
}

// JWTAuth requires bearer JWT auth for all registered routes.
func (api *API) JWTAuth(signingKey []byte) *API {
	api.jwtAuth = newJWTAuthenticator(signingKey)
	return api
}

// ObscureIf hides all routes, including OpenAPI endpoints, when predicate returns true.
func (api *API) ObscureIf(shouldObscure func(RouteCx) bool) *API {
	api.obscureIf = shouldObscure
	return api
}

// AllowUnsignedJWT permits alg=none tokens when JWT auth enabled.
func (api *API) AllowUnsignedJWT() *API {
	if api.jwtAuth == nil {
		api.jwtAuth = newJWTAuthenticator(nil)
	}
	api.jwtAuth.AllowUnsigned()
	return api
}

// Routes registers routes on API.
func (api *API) Routes(routeDefinitions ...routeDefinition) *API {
	for _, route := range routeDefinitions {
		handler := route.handler(api.reflector, func() *jwtAuthenticator { return api.jwtAuth })
		api.mux.Handle(handler.pattern(), handler)
		api.spec = api.spec.withRoute(handler.schema())
	}

	return api
}

// PrefixRoutes registers routes after applying shared path prefix.
func (api *API) PrefixRoutes(prefix string, routeDefinitions ...routeDefinition) *API {
	return api.Routes(Prefix(prefix, routeDefinitions...)...)
}

// Mount mounts handler beneath prefix on mux.
func Mount(mux *http.ServeMux, prefix string, handler http.Handler) {
	prefix = normalizeMountPrefix(prefix)
	mounted := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != prefix && !strings.HasPrefix(request.URL.Path, prefix+"/") {
			http.NotFound(response, request)
			return
		}

		requestCopy := new(http.Request)
		*requestCopy = *request
		urlCopy := *request.URL
		urlCopy.Path = strings.TrimPrefix(request.URL.Path, prefix)
		if urlCopy.Path == "" {
			urlCopy.Path = "/"
		}
		if request.URL.RawPath != "" {
			urlCopy.RawPath = strings.TrimPrefix(request.URL.RawPath, prefix)
			if urlCopy.RawPath == "" {
				urlCopy.RawPath = "/"
			}
		}
		requestCopy.URL = &urlCopy

		handler.ServeHTTP(response, requestCopy)
	})

	mux.Handle(prefix, mounted)
	mux.Handle(prefix+"/", mounted)
}

func normalizeMountPrefix(prefix string) string {
	prefix = "/" + strings.Trim(prefix, "/")
	if prefix == "/" {
		return ""
	}
	return prefix
}

func (api *API) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if api.shouldObscure(request) {
		http.NotFound(response, request)
		return
	}

	api.mux.ServeHTTP(response, request)
}

func (api *API) shouldObscure(request *http.Request) bool {
	if api.obscureIf == nil {
		return false
	}

	authToken := authTokenFromRequest(request)
	var token *TokenPayload
	if api.jwtAuth != nil && authToken != "" {
		parsedToken, err := api.jwtAuth.parse(authToken)
		if err == nil {
			token = parsedToken
		}
	}

	cx := newRouteCx(request.Context(), nil, request.URL.Query(), authToken, token)
	return api.obscureIf(cx)
}
