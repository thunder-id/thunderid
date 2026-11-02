// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every table that carries a DEPLOYMENT_ID column holds the rows of every deployment the server
// serves, so a query on one must say which deployment's rows it means. This test reads every DBQuery
// in the backend and fails on one that touches such a table without a DEPLOYMENT_ID predicate for
// it, or inserts into one without the column. It finds the queries by walking the sources, so a new
// one is checked without being registered anywhere.
//
// A query whose text is put together at run time is checked through the parts that are string
// literals and constants. One that cannot be read far enough to show a predicate must be listed in
// unscopedQueries with the reason it is safe; a new one is reported here until it is.
func TestEveryQueryOnADeploymentTableNamesTheDeployment(t *testing.T) {
	scoped := scopedTables(t)
	var failures []string
	for _, q := range backendQueries(t) {
		if reason, listed := unscopedQueries[q.id]; listed {
			if reason == "" {
				failures = append(failures, q.where+": "+q.id+" is listed without a reason")
			}
			continue
		}
		reported := map[string]bool{}
		for _, text := range q.texts {
			problems := unscopedTables(text.sql, text.complete, scoped)
			if !text.complete && (text.sql == "" || len(problems) > 0) {
				problems = append(problems, "cannot be read from the source far enough to check; "+
					"build it from a constant, or list it in unscopedQueries with why it is safe")
			}
			for _, problem := range problems {
				if !reported[problem] {
					reported[problem] = true
					failures = append(failures, q.where+": "+q.id+": "+problem)
				}
			}
		}
	}
	sort.Strings(failures)
	if len(failures) > 0 {
		t.Errorf("%d queries on a deployment table do not name the deployment:\n  %s",
			len(failures), strings.Join(failures, "\n  "))
	}
}

// unscopedQueries are the queries allowed to touch a deployment table without naming the deployment,
// each with why that is safe.
var unscopedQueries = map[string]string{
	"ASQ-ENTITY_MGT-20":  "the predicate is appended by utils.AppendDeploymentIDToFilterQuery after the literal",
	"ASQ-ENTITY_MGT-21":  "the predicate is appended by utils.AppendDeploymentIDToFilterQuery after the literal",
	"CNQ-CONSENT_MGT-07": "C.DEPLOYMENT_ID = $1 is the first of the conditions the builder joins into its WHERE",
	"buildFilterQueryWithOffset": "extends a base query its callers pass in and scope with " +
		"utils.AppendDeploymentIDToFilterQuery; the identifier subquery is scoped through the entity's own id",
	"SHQ-SHARING_MGT-19": "built by a closure whose text ends in p.DEPLOYMENT_ID = <parameter>",
}

// backendRoot is the backend module, from this package's directory.
const backendRoot = "../../../.."

// scopedTables reads the schemas and returns every table with a DEPLOYMENT_ID column.
func scopedTables(t *testing.T) map[string]bool {
	t.Helper()
	tables := map[string]bool{}
	schemas, err := filepath.Glob(filepath.Join(backendRoot, "dbscripts", "*", "postgres.sql"))
	if err != nil || len(schemas) == 0 {
		t.Fatalf("no schemas found under %s: %v", backendRoot, err)
	}
	table := regexp.MustCompile(`(?s)CREATE TABLE "([A-Z_]+)"\s*\((.*?)\n\);`)
	for _, schema := range schemas {
		sql, err := os.ReadFile(schema) // #nosec G304 -- a schema of this repository, found by the glob above
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range table.FindAllStringSubmatch(string(sql), -1) {
			if strings.Contains(m[2], "DEPLOYMENT_ID") {
				tables[m[1]] = true
			}
		}
	}
	return tables
}

// queryText is one text of a DBQuery as far as the source lets it be read, and whether it was read
// whole.
type queryText struct {
	sql      string
	complete bool
}

// query is one DBQuery literal found in the sources, with each of its texts.
type query struct {
	id    string
	where string
	texts []queryText
}

