# speck

OAS API without the weight: a tiny Go toolkit for crisp JSON services with docs built in.

Features:
- typed route builders with Go generics
- request context wrapper with path params, query params, and auth token access
- optional JWT bearer auth with scope helpers
- generated OpenAPI 3.1 JSON and YAML docs
- compact error model for public vs internal failures
- route prefixing and handler mounting helpers

## Install

```bash
go get github.com/Vehmloewff/speck
```

## Imports

- `github.com/Vehmloewff/speck` — the API builder, routing, OpenAPI, and JWT helpers
- `github.com/vehmloewff/report` — structured application errors

## Quick start

```go
package main

import (
	"log"
	"net/http"

	"github.com/Vehmloewff/speck"
	"github.com/vehmloewff/report"
)

type helloOutput struct {
	Message string `json:"message"`
}

func main() {
	handler := speck.New("Example API").
		Description("Simple typed JSON API").
		Version("1.0.0").
		Routes(
			speck.Get("/hello", func(cx speck.RouteCx) (helloOutput, report.Err) {
				name := cx.QueryParam("name")
				if name == "" {
					name = "world"
				}
				return helloOutput{Message: "Hello, " + name + "!"}, nil
			}).
				Description("Returns a greeting.").
				DetailQueryParam("name", "Optional name to greet."),
		)

	log.Fatal(http.ListenAndServe(":8080", handler))
}
```

Routes above expose:
- `GET /hello`
- `GET /openapi.json`
- `GET /openapi.yaml`

## Route definitions

Use helpers:
- `speck.Get`
- `speck.Head`
- `speck.Post`
- `speck.Put`
- `speck.Patch`
- `speck.Delete`
- `speck.Route`

Each route takes typed input/output values:

```go
type createThingInput struct {
	Name string `json:"name"`
}

type createThingOutput struct {
	ID string `json:"id"`
}

speck.Post("/things", func(cx speck.RouteCx, input createThingInput) (createThingOutput, report.Err) {
	if input.Name == "" {
		return createThingOutput{}, report.New("name required")
	}
	return createThingOutput{ID: "thing_123"}, nil
})
```

## Request context

`speck.RouteCx` gives access to:
- request `Context()`
- bearer token via `AuthToken()` / `Auth()`
- parsed JWT via `Token()` / `GetToken()`
- path params via `PathParam()`
- query params via `QueryParam()` and `QueryParamValues()`

## JWT auth

Enable JWT auth for all routes on one API:

```go
handler := speck.New("Secure API").
	JWTAuth([]byte("secret-key")).
	Routes(
		speck.Get("/hello", func(cx speck.RouteCx) (map[string]string, report.Err) {
			if e := cx.Token().EnsureScope("hello:read"); e != nil {
				return nil, report.From(e)
			}
			return map[string]string{"sub": cx.Token().Subject}, nil
		}),
	)
```

Supported JWT algorithms:
- `HS256`
- `none` only when `AllowUnsignedJWT()` enabled

## Errors

Use package `report` for user-safe and internal errors:

```go
return output{}, report.New("bad input")
return output{}, report.From(dbErr).Internal()
```

Behavior:
- not found hints → HTTP 404
- not permitted hints → HTTP 403
- other public errors → HTTP 400 with the original message
- internal errors → HTTP 500 with `{"error":"Internal error"}`

## OpenAPI

Schemas come from `github.com/invopop/jsonschema`.

Generated docs include:
- API title, description, version, servers
- route descriptions
- path/query param descriptions
- request/response JSON schemas

### Struct tags for schema docs

OpenAPI schemas come from Go struct shapes plus `jsonschema` tags.
Use tags to add field descriptions, examples, defaults, enums, formats, and validation hints.

```go
type createThingInput struct {
	Name  string `json:"name" jsonschema:"title=Name,description=The display name for the thing,example=hello world"`
	Kind  string `json:"kind" jsonschema:"description=The thing kind,enum=small,enum=large,default=small"`
	Count int    `json:"count,omitempty" jsonschema:"description=How many things to create,minimum=1"`
}
```

Those tags flow into `openapi.json` and `openapi.yaml` automatically.

## Mounting under prefix

```go
mux := http.NewServeMux()
speck.Mount(mux, "/v1", handler)
```

Mounted handler still serves docs under mounted prefix, while OpenAPI path entries remain relative to handler route definitions.

## Development

```bash
go test ./...
```
