// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/thunder-id/thunderid/internal/system/database/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// scimFilterSupportedLogicalOps/CompareOps gate which RFC 7644 keywords are
// currently accepted. scimFilterReservedCompareOps is the full comparison
// keyword set, used to tell "reserved but not enabled" apart from "not a
// keyword". Enabling a new operator only means flipping its entry here.
var (
	scimFilterSupportedLogicalOps = map[string]bool{
		"and": true,
	}
	scimFilterSupportedCompareOps = map[string]bool{
		"eq": true,
	}
	scimFilterReservedCompareOps = map[string]bool{
		"eq": true, "ne": true, "co": true, "sw": true, "ew": true,
		"pr": true, "gt": true, "lt": true, "ge": true, "le": true,
	}
)

// FilterAttrRules bundles the resource-specific attribute checks used when parsing
// SCIM filter expressions, so the grammar in this file stays resource-agnostic and
// reusable by any resource (users today, groups or others later).
type FilterAttrRules struct {
	IsUnsupported func(attr string) bool
	IsCore        func(attr string) bool
	Translate     func(attr string) string
}

type scimFilterNode interface {
	isSCIMFilterNode()
}

type scimFilterExpressionNode struct {
	Attr  string
	Op    string
	Value interface{}
}

func (*scimFilterExpressionNode) isSCIMFilterNode() {}

// scimFilterOperationNode: "and"/"or" use Left+Right, "not" is unary and uses Left only.
type scimFilterOperationNode struct {
	Op    string
	Left  scimFilterNode
	Right scimFilterNode
}

func (*scimFilterOperationNode) isSCIMFilterNode() {}

// ParseSCIMFilterForEq parses a SCIM filter string containing one or more
// "eq" comparisons joined by "and", with no "or", "not", grouping, or square
// brackets. Returns a native filter map suitable for a resource's list query,
// or a ServiceError with a specific detail if the expression uses any
// unsupported syntax.
func ParseSCIMFilterForEq(filterStr string, rules FilterAttrRules) (map[string]interface{}, *tidcommon.ServiceError) {
	filterStr = strings.TrimSpace(filterStr)
	if filterStr == "" {
		return nil, nil
	}
	tokens, err := tokenizeSCIMFilter(filterStr)
	if err != nil {
		return nil, newInvalidFilterSyntaxError(err.Error())
	}
	parser := &scimFilterParser{tokens: tokens, rules: rules}
	root, err := parser.expression()
	if err != nil {
		return nil, newInvalidFilterSyntaxError(err.Error())
	}
	if parser.pos != len(parser.tokens) {
		return nil, newInvalidFilterSyntaxError("unexpected content at the end of the filter expression")
	}
	filters := make(map[string]interface{})
	if err := flattenSCIMFilterTree(root, filters); err != nil {
		return nil, newInvalidFilterSyntaxError(err.Error())
	}
	return filters, nil
}

// flattenSCIMFilterTree only ever sees leaves and "and" branches today, since the
// parser rejects any other operator before building a node for it. The default
// case is a guard against silently mis-querying if support is widened without
// updating this function to build a real query for non-AND trees.
func flattenSCIMFilterTree(node scimFilterNode, out map[string]interface{}) error {
	switch n := node.(type) {
	case *scimFilterExpressionNode:
		if _, exists := out[n.Attr]; exists {
			return fmt.Errorf("attribute %q is repeated in the filter expression", n.Attr)
		}
		out[n.Attr] = n.Value
		return nil
	case *scimFilterOperationNode:
		if n.Op != "and" {
			return fmt.Errorf("%q filter expressions are not yet supported by this store", n.Op)
		}
		if err := flattenSCIMFilterTree(n.Left, out); err != nil {
			return err
		}
		return flattenSCIMFilterTree(n.Right, out)
	default:
		return fmt.Errorf("unrecognized filter expression node")
	}
}

type scimTokenKind int

const (
	scimTokWord scimTokenKind = iota
	scimTokQuoted
	scimTokGrouping
)

type scimToken struct {
	kind scimTokenKind
	text string
}

