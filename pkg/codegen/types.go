package codegen

import (
	"fmt"
	"go/token"
	"strings"
	"unicode"

	"github.com/palantir/goastwriter/astgen"
	"github.com/palantir/goastwriter/expression"
	"github.com/palantir/goastwriter/statement"
)

// ErrorReturn creates: return fmt.Errorf("msg: %w", err)
func ErrorReturn(msg string) astgen.ASTStmt {
	return statement.NewReturn(
		expression.NewCallFunction("fmt", "Errorf",
			expression.StringVal(msg+": %w"),
			expression.VariableVal("err"),
		),
	)
}

// NilErrorCheck creates: if err != nil { return ..., err }
func NilErrorCheck(returnVals ...astgen.ASTExpr) astgen.ASTStmt {
	vals := append(returnVals, expression.VariableVal("err"))
	return &statement.If{
		Cond: expression.NewBinary(
			expression.VariableVal("err"),
			token.NEQ,
			expression.Nil,
		),
		Body: []astgen.ASTStmt{
			statement.NewReturn(vals...),
		},
	}
}

// NilErrorReturn creates: if err != nil { return err }
// Shorthand for single-return error check.
func NilErrorReturn() astgen.ASTStmt {
	return NilErrorCheck()
}

// NilErrorReturnWrapped creates: if err != nil { return fmt.Errorf("msg: %w", err) }
func NilErrorReturnWrapped(msg string) astgen.ASTStmt {
	return &statement.If{
		Cond: expression.NewBinary(
			expression.VariableVal("err"),
			token.NEQ,
			expression.Nil,
		),
		Body: []astgen.ASTStmt{
			ErrorReturn(msg),
		},
	}
}

// ShortVarAssign creates: lhs := rhs (short variable declaration)
func ShortVarAssign(lhs string, rhs astgen.ASTExpr) astgen.ASTStmt {
	return statement.NewAssignment(
		expression.VariableVal(lhs),
		token.DEFINE,
		rhs,
	)
}

// Assign creates: lhs = rhs (assignment)
func Assign(lhs string, rhs astgen.ASTExpr) astgen.ASTStmt {
	return statement.NewAssignment(
		expression.VariableVal(lhs),
		token.ASSIGN,
		rhs,
	)
}

// StructFieldWithJSON creates a FieldSpec with a json tag derived from the field name.
func StructFieldWithJSON(name, typ string) FieldSpec {
	return FieldSpec{
		Name: name,
		Type: typ,
		Tag:  fmt.Sprintf(`json:"%s"`, toSnakeCase(name)),
	}
}

// StructFieldWithTags creates a FieldSpec with custom struct tags.
func StructFieldWithTags(name, typ, tags string) FieldSpec {
	return FieldSpec{
		Name: name,
		Type: typ,
		Tag:  tags,
	}
}

// ContextParam creates a ParamSpec for context.Context as first parameter.
func ContextParam() ParamSpec {
	return ParamSpec{Name: "ctx", Type: "context.Context"}
}

// ErrorReturn creates a single "error" return type list.
func ErrorReturnType() []string {
	return []string{"error"}
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
