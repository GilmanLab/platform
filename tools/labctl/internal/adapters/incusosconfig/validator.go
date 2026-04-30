package incusosconfig

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/yaml"
	schemaincusos "github.com/gilmanlab/platform/schemas/lab/incusos"
)

const imageBuildDefinition = "#ImageBuild"

// Validator validates IncusOS image build YAML against the shared CUE schema.
type Validator struct{}

// New constructs a Validator.
func New() Validator {
	return Validator{}
}

// ValidateYAML validates YAML input and decodes a defaulted ImageBuild.
func (Validator) ValidateYAML(filename string, data []byte) (schemaincusos.ImageBuild, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileString(schemaincusos.SchemaSource())
	if err := schema.Err(); err != nil {
		return schemaincusos.ImageBuild{}, fmt.Errorf("compile IncusOS schema: %w", err)
	}

	source, err := yaml.Extract(filename, data)
	if err != nil {
		return schemaincusos.ImageBuild{}, fmt.Errorf("parse %s as YAML: %w", filename, err)
	}

	input := ctx.BuildFile(source)
	value := schema.LookupPath(cue.ParsePath(imageBuildDefinition)).Unify(input)
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return schemaincusos.ImageBuild{}, fmt.Errorf("validate %s: %w", filename, err)
	}

	var config schemaincusos.ImageBuild
	if err := value.Decode(&config); err != nil {
		return schemaincusos.ImageBuild{}, fmt.Errorf("decode %s: %w", filename, err)
	}

	return config, nil
}
