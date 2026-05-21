package speck

import "github.com/invopop/jsonschema"

type openapiSpec struct {
	OpenAPI string                  `json:"openapi" yaml:"openapi"`
	Info    openapiInfo             `json:"info" yaml:"info"`
	Servers []openapiServer         `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths   map[string]openapiPaths `json:"paths" yaml:"paths"`
}

type openapiInfo struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

type openapiServer struct {
	URL string `json:"url" yaml:"url"`
}

type openapiPaths map[string]openapiOperation

type openapiOperation struct {
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters  []openapiParameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *openapiRequestBody        `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]openapiResponse `json:"responses" yaml:"responses"`
}

type openapiParameter struct {
	Name        string             `json:"name" yaml:"name"`
	In          string             `json:"in" yaml:"in"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool               `json:"required" yaml:"required"`
	Schema      *jsonschema.Schema `json:"schema" yaml:"schema"`
}

type openapiRequestBody struct {
	Required bool                        `json:"required" yaml:"required"`
	Content  map[string]openapiMediaType `json:"content" yaml:"content"`
}

type openapiResponse struct {
	Description string                      `json:"description" yaml:"description"`
	Content     map[string]openapiMediaType `json:"content,omitempty" yaml:"content,omitempty"`
}

type openapiMediaType struct {
	Schema *jsonschema.Schema `json:"schema" yaml:"schema"`
}

type openapiRoute struct {
	method    string
	path      string
	operation openapiOperation
}

func newOpenAPISpec(title string) openapiSpec {
	return openapiSpec{
		OpenAPI: "3.1.0",
		Info: openapiInfo{
			Title: title,
		},
		Paths: map[string]openapiPaths{},
	}
}

func (spec openapiSpec) withRoute(route openapiRoute) openapiSpec {
	if spec.Paths[route.path] == nil {
		spec.Paths[route.path] = openapiPaths{}
	}

	spec.Paths[route.path][route.method] = route.operation
	return spec
}