// backendQueries finds every DBQuery literal under the backend's internal packages.
func backendQueries(t *testing.T) []query {
	t.Helper()
	var queries []query
	root := filepath.Join(backendRoot, "internal")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		// Names are resolved within the function that uses them, by the assignments made before the
		// query, over what the file declares at package level.
		packageNames := fileStrings{}
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				packageNames.collect(gen)
			}
		}
		for _, decl := range file.Decls {
			names, fallback := packageNames, "(unnamed)"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				names, fallback = packageNames.within(fn), fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || !isDBQuery(lit.Type) {
					return true
				}
				queries = append(queries, names.queryOf(lit, fset.Position(lit.Pos()).String(), fallback))
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) == 0 {
		t.Fatalf("no queries found under %s", root)
	}
	return queries
}

// isDBQuery reports whether a composite literal's type is DBQuery, from this package or imported
// under any name.
func isDBQuery(expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.Ident:
		return typ.Name == "DBQuery"
	case *ast.SelectorExpr:
		return typ.Sel.Name == "DBQuery"
	}
	return false
}

// binding is one value a name was given, and where.
type binding struct {
	at    token.Pos
	value ast.Expr
}

// fileStrings are the values the names in scope were given, in source order, so a query is read with
// the assignments made before it rather than any made after.
type fileStrings map[string][]binding

// collect adds the names a declaration or assignment gives a value.
func (s fileStrings) collect(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if i < len(node.Values) {
					s[name.Name] = append(s[name.Name], binding{at: node.Pos(), value: node.Values[i]})
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				name, ok := lhs.(*ast.Ident)
				if !ok || i >= len(node.Rhs) {
					continue
				}
				value := node.Rhs[i]
				if node.Tok == token.ADD_ASSIGN {
					// "name += x" keeps what the name held before, read as of this assignment.
					held := &ast.Ident{NamePos: node.Pos(), Name: name.Name}
					value = &ast.BinaryExpr{X: held, Op: token.ADD, Y: value}
				}
				s[name.Name] = append(s[name.Name], binding{at: node.Pos(), value: value})
			}
		}
		return true
	})
}

// within returns the names in scope inside a function: the package's and the function's own.
func (s fileStrings) within(fn *ast.FuncDecl) fileStrings {
	scope := fileStrings{}
	for name, bindings := range s {
		scope[name] = append([]binding(nil), bindings...)
	}
	if fn.Body != nil {
		scope.collect(fn.Body)
	}
	return scope
}

// valueBefore is the value a name held at a position: the last it was given before it.
func (s fileStrings) valueBefore(name string, at token.Pos) (ast.Expr, bool) {
	var found ast.Expr
	for _, b := range s[name] {
		if b.at < at {
			found = b.value
		}
	}
	return found, found != nil
}

// queryOf reads a DBQuery literal's id and texts. A query whose id cannot be read, as one a builder
// takes as an argument, is named after the function that builds it.
func (s fileStrings) queryOf(lit *ast.CompositeLit, where, fallback string) query {
	q := query{id: fallback, where: where}
	for _, element := range lit.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "ID":
			if id, _ := s.resolve(kv.Value, lit.Pos(), 0); id != "" {
				q.id = id
			}
		case "Query", "PostgresQuery", "SQLiteQuery":
			sql, complete := s.resolve(kv.Value, lit.Pos(), 0)
			q.texts = append(q.texts, queryText{sql: sql, complete: complete})
		}
	}
	return q
}

