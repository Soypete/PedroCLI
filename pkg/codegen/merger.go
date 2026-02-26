package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

// MergeStrategy controls how new code is merged into existing files.
type MergeStrategy int

const (
	// AppendDecl adds the declaration to the end of the file.
	AppendDecl MergeStrategy = iota
	// ReplaceDecl replaces an existing declaration with the same name.
	ReplaceDecl
	// AddFieldToStruct adds fields to an existing struct.
	AddFieldToStruct
	// AddMethodToType adds a method for a given receiver type.
	AddMethodToType
)

// MergeRequest describes what to merge into an existing file.
type MergeRequest struct {
	FilePath   string
	Strategy   MergeStrategy
	TargetName string            // name of the declaration to modify
	NewImports map[string]string // additional imports needed
	NewCode    []byte            // Go source code to merge (must be valid Go within a package)
}

// MergeIntoFile reads an existing Go file, merges new declarations,
// and writes the result back. Uses go/ast for the merge and
// go/format to ensure gofmt compliance.
func MergeIntoFile(req MergeRequest) error {
	// Parse existing file
	fset := token.NewFileSet()
	existing, err := parser.ParseFile(fset, req.FilePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing existing file: %w", err)
	}

	// Parse the new code to get its AST.
	// Wrap in a package declaration if it doesn't have one.
	newCode := req.NewCode
	if !hasPackageDecl(newCode) {
		newCode = append([]byte(fmt.Sprintf("package %s\n", existing.Name.Name)), newCode...)
	}

	newFset := token.NewFileSet()
	newFile, err := parser.ParseFile(newFset, "", newCode, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing new code: %w", err)
	}

	// Merge imports
	if len(req.NewImports) > 0 {
		mergeImports(existing, req.NewImports)
	}
	// Also merge any imports from the new code
	newImports := ExtractImports(newFile)
	if len(newImports) > 0 {
		mergeImports(existing, newImports)
	}

	// Apply merge strategy
	switch req.Strategy {
	case AppendDecl:
		for _, d := range newFile.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
				continue // skip import decls, handled separately
			}
			existing.Decls = append(existing.Decls, d)
		}

	case ReplaceDecl:
		replaced := false
		for i, d := range existing.Decls {
			if matchesDecl(d, req.TargetName) {
				// Find the replacement in new code
				for _, nd := range newFile.Decls {
					if matchesDecl(nd, req.TargetName) {
						existing.Decls[i] = nd
						replaced = true
						break
					}
				}
				break
			}
		}
		if !replaced {
			return fmt.Errorf("declaration %q not found in %s", req.TargetName, req.FilePath)
		}

	case AddFieldToStruct:
		if err := addFieldsToStruct(existing, newFile, req.TargetName); err != nil {
			return err
		}

	case AddMethodToType:
		for _, d := range newFile.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv != nil {
				existing.Decls = append(existing.Decls, d)
			}
		}
	}

	// Render back to source
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, existing); err != nil {
		return fmt.Errorf("printing AST: %w", err)
	}

	// Run gofmt for final formatting
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}

	return os.WriteFile(req.FilePath, formatted, 0644)
}

// hasPackageDecl checks whether source code already has a package declaration.
func hasPackageDecl(src []byte) bool {
	s := strings.TrimSpace(string(src))
	return strings.HasPrefix(s, "package ")
}

// matchesDecl checks whether a declaration matches the given name.
func matchesDecl(d ast.Decl, name string) bool {
	switch dd := d.(type) {
	case *ast.FuncDecl:
		return dd.Name.Name == name
	case *ast.GenDecl:
		for _, spec := range dd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
				return true
			}
		}
	}
	return false
}

// mergeImports adds missing imports to a Go file AST.
func mergeImports(f *ast.File, newImports map[string]string) {
	// Find or create import declaration
	var importDecl *ast.GenDecl
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			importDecl = gd
			break
		}
	}

	if importDecl == nil {
		importDecl = &ast.GenDecl{Tok: token.IMPORT, Lparen: 1}
		// Insert after package clause (before all other decls)
		f.Decls = append([]ast.Decl{importDecl}, f.Decls...)
	}

	// Check existing imports
	existingPaths := make(map[string]bool)
	for _, spec := range importDecl.Specs {
		if is, ok := spec.(*ast.ImportSpec); ok {
			path := strings.Trim(is.Path.Value, `"`)
			existingPaths[path] = true
		}
	}

	// Add missing imports
	for path, alias := range newImports {
		if existingPaths[path] {
			continue
		}
		spec := &ast.ImportSpec{
			Path: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, path)},
		}
		if alias != "" {
			spec.Name = ast.NewIdent(alias)
		}
		importDecl.Specs = append(importDecl.Specs, spec)
	}

	importDecl.Lparen = 1 // force parenthesized imports
}

// addFieldsToStruct finds a struct by name and adds new fields from newFile.
func addFieldsToStruct(existing *ast.File, newFile *ast.File, structName string) error {
	// Find the target struct in existing file
	var targetStruct *ast.StructType
	for _, d := range existing.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != structName {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				targetStruct = st
			}
		}
	}
	if targetStruct == nil {
		return fmt.Errorf("struct %q not found", structName)
	}

	// Find new fields from the new code's struct
	for _, d := range newFile.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != structName {
				continue
			}
			newStruct, ok := ts.Type.(*ast.StructType)
			if !ok || newStruct.Fields == nil {
				continue
			}
			// Add all new fields
			if targetStruct.Fields == nil {
				targetStruct.Fields = &ast.FieldList{}
			}
			targetStruct.Fields.List = append(targetStruct.Fields.List, newStruct.Fields.List...)
		}
	}

	return nil
}
