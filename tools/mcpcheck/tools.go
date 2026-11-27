// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

const mcpImportPath = "github.com/modelcontextprotocol/go-sdk/mcp"

// loadTools returns the names of the tools registered as mcp.Tool literals in the tools.go and
// tool.go files under dir, each with the file that registers it.
func loadTools(dir string) (map[string]string, []problem, error) {
	tools := map[string]string{}
	var problems []problem
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || (e.Name() != "tools.go" && e.Name() != "tool.go") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		names, unreadable, err := parseTools(path)
		if err != nil {
			return err
		}
		for _, n := range names {
			tools[n] = path
		}
		for _, expr := range unreadable {
			problems = append(problems, problem{
				key: fmt.Sprintf("%s unreadable tool name %s", filepath.ToSlash(rel), expr),
				msg: fmt.Sprintf("%s: mcp.Tool Name %s is not a string literal; write the name as a literal "+
					"so the exposure check can read it", path, expr),
			})
		}
		return nil
	})
	return tools, problems, err
}

// parseTools returns the Name of every mcp.Tool literal in the file, and the source of any Name it
// cannot resolve to a string, so a tool is never registered unseen.
func parseTools(path string) (names, unreadable []string, err error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, nil, err
	}
	pkg := ""
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == mcpImportPath {
			pkg = "mcp"
			if imp.Name != nil {
				pkg = imp.Name.Name
			}
		}
	}
	if pkg == "" {
		return nil, nil, nil
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isSelector(lit.Type, pkg, "Tool") {
			return true
		}
		for _, e := range lit.Elts {
			kv, ok := e.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "Name" {
				continue
			}
			if s, ok := stringLit(kv.Value); ok {
				names = append(names, s)
			} else {
				unreadable = append(unreadable, types.ExprString(kv.Value))
			}
		}
		return true
	})
	return names, unreadable, nil
}

func isSelector(e ast.Expr, pkg, name string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == pkg && s.Sel.Name == name
}

// stringLit resolves a string literal, or a concatenation of string literals.
func stringLit(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		x, okX := stringLit(v.X)
		y, okY := stringLit(v.Y)
		return x + y, okX && okY && v.Op == token.ADD
	case *ast.ParenExpr:
		return stringLit(v.X)
	}
	return "", false
}

// unprefixed is the name an x-thunderid-mcp tools entry uses for a registered tool.
func unprefixed(name string) string {
	return strings.TrimPrefix(name, toolPrefix)
}
