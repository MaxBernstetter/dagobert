package playbook

import (
	"encoding/json"
	_ "embed"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaResourceURI must match the $id in src/strategy/schema/playbook.schema.json (and the embedded copy).
const SchemaResourceURI = "https://github.com/MaxBernstetter/trading-copilot/src/strategy/schema/playbook.schema.json"

//go:embed playbook.schema.json
var embeddedSchema []byte

// LoadSchemaCompiler returns a JSON Schema compiler for playbook documents.
// SCHEMA_PATH, when set, overrides the embedded schema file.
func LoadSchemaCompiler() (*jsonschema.Compiler, error) {
	data := embeddedSchema
	if path := os.Getenv("SCHEMA_PATH"); path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%w: read schema: %v", ErrSchemaLoad, err)
		}
	}

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%w: parse schema json: %v", ErrSchemaLoad, err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(SchemaResourceURI, doc); err != nil {
		return nil, fmt.Errorf("%w: add schema resource: %v", ErrSchemaLoad, err)
	}
	if _, err := compileSchema(c); err != nil {
		return nil, err
	}
	return c, nil
}

func compileSchema(compiler *jsonschema.Compiler) (sch *jsonschema.Schema, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrSchemaLoad, r)
		}
	}()
	sch, err = compiler.Compile(SchemaResourceURI)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaLoad, err)
	}
	return sch, nil
}