// resolve reads the string an expression evaluates to, as far as literals, concatenation, the names
// in scope and the string arguments of calls allow, and reports whether it was read whole. What
// cannot be read is left out, so a predicate present in the readable parts is still seen.
func (s fileStrings) resolve(expr ast.Expr, at token.Pos, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if value, err := strconv.Unquote(e.Value); err == nil {
				return value, true
			}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			x, xc := s.resolve(e.X, at, depth+1)
			y, yc := s.resolve(e.Y, at, depth+1)
			return x + y, xc && yc
		}
	case *ast.ParenExpr:
		return s.resolve(e.X, at, depth+1)
	case *ast.Ident:
		// A name stands for what it held where it is read: inside the query, or at an earlier
		// assignment that built on it.
		if e.NamePos != token.NoPos && e.NamePos < at {
			at = e.NamePos
		}
		if value, ok := s.valueBefore(e.Name, at); ok {
			return s.resolve(value, at, depth+1)
		}
	case *ast.CallExpr:
		var parts []string
		for _, arg := range e.Args {
			part, _ := s.resolve(arg, at, depth+1)
			parts = append(parts, part)
		}
		return strings.Join(parts, " "), false
	}
	return "", false
}

// tableReference finds each table a statement reads or writes, with the alias it gives it.
var tableReference = regexp.MustCompile(
	`(?i)\b(FROM|JOIN|UPDATE|INTO)\s+"([A-Z_]+)"(?:\s+(?:AS\s+)?([A-Za-z_][A-Za-z0-9_]*))?`)

// parameter is a query parameter as the dialects write it, as a format verb a builder fills in with
// one, or, in a text put together at run time, the gap an unreadable placeholder leaves before the
// next clause.
const parameter = `(?:\$\d+|\?|\$%d|%s|%v|\$?\s*(?:AND\b|OR\b|ORDER\b|GROUP\b|LIMIT\b|\)|$))`

// parameterBound finds a DEPLOYMENT_ID, qualified or not, compared with a parameter: the predicate
// that scopes a table to the request's deployment.
var parameterBound = regexp.MustCompile(
	`(?i)(?:("?[A-Za-z_][A-Za-z0-9_]*"?)\.)?\bDEPLOYMENT_ID\s*=\s*` + parameter + `|` +
		`(?:\$\d+|\?)\s*=\s*(?:("?[A-Za-z_][A-Za-z0-9_]*"?)\.)?\bDEPLOYMENT_ID\b`)

// equatedDeployments finds two tables' DEPLOYMENT_ID columns compared with each other, as a join
// does, which carries one table's scope to the other.
var equatedDeployments = regexp.MustCompile(
	`(?i)("?[A-Za-z_][A-Za-z0-9_]*"?)\.DEPLOYMENT_ID\s*=\s*("?[A-Za-z_][A-Za-z0-9_]*"?)\.DEPLOYMENT_ID`)

// commonTable finds the name a statement gives a common table expression.
var commonTable = regexp.MustCompile(`(?i)\bWITH\s+(?:RECURSIVE\s+)?([A-Za-z_][A-Za-z0-9_]*)\s+AS\b`)

// unquotedSource finds a source named without quotes, a common table expression, with its alias.
var unquotedSource = regexp.MustCompile(
	`(?i)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_]*)\s+(?:AS\s+)?([A-Za-z_][A-Za-z0-9_]*)`)

// setOperator finds a statement made of several selects.
var setOperator = regexp.MustCompile(`(?i)\b(UNION|INTERSECT|EXCEPT)\b`)

// matchesNothing finds a statement written to return no row, as a query answering an empty list does.
var matchesNothing = regexp.MustCompile(`(?i)\bWHERE\s+1\s*=\s*0\b`)

// insertColumns finds the column list of an insert.
var insertColumns = regexp.MustCompile(`(?is)INSERT\s+INTO\s+"([A-Z_]+)"\s*\(([^)]*)\)`)

// sqlKeywords are the words that may follow a table name and are not an alias.
var sqlKeywords = map[string]bool{
	"WHERE": true, "SET": true, "ON": true, "LEFT": true, "RIGHT": true, "INNER": true, "OUTER": true,
	"FULL": true, "CROSS": true, "JOIN": true, "USING": true, "VALUES": true, "GROUP": true,
	"ORDER": true, "LIMIT": true, "OFFSET": true, "RETURNING": true, "SELECT": true, "UNION": true,
	"AND": true, "OR": true, "NOT": true, "EXISTS": true, "DEFAULT": true, "FOR": true, "HAVING": true,
	"WITH": true, "AS": true, "DO": true, "NATURAL": true, "IS": true, "IN": true,
}

