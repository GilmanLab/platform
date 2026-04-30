package incusos

import _ "embed"

//go:embed schema.cue
var schemaSource string

// SchemaSource returns the CUE source for the IncusOS schema package.
func SchemaSource() string {
	return schemaSource
}
