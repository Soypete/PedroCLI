// Package codegen provides AST-based Go code generation using goastwriter.
// It wraps goastwriter with Pedro-specific helpers for generating structurally
// correct, gofmt-compliant Go source files.
package codegen

import (
	"github.com/palantir/goastwriter"
	"github.com/palantir/goastwriter/astgen"
	"github.com/palantir/goastwriter/decl"
	"github.com/palantir/goastwriter/expression"
)

// GoFileSpec describes a complete Go source file to be generated.
type GoFileSpec struct {
	Package    string
	Imports    map[string]string // path -> alias ("" for no alias)
	Structs    []StructSpec
	Interfaces []InterfaceSpec
	Functions  []FunctionSpec
	Vars       []VarSpec
	Consts     []ConstSpec
}

// StructSpec describes a struct declaration.
type StructSpec struct {
	Name    string
	Comment string
	Fields  []FieldSpec
}

// FieldSpec describes a struct field.
type FieldSpec struct {
	Name    string
	Type    string
	Tag     string
	Comment string
}

// InterfaceSpec describes an interface declaration.
type InterfaceSpec struct {
	Name    string
	Comment string
	Methods []MethodSigSpec
}

// MethodSigSpec describes a method signature in an interface.
type MethodSigSpec struct {
	Name    string
	Params  []ParamSpec
	Returns []string
}

// FunctionSpec describes a function or method.
type FunctionSpec struct {
	Name     string
	Comment  string
	Receiver *ReceiverSpec // nil for standalone functions
	Params   []ParamSpec
	Returns  []string          // return type strings
	Body     []astgen.ASTStmt  // goastwriter statement nodes
}

// ReceiverSpec describes a method receiver.
type ReceiverSpec struct {
	Name    string
	Type    string
	Pointer bool
}

// ParamSpec describes a function parameter.
type ParamSpec struct {
	Name string
	Type string
}

// VarSpec describes a variable declaration.
type VarSpec struct {
	Name  string
	Type  string
	Value astgen.ASTExpr // nil for zero-value declarations
}

// ConstSpec describes a constant declaration.
type ConstSpec struct {
	Name  string
	Type  string
	Value astgen.ASTExpr
}

// WriteGoFile generates a complete Go source file from a GoFileSpec.
// Output is always gofmt-compliant.
func WriteGoFile(spec GoFileSpec) ([]byte, error) {
	var parts []astgen.ASTDecl

	// Imports
	if len(spec.Imports) > 0 {
		parts = append(parts, decl.NewImports(spec.Imports))
	}

	// Structs
	for _, s := range spec.Structs {
		fields := make(expression.StructFields, len(s.Fields))
		for i, f := range s.Fields {
			fields[i] = &expression.StructField{
				Name:    f.Name,
				Type:    expression.Type(f.Type),
				Tag:     f.Tag,
				Comment: f.Comment,
			}
		}
		parts = append(parts, decl.NewStruct(s.Name, fields, s.Comment))
	}

	// Interfaces
	for _, iface := range spec.Interfaces {
		parts = append(parts, buildInterface(iface))
	}

	// Vars
	for _, v := range spec.Vars {
		varDecl := decl.NewVar(v.Name, expression.Type(v.Type))
		if v.Value != nil {
			varDecl.Value = v.Value
		}
		parts = append(parts, varDecl)
	}

	// Functions and methods
	for _, fn := range spec.Functions {
		parts = append(parts, buildFunction(fn))
	}

	return goastwriter.Write(spec.Package, parts...)
}

// buildFunction creates a goastwriter Function or Method from a FunctionSpec.
func buildFunction(fn FunctionSpec) astgen.ASTDecl {
	params := make(expression.FuncParams, len(fn.Params))
	for i, p := range fn.Params {
		params[i] = expression.NewFuncParam(p.Name, expression.Type(p.Type))
	}

	retTypes := make(expression.Types, len(fn.Returns))
	for i, r := range fn.Returns {
		retTypes[i] = expression.Type(r)
	}

	f := &decl.Function{
		Name: fn.Name,
		FuncType: expression.FuncType{
			Params:      params,
			ReturnTypes: retTypes,
		},
		Body:    fn.Body,
		Comment: fn.Comment,
	}

	if fn.Receiver != nil {
		recvType := expression.Type(fn.Receiver.Type)
		if fn.Receiver.Pointer {
			recvType = recvType.Pointer()
		}
		return &decl.Method{
			Function:     *f,
			ReceiverName: fn.Receiver.Name,
			ReceiverType: recvType,
		}
	}

	return f
}

// buildInterface creates a goastwriter Interface from an InterfaceSpec.
func buildInterface(iface InterfaceSpec) astgen.ASTDecl {
	var funcs []*expression.InterfaceFunctionDecl
	for _, m := range iface.Methods {
		params := make(expression.FuncParams, len(m.Params))
		for i, p := range m.Params {
			params[i] = expression.NewFuncParam(p.Name, expression.Type(p.Type))
		}

		retTypes := make(expression.Types, len(m.Returns))
		for i, r := range m.Returns {
			retTypes[i] = expression.Type(r)
		}

		funcs = append(funcs, &expression.InterfaceFunctionDecl{
			Name:        m.Name,
			Params:      params,
			ReturnTypes: retTypes,
		})
	}

	return &decl.Interface{
		Name: iface.Name,
		InterfaceType: expression.InterfaceType{
			Functions: funcs,
		},
		Comment: iface.Comment,
	}
}