// tokenizeSCIMFilter splits filterStr into words, quoted string values, and
// grouping characters, without regular expressions.
func tokenizeSCIMFilter(filterStr string) ([]scimToken, error) {
	runes := []rune(filterStr)
	tokens := make([]scimToken, 0, len(runes)/4+1)
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			tokens = append(tokens, scimToken{kind: scimTokWord, text: string(buf)})
			buf = buf[:0]
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			flush()
		case r == '(' || r == ')' || r == '[' || r == ']':
			flush()
			tokens = append(tokens, scimToken{kind: scimTokGrouping, text: string(r)})
		case r == '"':
			flush()
			start := i
			i++
			for i < len(runes) && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < len(runes) {
					i++
				}
				i++
			}
			if i >= len(runes) {
				return nil, fmt.Errorf("unterminated quoted string in filter expression")
			}
			tokens = append(tokens, scimToken{kind: scimTokQuoted, text: string(runes[start : i+1])})
		default:
			buf = append(buf, r)
		}
	}
	flush()
	return tokens, nil
}

// scimFilterParser is a recursive-descent parser, structured like WSO2 Charon's
// FilterTreeManager: expression() is "or" (lowest precedence), term() is "and",
// factor() is "not" / parens / a leaf comparison (highest precedence).
type scimFilterParser struct {
	tokens []scimToken
	pos    int
	rules  FilterAttrRules
}

func (p *scimFilterParser) peek() (scimToken, bool) {
	if p.pos >= len(p.tokens) {
		return scimToken{}, false
	}
	return p.tokens[p.pos], true
}

func (p *scimFilterParser) next() (scimToken, bool) {
	tok, ok := p.peek()
	if ok {
		p.pos++
	}
	return tok, ok
}

func (p *scimFilterParser) peekIsWord(word string) bool {
	tok, ok := p.peek()
	return ok && tok.kind == scimTokWord && strings.EqualFold(tok.text, word)
}

var errCompoundNotSupported = fmt.Errorf(
	"compound filter expressions are only supported using 'and'; 'or', 'not', and grouping are not supported")

func (p *scimFilterParser) expression() (scimFilterNode, error) {
	left, err := p.term()
	if err != nil {
		return nil, err
	}
	for p.peekIsWord("or") {
		p.next()
		if !scimFilterSupportedLogicalOps["or"] {
			return nil, errCompoundNotSupported
		}
		right, err := p.term()
		if err != nil {
			return nil, err
		}
		left = &scimFilterOperationNode{Op: "or", Left: left, Right: right}
	}
	return left, nil
}

func (p *scimFilterParser) term() (scimFilterNode, error) {
	left, err := p.factor()
	if err != nil {
		return nil, err
	}
	for p.peekIsWord("and") {
		p.next()
		if !scimFilterSupportedLogicalOps["and"] {
			return nil, errCompoundNotSupported
		}
		right, err := p.factor()
		if err != nil {
			return nil, err
		}
		left = &scimFilterOperationNode{Op: "and", Left: left, Right: right}
	}
	return left, nil
}

func (p *scimFilterParser) factor() (scimFilterNode, error) {
	tok, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("invalid filter expression; expected format: 'attrPath eq value'")
	}
	if tok.kind == scimTokWord && strings.EqualFold(tok.text, "not") {
		p.next()
		if !scimFilterSupportedLogicalOps["not"] {
			return nil, errCompoundNotSupported
		}
		operand, err := p.factor()
		if err != nil {
			return nil, err
		}
		return &scimFilterOperationNode{Op: "not", Left: operand}, nil
	}
	if tok.kind == scimTokGrouping && tok.text == "(" {
		p.next()
		if !scimFilterSupportedLogicalOps["or"] && !scimFilterSupportedLogicalOps["not"] {
			return nil, errCompoundNotSupported
		}
		node, err := p.expression()
		if err != nil {
			return nil, err
		}
		closeTok, ok := p.next()
		if !ok || closeTok.kind != scimTokGrouping || closeTok.text != ")" {
			return nil, fmt.Errorf("unmatched '(' in filter expression")
		}
		return node, nil
	}
	if tok.kind == scimTokGrouping {
		return nil, errCompoundNotSupported
	}
	return p.leaf()
}

