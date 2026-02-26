package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatGoSource_Valid(t *testing.T) {
	src := []byte(`package main

import "fmt"

func   hello( )  {
fmt.Println( "hello" )
}
`)

	formatted, err := FormatGoSource(src)
	require.NoError(t, err)

	fmtStr := string(formatted)
	// Should be properly formatted
	assert.Contains(t, fmtStr, "func hello()")
	assert.Contains(t, fmtStr, `fmt.Println("hello")`)

	// Verify it parses back cleanly
	_, _, parseErr := ParseGoSource(formatted)
	assert.NoError(t, parseErr)
}

func TestFormatGoSource_InvalidSyntax(t *testing.T) {
	src := []byte(`package main

func hello( {
`)

	formatted, err := FormatGoSource(src)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse error")
	// Should return original source on error
	assert.Equal(t, src, formatted)
}

func TestFormatGoSource_AlreadyFormatted(t *testing.T) {
	src := []byte(`package main

func hello() {}
`)

	formatted, err := FormatGoSource(src)
	require.NoError(t, err)
	assert.Equal(t, string(src), string(formatted))
}

func TestValidateGoSource_Valid(t *testing.T) {
	src := []byte(`package main

func hello() {}
`)
	err := ValidateGoSource(src)
	assert.NoError(t, err)
}

func TestValidateGoSource_Invalid(t *testing.T) {
	src := []byte(`not valid go at all`)
	err := ValidateGoSource(src)
	assert.Error(t, err)
}

func TestValidateGoSource_MissingPackage(t *testing.T) {
	src := []byte(`func hello() {}`)
	err := ValidateGoSource(src)
	assert.Error(t, err)
}
