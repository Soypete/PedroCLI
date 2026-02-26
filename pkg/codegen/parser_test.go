package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoFile(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	src := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}
`
	require.NoError(t, os.WriteFile(path, []byte(src), 0644))

	f, fset, err := ParseGoFile(path)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.NotNil(t, fset)
	assert.Equal(t, "main", f.Name.Name)
}

func TestParseGoFile_NotFound(t *testing.T) {
	_, _, err := ParseGoFile("/nonexistent/file.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading file")
}

func TestParseGoFile_InvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	require.NoError(t, os.WriteFile(path, []byte("not valid go"), 0644))

	_, _, err := ParseGoFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing file")
}

func TestParseGoSource(t *testing.T) {
	src := []byte(`package main

func main() {}
`)
	f, fset, err := ParseGoSource(src)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.NotNil(t, fset)
	assert.Equal(t, "main", f.Name.Name)
}

func TestExtractImports(t *testing.T) {
	src := []byte(`package main

import (
	"fmt"
	"os"
	mylib "github.com/example/lib"
)

func main() {}
`)
	f, _, err := ParseGoSource(src)
	require.NoError(t, err)

	imports := ExtractImports(f)
	assert.Equal(t, "", imports["fmt"])
	assert.Equal(t, "", imports["os"])
	assert.Equal(t, "mylib", imports["github.com/example/lib"])
}

func TestFindFunction(t *testing.T) {
	src := []byte(`package main

func hello() {}
func goodbye() {}
`)
	f, _, err := ParseGoSource(src)
	require.NoError(t, err)

	fn := FindFunction(f, "hello")
	require.NotNil(t, fn)
	assert.Equal(t, "hello", fn.Name.Name)

	fn = FindFunction(f, "goodbye")
	require.NotNil(t, fn)

	fn = FindFunction(f, "missing")
	assert.Nil(t, fn)
}

func TestFindStruct(t *testing.T) {
	src := []byte(`package models

type User struct {
	Name string
}

type Config struct {
	Path string
}
`)
	f, _, err := ParseGoSource(src)
	require.NoError(t, err)

	ts := FindStruct(f, "User")
	require.NotNil(t, ts)
	assert.Equal(t, "User", ts.Name.Name)

	ts = FindStruct(f, "Config")
	require.NotNil(t, ts)

	ts = FindStruct(f, "Missing")
	assert.Nil(t, ts)
}

func TestFindInterface(t *testing.T) {
	src := []byte(`package service

type Reader interface {
	Read() error
}
`)
	f, _, err := ParseGoSource(src)
	require.NoError(t, err)

	ts := FindInterface(f, "Reader")
	require.NotNil(t, ts)
	assert.Equal(t, "Reader", ts.Name.Name)

	ts = FindInterface(f, "Missing")
	assert.Nil(t, ts)
}

func TestListDeclarations(t *testing.T) {
	src := []byte(`package main

import "fmt"

const MaxItems = 100

var counter int

type Config struct {
	Name string
}

func hello() {
	fmt.Println("hello")
}
`)
	f, _, err := ParseGoSource(src)
	require.NoError(t, err)

	decls := ListDeclarations(f)
	require.Len(t, decls, 4)

	names := map[string]string{}
	for _, d := range decls {
		names[d.Name] = d.Kind
	}
	assert.Equal(t, "const", names["MaxItems"])
	assert.Equal(t, "var", names["counter"])
	assert.Equal(t, "type", names["Config"])
	assert.Equal(t, "func", names["hello"])
}

func TestPackageName(t *testing.T) {
	src := []byte(`package mypackage

func hello() {}
`)
	f, _, err := ParseGoSource(src)
	require.NoError(t, err)
	assert.Equal(t, "mypackage", PackageName(f))
}
