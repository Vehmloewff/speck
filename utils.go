package speck

import (
	"context"
	"net/url"
)

// NoRouteInput is placeholder input type for routes without a body.
type NoRouteInput struct{}

// RouteCx exposes request-derived data to route handlers.
type RouteCx struct {
	context     context.Context
	pathParams  map[string]string
	queryParams url.Values
	authToken   string
	token       *TokenPayload
}

func newRouteCx(ctx context.Context, pathParams map[string]string, queryParams url.Values, authToken string, token *TokenPayload) RouteCx {
	if ctx == nil {
		ctx = context.Background()
	}
	return RouteCx{
		context:     ctx,
		pathParams:  pathParams,
		queryParams: queryParams,
		authToken:   authToken,
		token:       token,
	}
}

func (cx RouteCx) Context() context.Context {
	if cx.context == nil {
		return context.Background()
	}
	return cx.context
}

func (cx RouteCx) AuthToken() string {
	return cx.authToken
}

func (cx RouteCx) Auth() string {
	return cx.authToken
}

func (cx RouteCx) GetToken() *TokenPayload {
	return cx.token
}

func (cx RouteCx) Token() *TokenPayload {
	return cx.token
}

func (cx RouteCx) PathParams() map[string]string {
	params := make(map[string]string, len(cx.pathParams))
	for name, value := range cx.pathParams {
		params[name] = value
	}
	return params
}

func (cx RouteCx) PathParam(name string) string {
	return cx.pathParams[name]
}

func (cx RouteCx) HasPathParam(name string) bool {
	_, ok := cx.pathParams[name]
	return ok
}

func (cx RouteCx) QueryParams() url.Values {
	params := make(url.Values, len(cx.queryParams))
	for name, values := range cx.queryParams {
		params[name] = append([]string(nil), values...)
	}
	return params
}

func (cx RouteCx) QueryParam(name string) string {
	return cx.queryParams.Get(name)
}

func (cx RouteCx) QueryParamValues(name string) []string {
	values := cx.queryParams[name]
	return append([]string(nil), values...)
}

func (cx RouteCx) HasQueryParam(name string) bool {
	_, ok := cx.queryParams[name]
	return ok
}
