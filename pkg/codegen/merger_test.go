package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeIntoFile_AppendDecl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	// Write initial file
	initial := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	// Append a new function
	newFunc := `package main

func goodbye() {
	fmt.Println("goodbye")
}
`
	err := MergeIntoFile(MergeRequest{
		FilePath: path,
		Strategy: AppendDecl,
		NewCode:  []byte(newFunc),
	})
	require.NoError(t, err)

	// Read back and verify
	result, err := os.ReadFile(path)
	require.NoError(t, err)
	resultStr := string(result)

	assert.Contains(t, resultStr, "func hello()")
	assert.Contains(t, resultStr, "func goodbye()")

	// Verify it's valid Go
	_, _, parseErr := ParseGoSource(result)
	assert.NoError(t, parseErr)
}

func TestMergeIntoFile_ReplaceDecl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	initial := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}

func world() {
	fmt.Println("world")
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	// Replace hello function
	replacement := `package main

func hello() {
	fmt.Println("replaced!")
}
`
	err := MergeIntoFile(MergeRequest{
		FilePath:   path,
		Strategy:   ReplaceDecl,
		TargetName: "hello",
		NewCode:    []byte(replacement),
	})
	require.NoError(t, err)

	result, err := os.ReadFile(path)
	require.NoError(t, err)
	resultStr := string(result)

	assert.Contains(t, resultStr, `fmt.Println("replaced!")`)
	assert.Contains(t, resultStr, "func world()")
	assert.NotContains(t, resultStr, `fmt.Println("hello")`)

	_, _, parseErr := ParseGoSource(result)
	assert.NoError(t, parseErr)
}

func TestMergeIntoFile_ReplaceDecl_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	initial := `package main

func hello() {}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	err := MergeIntoFile(MergeRequest{
		FilePath:   path,
		Strategy:   ReplaceDecl,
		TargetName: "missing",
		NewCode:    []byte("package main\nfunc missing() {}"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `"missing" not found`)
}

func TestMergeIntoFile_AddImports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	initial := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	// Append function with new imports
	newFunc := `package main

func doIO() {
	os.Exit(0)
}
`
	err := MergeIntoFile(MergeRequest{
		FilePath:   path,
		Strategy:   AppendDecl,
		NewImports: map[string]string{"os": ""},
		NewCode:    []byte(newFunc),
	})
	require.NoError(t, err)

	result, err := os.ReadFile(path)
	require.NoError(t, err)
	resultStr := string(result)

	assert.Contains(t, resultStr, `"fmt"`)
	assert.Contains(t, resultStr, `"os"`)
	assert.Contains(t, resultStr, "func doIO()")
}

func TestMergeIntoFile_AddFieldToStruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	initial := `package models

type User struct {
	Name string
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	// Add fields to the struct
	newFields := `package models

type User struct {
	Email string
	Age   int
}
`
	err := MergeIntoFile(MergeRequest{
		FilePath:   path,
		Strategy:   AddFieldToStruct,
		TargetName: "User",
		NewCode:    []byte(newFields),
	})
	require.NoError(t, err)

	result, err := os.ReadFile(path)
	require.NoError(t, err)
	resultStr := string(result)

	// gofmt aligns struct fields, so check field presence flexibly
	assert.Contains(t, resultStr, "Name")
	assert.Contains(t, resultStr, "Email")
	assert.Contains(t, resultStr, "Age")

	_, _, parseErr := ParseGoSource(result)
	assert.NoError(t, parseErr)
}

func TestMergeIntoFile_AddMethodToType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	initial := `package models

type User struct {
	Name string
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	// Add a method
	newMethod := `package models

func (u *User) FullName() string {
	return u.Name
}
`
	err := MergeIntoFile(MergeRequest{
		FilePath: path,
		Strategy: AddMethodToType,
		NewCode:  []byte(newMethod),
	})
	require.NoError(t, err)

	result, err := os.ReadFile(path)
	require.NoError(t, err)
	resultStr := string(result)

	assert.Contains(t, resultStr, "type User struct")
	assert.Contains(t, resultStr, "func (u *User) FullName() string")

	_, _, parseErr := ParseGoSource(result)
	assert.NoError(t, parseErr)
}

func TestMergeIntoFile_NoPackageDecl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	initial := `package main

func hello() {}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	// New code without package declaration - should be auto-wrapped
	newFunc := `func world() {}
`
	err := MergeIntoFile(MergeRequest{
		FilePath: path,
		Strategy: AppendDecl,
		NewCode:  []byte(newFunc),
	})
	require.NoError(t, err)

	result, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(result), "func hello()")
	assert.Contains(t, string(result), "func world()")
}

func TestHasPackageDecl(t *testing.T) {
	assert.True(t, hasPackageDecl([]byte("package main\n\nfunc hello() {}")))
	assert.True(t, hasPackageDecl([]byte("  package main\n")))
	assert.False(t, hasPackageDecl([]byte("func hello() {}")))
	assert.False(t, hasPackageDecl([]byte("// comment\nfunc hello() {}")))
}
