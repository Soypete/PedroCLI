package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ID", "i_d"},
		{"Name", "name"},
		{"UserName", "user_name"},
		{"HTTPServer", "h_t_t_p_server"},
		{"simple", "simple"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, toSnakeCase(tt.input))
		})
	}
}

func TestStructFieldWithJSON(t *testing.T) {
	f := StructFieldWithJSON("UserName", "string")
	assert.Equal(t, "UserName", f.Name)
	assert.Equal(t, "string", f.Type)
	assert.Equal(t, `json:"user_name"`, f.Tag)
}

func TestStructFieldWithTags(t *testing.T) {
	f := StructFieldWithTags("Name", "string", `json:"name" db:"user_name"`)
	assert.Equal(t, "Name", f.Name)
	assert.Equal(t, "string", f.Type)
	assert.Equal(t, `json:"name" db:"user_name"`, f.Tag)
}

func TestContextParam(t *testing.T) {
	p := ContextParam()
	assert.Equal(t, "ctx", p.Name)
	assert.Equal(t, "context.Context", p.Type)
}

func TestErrorReturnType(t *testing.T) {
	ret := ErrorReturnType()
	assert.Equal(t, []string{"error"}, ret)
}
