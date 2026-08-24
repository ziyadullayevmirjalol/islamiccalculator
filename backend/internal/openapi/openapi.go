// Package openapi embeds the API contract. It lives under internal/
// (not api/) because Vercel treats every file in api/ as a serverless
// function entrypoint.
package openapi

import _ "embed"

//go:embed openapi.yaml
var Spec []byte