func (p *scimFilterParser) leaf() (scimFilterNode, error) {
	attrTok, ok := p.next()
	if !ok || attrTok.kind != scimTokWord {
		return nil, fmt.Errorf("invalid filter expression; expected format: 'attrPath eq value'")
	}
	opTok, ok := p.next()
	if !ok || opTok.kind != scimTokWord {
		return nil, fmt.Errorf("invalid filter expression; expected format: 'attrPath eq value'")
	}
	op := strings.ToLower(opTok.text)
	if !scimFilterReservedCompareOps[op] {
		return nil, fmt.Errorf("invalid filter expression; expected format: 'attrPath eq value'")
	}
	if !scimFilterSupportedCompareOps[op] {
		return nil, fmt.Errorf("the specified filter operator is not supported; only 'eq' is supported")
	}
	// Every supported operator today is binary (attr op value); a unary operator like
	// "pr" would need this to stop unconditionally consuming a value token for it.
	valueTok, ok := p.next()
	if !ok || valueTok.kind == scimTokGrouping {
		return nil, fmt.Errorf("invalid filter expression; expected format: 'attrPath eq value'")
	}
	prefix, attr, valid := splitSCIMAttrPath(attrTok.text)
	if !valid {
		return nil, fmt.Errorf("invalid filter expression; expected format: 'attrPath eq value'")
	}
	if p.rules.IsUnsupported(attr) {
		return nil, fmt.Errorf("filtering on %q is not supported", attr)
	}
	// Custom/extension attributes must be qualified with their schema URN per RFC 7643 §3.10;
	// core schema attributes may be referenced with or without a URN prefix.
	if prefix == "" && !p.rules.IsCore(attr) {
		return nil, fmt.Errorf("filtering on custom attribute %q requires a schema URN prefix", attr)
	}
	attribute := p.rules.Translate(attr)
	if err := utils.ValidateKey(attribute); err != nil {
		return nil, fmt.Errorf("filtering on %q is not supported", attr)
	}
	value, err := parseSCIMCompValue(valueTok.text)
	if err != nil {
		return nil, err
	}
	return &scimFilterExpressionNode{Attr: attribute, Op: op, Value: value}, nil
}

func splitSCIMAttrPath(token string) (prefix, attr string, ok bool) {
	attr = token
	if idx := strings.LastIndexByte(token, ':'); idx >= 0 {
		prefix, attr = token[:idx+1], token[idx+1:]
	}
	if !isValidSCIMAttrName(attr) {
		return "", "", false
	}
	if prefix != "" {
		for _, seg := range strings.Split(strings.TrimSuffix(prefix, ":"), ":") {
			if !isValidSCIMAttrURNSegment(seg) {
				return "", "", false
			}
		}
	}
	return prefix, attr, true
}

func isValidSCIMAttrName(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	if !isASCIILetter(runes[0]) {
		return false
	}
	return allASCIIAttrChars(runes[1:])
}

func isValidSCIMAttrURNSegment(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	if !isASCIILetter(runes[0]) && !isASCIIDigit(runes[0]) {
		return false
	}
	return allASCIIAttrChars(runes[1:])
}

func allASCIIAttrChars(runes []rune) bool {
	for _, r := range runes {
		if !isASCIILetter(r) && !isASCIIDigit(r) && r != '.' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func isASCIILetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isASCIIDigit(r rune) bool  { return r >= '0' && r <= '9' }

// parseSCIMCompValue converts a raw SCIM compValue token into a typed Go value.
// compValue = false / null / true / number / string  (RFC 7159 JSON rules)
func parseSCIMCompValue(raw string) (interface{}, error) {
	// Quoted string — parse as a JSON string literal so escapes are handled correctly.
	if len(raw) > 0 && raw[0] == '"' {
		s, err := strconv.Unquote(raw)
		if err == nil {
			return s, nil
		}
		return nil, fmt.Errorf("invalid quoted string comparison value: %q", raw)
	}
	lower := strings.ToLower(raw)
	switch lower {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		// null comparisons are not meaningful for our store.
		return nil, fmt.Errorf("null comparison values are not supported")
	}
	// Integer
	if intVal, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return intVal, nil
	}
	// Decimal
	if floatVal, err := strconv.ParseFloat(raw, 64); err == nil {
		return floatVal, nil
	}
	return nil, fmt.Errorf("unrecognized comparison value: %q", raw)
}
