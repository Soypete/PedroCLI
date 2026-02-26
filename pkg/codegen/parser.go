package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// ParseGoFile reads a Go source file and returns its AST.
func ParseGoFile(path string) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading file: %w", err)
	}

	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing file: %w", err)
	}

	return f, fset, nil
}

// ParseGoSource parses Go source code from a byte slice and returns its AST.
func ParseGoSource(src []byte) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing source: %w", err)
	}

	return f, fset, nil
}

// ExtractImports returns the current imports from a parsed Go file.
func ExtractImports(f *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, imp := range f.Imports {
		path := imp.Path.Value[1 : len(imp.Path.Value)-1] // strip quotes
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		imports[path] = alias
	}
	return imports
}

// FindFunction finds a function declaration by name in a parsed file.
func FindFunction(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// FindStruct finds a struct type declaration by name.
func FindStruct(f *ast.File, name string) *ast.TypeSpec {
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name.Name == name {
				if _, isStruct := ts.Type.(*ast.StructType); isStruct {
					return ts
				}
			}
		}
	}
	return nil
}

// FindInterface finds an interface type declaration by name.
func FindInterface(f *ast.File, name string) *ast.TypeSpec {
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name.Name == name {
				if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
					return ts
				}
			}
		}
	}
	return nil
}

// DeclInfo holds information about a top-level declaration.
type DeclInfo struct {
	Name string
	Kind string // "func", "method", "type", "var", "const"
}

// ListDeclarations returns all top-level declaration names and kinds.
func ListDeclarations(f *ast.File) []DeclInfo {
	var decls []DeclInfo
	for _, d := range f.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			info := DeclInfo{Name: dd.Name.Name, Kind: "func"}
			if dd.Recv != nil && len(dd.Recv.List) > 0 {
				info.Kind = "method"
			}
			decls = append(decls, info)
		case *ast.GenDecl:
			for _, spec := range dd.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					decls = append(decls, DeclInfo{Name: s.Name.Name, Kind: "type"})
				case *ast.ValueSpec:
					kind := "var"
					if dd.Tok == token.CONST {
						kind = "const"
					}
					for _, name := range s.Names {
						decls = append(decls, DeclInfo{Name: name.Name, Kind: kind})
					}
				}
			}
		}
	}
	return decls
}

// PackageName returns the package name from a parsed Go file.
func PackageName(f *ast.File) string {
	return f.Name.Name
}
