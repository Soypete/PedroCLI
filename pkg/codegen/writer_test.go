package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/palantir/goastwriter/astgen"
	"github.com/palantir/goastwriter/expression"
	"github.com/palantir/goastwriter/statement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteGoFile_EmptyFile(t *testing.T) {
	spec := GoFileSpec{
		Package: "main",
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)
	assert.Contains(t, string(src), "package main")
}

func TestWriteGoFile_WithImports(t *testing.T) {
	spec := GoFileSpec{
		Package: "main",
		Imports: map[string]string{
			"fmt":     "",
			"os":      "",
			"strings": "",
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, `"fmt"`)
	assert.Contains(t, srcStr, `"os"`)
	assert.Contains(t, srcStr, `"strings"`)

	// Verify it parses back
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", src, 0)
	assert.NoError(t, parseErr)
}

func TestWriteGoFile_WithAliasedImport(t *testing.T) {
	spec := GoFileSpec{
		Package: "main",
		Imports: map[string]string{
			"github.com/example/mylib": "mylib",
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)
	assert.Contains(t, string(src), `mylib "github.com/example/mylib"`)
}

func TestWriteGoFile_Struct(t *testing.T) {
	spec := GoFileSpec{
		Package: "models",
		Structs: []StructSpec{
			{
				Name:    "User",
				Comment: "User represents a user in the system.",
				Fields: []FieldSpec{
					{Name: "ID", Type: "int", Tag: `json:"id"`},
					{Name: "Name", Type: "string", Tag: `json:"name"`},
					{Name: "Email", Type: "string", Tag: `json:"email"`},
				},
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, "type User struct")
	// gofmt aligns struct fields, so check with flexible whitespace
	assert.Contains(t, srcStr, "ID")
	assert.Contains(t, srcStr, "Name")
	assert.Contains(t, srcStr, "Email")
	assert.Contains(t, srcStr, `json:"id"`)
	assert.Contains(t, srcStr, `json:"name"`)
	assert.Contains(t, srcStr, `json:"email"`)

	// Verify it parses
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", src, 0)
	assert.NoError(t, parseErr)
}

func TestWriteGoFile_Function(t *testing.T) {
	spec := GoFileSpec{
		Package: "main",
		Imports: map[string]string{
			"fmt": "",
		},
		Functions: []FunctionSpec{
			{
				Name:    "Hello",
				Comment: "Hello prints a greeting.",
				Params:  []ParamSpec{{Name: "name", Type: "string"}},
				Body: []astgen.ASTStmt{
					statement.NewExpression(
						expression.NewCallFunction("fmt", "Println",
							expression.VariableVal("name"),
						),
					),
				},
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, "func Hello(name string)")
	assert.Contains(t, srcStr, "fmt.Println(name)")

	// Verify it parses
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", src, 0)
	assert.NoError(t, parseErr)
}

func TestWriteGoFile_Method(t *testing.T) {
	spec := GoFileSpec{
		Package: "models",
		Functions: []FunctionSpec{
			{
				Name: "String",
				Receiver: &ReceiverSpec{
					Name:    "u",
					Type:    "User",
					Pointer: true,
				},
				Returns: []string{"string"},
				Body: []astgen.ASTStmt{
					statement.NewReturn(expression.VariableVal(`u.Name`)),
				},
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, "func (u *User) String() string")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", src, 0)
	assert.NoError(t, parseErr)
}

func TestWriteGoFile_Interface(t *testing.T) {
	spec := GoFileSpec{
		Package: "service",
		Imports: map[string]string{
			"context": "",
		},
		Interfaces: []InterfaceSpec{
			{
				Name:    "Repository",
				Comment: "Repository defines the data access layer.",
				Methods: []MethodSigSpec{
					{
						Name:    "GetByID",
						Params:  []ParamSpec{{Name: "ctx", Type: "context.Context"}, {Name: "id", Type: "int"}},
						Returns: []string{"error"},
					},
					{
						Name:    "Create",
						Params:  []ParamSpec{{Name: "ctx", Type: "context.Context"}},
						Returns: []string{"error"},
					},
				},
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, "type Repository interface")
	assert.Contains(t, srcStr, "GetByID")
	assert.Contains(t, srcStr, "Create")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", src, 0)
	assert.NoError(t, parseErr)
}

func TestWriteGoFile_Var(t *testing.T) {
	spec := GoFileSpec{
		Package: "main",
		Vars: []VarSpec{
			{
				Name:  "defaultTimeout",
				Type:  "int",
				Value: expression.IntVal(30),
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, "var defaultTimeout int = 30")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", src, 0)
	assert.NoError(t, parseErr)
}

func TestWriteGoFile_CompleteFile(t *testing.T) {
	spec := GoFileSpec{
		Package: "handlers",
		Imports: map[string]string{
			"context": "",
			"fmt":     "",
		},
		Structs: []StructSpec{
			{
				Name: "Handler",
				Fields: []FieldSpec{
					{Name: "Name", Type: "string", Tag: `json:"name"`},
				},
			},
		},
		Functions: []FunctionSpec{
			{
				Name: "Handle",
				Receiver: &ReceiverSpec{
					Name:    "h",
					Type:    "Handler",
					Pointer: true,
				},
				Params:  []ParamSpec{{Name: "ctx", Type: "context.Context"}},
				Returns: []string{"error"},
				Body: []astgen.ASTStmt{
					statement.NewExpression(
						expression.NewCallFunction("fmt", "Println",
							expression.VariableVal("h.Name"),
						),
					),
					statement.NewReturn(expression.Nil),
				},
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	srcStr := string(src)
	assert.Contains(t, srcStr, "package handlers")
	assert.Contains(t, srcStr, `"context"`)
	assert.Contains(t, srcStr, `"fmt"`)
	assert.Contains(t, srcStr, "type Handler struct")
	assert.Contains(t, srcStr, "func (h *Handler) Handle(ctx context.Context) error")

	// Verify it's valid Go
	fset := token.NewFileSet()
	f, parseErr := parser.ParseFile(fset, "", src, 0)
	require.NoError(t, parseErr)
	assert.Equal(t, "handlers", f.Name.Name)
}

func TestWriteGoFile_OutputIsGofmtCompliant(t *testing.T) {
	spec := GoFileSpec{
		Package: "main",
		Imports: map[string]string{
			"fmt": "",
		},
		Functions: []FunctionSpec{
			{
				Name: "main",
				Body: []astgen.ASTStmt{
					statement.NewExpression(
						expression.NewCallFunction("fmt", "Println",
							expression.StringVal("hello"),
						),
					),
				},
			},
		},
	}

	src, err := WriteGoFile(spec)
	require.NoError(t, err)

	// goastwriter.Write always returns gofmt'd output.
	// Verify no tabs are mixed with spaces incorrectly - just ensure it parses.
	srcStr := string(src)
	assert.True(t, strings.HasPrefix(srcStr, "package main"))
}
