package main

import (
	"fmt"
	"os"
	"strings"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// camelCase converts a snake_case string to camelCase
func camelCase(s string) string {
	caser := cases.Title(language.Und)
	parts := strings.Split(s, "_")
	for i := range parts {
		parts[i] = caser.String(parts[i])
	}
	return parts[0] + strings.Join(parts[1:], "")
}

// protoFieldToGoMethod converts a protobuf field name to its corresponding Go method name
func protoFieldToGoMethod(fieldName string) string {
	// Split by underscore and convert each part to title case
	caser := cases.Title(language.Und)
	parts := strings.Split(fieldName, "_")
	for i := range parts {
		parts[i] = caser.String(parts[i])
	}
	return "Get" + strings.Join(parts, "")
}

// findResourceTypes inspects the Resource message's oneof field to find all possible types
func findResourceTypes() map[string]map[string]string {
	types := make(map[string]map[string]string)

	// Get the Resource message descriptor
	resource := &pb.Resource{}
	descriptor := resource.ProtoReflect().Descriptor()

	// Get all fields
	fields := descriptor.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.ContainingOneof() != nil {
			// Found a oneof field
			oneofFields := field.ContainingOneof().Fields()
			for j := 0; j < oneofFields.Len(); j++ {
				oneofField := oneofFields.Get(j)
				name := string(oneofField.Name())
				kind := string(oneofField.Message().Name())
				getter := protoFieldToGoMethod(name)
				specName := camelCase(name)
				types[kind] = map[string]string{
					"getter":   getter,
					"specName": specName,
				}
			}
		}
	}

	return types
}

func main() {
	// Get all oneof types dynamically
	types := findResourceTypes()
	if len(types) == 0 {
		panic("No resource types found in Resource message")
	}

	// Generate the code
	code := `//go:generate go run ../../../tools/gen-registry/main.go

// Code generated by gen-registry. DO NOT EDIT.2

package api

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

// Resource kind constants
const (
`

	// Generate kind constants
	for kind := range types {
		code += fmt.Sprintf("\tKind%s = \"%s\"\n", kind, kind)
	}

	code += `)

// KindFor returns the resource kind for a specific type
func KindFor[T proto.Message]() string {
	var zero T
	switch any(zero).(type) {
`
	// Generate kind switch cases
	for kind := range types {
		code += fmt.Sprintf("\tcase *%s:\n\t\treturn Kind%s\n", kind, kind)
	}
	code += "\tdefault:\n\t\tpanic(\"unregistered type\")\n\t}\n}\n\n"

	// Generate ExtractSpec function
	code += `// ExtractSpec extracts the typed spec from a resource using the protobuf type system
func ExtractSpec[T proto.Message](resource *Resource) (T, error) {
    var zero T
    switch any(zero).(type) {
`
	for kind, fields := range types {
		code += fmt.Sprintf("\tcase *%s:\n", kind)
		code += fmt.Sprintf("\t\tif spec := resource.%s(); spec != nil {\n", fields["getter"])
		code += "\t\t\treturn any(spec).(T), nil\n"
		code += "\t\t}\n"
	}
	code += `    }
    return zero, fmt.Errorf("resource does not contain spec of type %T", zero)
}

`

	// Generate SetSpec function
	code += `// SetSpec sets the spec field in a resource based on the type
func SetSpec[T proto.Message](resource *Resource, spec T) error {
	switch s := any(spec).(type) {
`
	for kind, fields := range types {
		code += fmt.Sprintf("\tcase *%s:\n", kind)
		code += fmt.Sprintf("\t\tresource.Spec = &Resource_%s{%s: s}\n", fields["specName"], fields["specName"])
		code += "\t\treturn nil\n"
	}
	code += `	default:
		return fmt.Errorf("unsupported resource type: %T", spec)
	}
}
`

	// Write the generated code to registry.go
	err := os.WriteFile("registry.go", []byte(code), 0644)
	if err != nil {
		panic(err)
	}
}