// unscopedTables reports each scoped table the statement touches without scoping it to the request's
// deployment: a read or write whose DEPLOYMENT_ID, by its alias when it has one, is neither compared
// with a parameter nor equated with a table's that is, or an insert without the column. Each alias
// is checked on its own, so a self-join scopes both sides. complete says whether the text was read
// whole from the source.
func unscopedTables(text string, complete bool, scoped map[string]bool) []string {
	// A statement written to return no row reads no deployment's rows, as long as it is one select
	// and not one branch of several.
	if matchesNothing.MatchString(text) && !setOperator.MatchString(text) {
		return nil
	}
	bound := boundQualifiers(text)
	var problems []string
	seen := map[string]bool{}
	for _, m := range tableReference.FindAllStringSubmatch(text, -1) {
		table, alias := m[2], m[3]
		if sqlKeywords[strings.ToUpper(alias)] {
			alias = ""
		}
		if !scoped[table] || seen[table+" "+alias] {
			continue
		}
		seen[table+" "+alias] = true
		if strings.EqualFold(m[1], "INTO") {
			continue
		}
		byName := bound[strings.ToLower(`"`+table+`"`)]
		// With no alias, a bare predicate scopes it. Only in a text that could not be read whole may
		// the alias be missing because it is put together at run time; there any bound qualifier
		// counts, since the predicate on it cannot be told from one on the table.
		scopedHere := byName || (alias == "" && (bound[""] || (!complete && len(bound) > 0))) ||
			(alias != "" && bound[strings.ToLower(alias)])
		if !scopedHere {
			named := table
			if alias != "" {
				named += " as " + alias
			}
			problems = append(problems,
				named+" is read or written without a DEPLOYMENT_ID predicate bound to a parameter")
		}
	}
	for _, m := range insertColumns.FindAllStringSubmatch(text, -1) {
		if scoped[m[1]] && !strings.Contains(strings.ToUpper(m[2]), "DEPLOYMENT_ID") {
			problems = append(problems, m[1]+" is inserted into without DEPLOYMENT_ID")
		}
	}
	return problems
}

// boundQualifiers reports which qualifiers, aliases or quoted table names in lower case, "" for none,
// have their DEPLOYMENT_ID compared with a parameter, directly or through another table's that is.
func boundQualifiers(text string) map[string]bool {
	bound := map[string]bool{}
	for _, m := range parameterBound.FindAllStringSubmatch(text, -1) {
		bound[strings.ToLower(m[1]+m[2])] = true
	}
	// An unqualified predicate scopes the tables named without an alias, so a qualifier spelled as
	// a quoted table name, and a common table expression whose anchor carries it, are scoped by it.
	if bound[""] {
		for _, m := range equatedDeployments.FindAllStringSubmatch(text, -1) {
			for _, qualifier := range m[1:] {
				if strings.HasPrefix(qualifier, `"`) {
					bound[strings.ToLower(qualifier)] = true
				}
			}
		}
		for _, m := range commonTable.FindAllStringSubmatch(text, -1) {
			bound[strings.ToLower(m[1])] = true
		}
	}
	// An alias of a common table expression is scoped as the expression is.
	for _, m := range unquotedSource.FindAllStringSubmatch(text, -1) {
		if bound[strings.ToLower(m[1])] {
			bound[strings.ToLower(m[2])] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, m := range equatedDeployments.FindAllStringSubmatch(text, -1) {
			left, right := strings.ToLower(m[1]), strings.ToLower(m[2])
			if bound[left] != bound[right] {
				bound[left], bound[right] = true, true
				changed = true
			}
		}
	}
	return bound
}
