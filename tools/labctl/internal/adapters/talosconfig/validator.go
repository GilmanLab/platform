package talosconfig

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/yaml"
	schematalos "github.com/gilmanlab/platform/schemas/lab/talos"
)

const imageBuildDefinition = "#ImageBuild"

// Validator validates Talos image build YAML against the shared CUE schema.
type Validator struct{}

// New constructs a Validator.
func New() Validator {
	return Validator{}
}

// ValidateYAML validates YAML input and decodes a defaulted ImageBuild.
func (Validator) ValidateYAML(filename string, data []byte) (schematalos.ImageBuild, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileString(schematalos.SchemaSource())
	if err := schema.Err(); err != nil {
		return schematalos.ImageBuild{}, fmt.Errorf("compile Talos schema: %w", err)
	}

	source, err := yaml.Extract(filename, data)
	if err != nil {
		return schematalos.ImageBuild{}, fmt.Errorf("parse %s as YAML: %w", filename, err)
	}

	input := ctx.BuildFile(source)
	value := schema.LookupPath(cue.ParsePath(imageBuildDefinition)).Unify(input)
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return schematalos.ImageBuild{}, fmt.Errorf("validate %s: %w", filename, err)
	}

	var config schematalos.ImageBuild
	if err := value.Decode(&config); err != nil {
		return schematalos.ImageBuild{}, fmt.Errorf("decode %s: %w", filename, err)
	}

	return config, nil
}
