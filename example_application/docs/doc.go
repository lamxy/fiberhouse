// Package docs is a compile-safe PLACEHOLDER for the swaggo/swag-generated
// OpenAPI documentation of example_application's HTTP API.
//
// This file is hand-written and intentionally minimal so that
// `go build ./...` and `go vet ./...` succeed WITHOUT ever running
// `swag init`. It is NOT a full API spec: the swagger.json/swagger.yaml
// artifacts and a richer generated docs.go (with the complete path/schema
// definitions parsed from the @Summary/@Param/@Success/... annotations on
// the handlers) only exist after `swag init` is run — see
// generate_swagger.sh and README.md in this directory.
//
// swag init OVERWRITES this file. That is expected and desired: once you
// run the generator, this hand-written placeholder is replaced by the real
// generated docs.go (same package name, same SwaggerInfo variable, but with
// docTemplate populated from the actual annotations). Do not hand-edit the
// generated output; edit the source annotations instead and regenerate.
//
// The general API annotations (@title, @version, @host, @BasePath, ...)
// that `swag init` needs are NOT declared in this package — swag reads them
// from a `-g` entrypoint file's package comment. example_application has no
// standalone HTTP server main package of its own (see README.md for why);
// the general annotations therefore live on example_main/main.go, which is
// the actual process that wires up example_application's providers/routes
// and serves its Swagger UI. Point `swag init -g` at that file (see
// generate_swagger.sh).
package docs

import "github.com/swaggo/swag"

// docTemplate is an intentionally empty OpenAPI (Swagger 2.0) document. It
// satisfies swag.Spec.SwaggerTemplate's shape so the swagger.HandlerDefault
// (gofiber/swagger) / ginSwagger.WrapHandler (swaggo/gin-swagger) UI, which
// is already wired in providers/module/{fiber,gin}_route_register.go, can
// render without panicking even before `swag init` has been run. Replace by
// regenerating; do not hand-edit.
const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {},
    "definitions": {}
}`

// SwaggerInfo holds the exported Swagger Info so callers/tests can inspect
// or override it, matching the shape swag init generates. Field values here
// are placeholders; swag init fills these from the @title/@version/@host/
// @BasePath annotations on the -g entrypoint (example_main/main.go).
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{"http", "https"},
	Title:            "FiberHouse Example Application API (placeholder — run swag init)",
	Description:      "Placeholder spec. Run generate_swagger.sh to produce the real generated docs.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
