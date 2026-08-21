// Package api embeds the OpenAPI contract so the server can serve its
// own documentation without filesystem dependencies.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
