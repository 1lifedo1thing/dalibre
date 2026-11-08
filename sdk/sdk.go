// Package sdk holds what plugin authors are given: the API's type
// declarations, the manifest's JSON Schema and a starter plugin.
package sdk

import "embed"

// Files are dalibre.d.ts, plugin.schema.json and template/.
//
//go:embed dalibre.d.ts plugin.schema.json template
var Files embed.FS
