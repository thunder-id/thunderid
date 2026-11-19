// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

// Grammar and hit policy.
const (
	// grammarVersionCurrent is the rule grammar this engine understands.
	grammarVersionCurrent = 1
	// HitPolicyFirst evaluates rules in author order; the first rule that is not false decides.
	HitPolicyFirst HitPolicy = "FIRST"
)

// Default and maximum limits on the size of a decision block.
const (
	defaultMaxRules     = 32
	defaultMaxDepth     = 6
	defaultMaxLeaves    = 128
	defaultMaxTests     = 32
	maxListSizeCap      = 1000
	hardCapMaxRules     = 64
	hardCapMaxDepth     = 8
	hardCapMaxLeaves    = 256
	hardCapMaxTests     = 64
	maxRuleIDLength     = 64
	maxLabelLength      = 200
	maxDurationDigits   = 9
	hashSetMinListSize  = 8
	nominalDaysPerYear  = 365
	nominalDaysPerMonth = 30
	secondsPerDay       = 86400
	secondsPerHour      = 3600
	secondsPerMinute    = 60
	daysPerWeek         = 7
	isoDateLength       = 10
	zoneOffsetLength    = len("+00:00")
)

// Largest hours and minutes of an RFC 3339 zone offset, as two-digit text.
const (
	maxOffsetHours   = "23"
	maxOffsetMinutes = "59"
)

// Field names of the decision block, as they appear in its JSON form.
const (
	fieldGrammarVersion  = "grammarVersion"
	fieldHitPolicy       = "hitPolicy"
	fieldRules           = "rules"
	fieldDefault         = "default"
	fieldUnknown         = "unknown"
	fieldTests           = "tests"
	fieldSource          = "source"
	fieldID              = "id"
	fieldLabel           = "label"
	fieldWhen            = "when"
	fieldNextNode        = "nextNode"
	fieldOp              = "op"
	fieldSignal          = "signal"
	fieldValue           = "value"
	fieldRef             = "ref"
	fieldCI              = "ci"
	fieldEvery           = "every"
	fieldOptions         = "options"
	fieldOf              = "of"
	fieldName            = "name"
	fieldFacts           = "facts"
	fieldUnresolved      = "unresolved"
	fieldContext         = "context"
	fieldExpect          = "expect"
	fieldFrontend        = "frontend"
	fieldFrontendVersion = "frontendVersion"
	fieldFlowType        = "flowType"
	fieldApplicationID   = "applicationId"
	fieldOUID            = "ouId"
)

// Operator names.
const (
	opEq         = "eq"
	opGt         = "gt"
	opGte        = "gte"
	opStartsWith = "startsWith"
	opEndsWith   = "endsWith"
	opContains   = "contains"
	opOlderThan  = "olderThan"
	opExists     = "exists"
	opIsEmpty    = "isEmpty"
	opAll        = "all"
	opAny        = "any"
	opNot        = "not"
)

// Diagnostic codes.
const (
	// CodeSyntax reports a value of the wrong JSON type, or a decision block that is not valid JSON.
	CodeSyntax DiagnosticCode = "E_SYNTAX"
	// CodeUnknownField reports a field the grammar does not define.
	CodeUnknownField DiagnosticCode = "E_UNKNOWN_FIELD"
	// CodeMissingField reports a required field that is absent or empty.
	CodeMissingField DiagnosticCode = "E_MISSING_FIELD"
	// CodeGrammarVersion reports a grammar version this engine does not support.
	CodeGrammarVersion DiagnosticCode = "E_GRAMMAR_VERSION"
	// CodeHitPolicy reports a hit policy this engine does not support.
	CodeHitPolicy DiagnosticCode = "E_HIT_POLICY"
	// CodeInvalidID reports a rule ID that does not match the allowed form.
	CodeInvalidID DiagnosticCode = "E_INVALID_ID"
	// CodeDuplicateRuleID reports two rules with the same ID.
	CodeDuplicateRuleID DiagnosticCode = "E_DUPLICATE_RULE_ID"
	// CodeNoRules reports a decision with no rules.
	CodeNoRules DiagnosticCode = "E_NO_RULES"
	// CodeMissingArm reports a missing default or unknown arm.
	CodeMissingArm DiagnosticCode = "E_MISSING_ARM"
	// CodeUnknownNode reports a missing nextNode.
	CodeUnknownNode DiagnosticCode = "E_UNKNOWN_NODE"
	// CodeArity reports a group with fewer than two children, or a negation without exactly one.
	CodeArity DiagnosticCode = "E_ARITY"
	// CodeOperand reports a value or ref that the operator does not accept.
	CodeOperand DiagnosticCode = "E_OPERAND"
	// CodeQualifier reports a qualifier that the operator does not accept.
	CodeQualifier DiagnosticCode = "E_QUALIFIER"
	// CodeOptions reports options that the operator does not accept.
	CodeOptions DiagnosticCode = "E_OPTIONS"
	// CodeUnknownOperator reports an operator that is not registered.
	CodeUnknownOperator DiagnosticCode = "E_UNKNOWN_OPERATOR"
	// CodeUnknownSignal reports a signal that is not declared.
	CodeUnknownSignal DiagnosticCode = "E_UNKNOWN_SIGNAL"
	// CodeLimit reports a limit that is exceeded.
	CodeLimit DiagnosticCode = "E_LIMIT"
	// CodeUnknownComparator reports a signal that declares a comparator that is not registered.
	CodeUnknownComparator DiagnosticCode = "E_UNKNOWN_COMPARATOR"
	// CodeIncomparableLiteral reports a literal the signal's declared comparator cannot read.
	CodeIncomparableLiteral DiagnosticCode = "W_INCOMPARABLE_LITERAL"
	// CodeOperatorComparator reports an operator whose needs the signal's declared comparator cannot meet.
	CodeOperatorComparator DiagnosticCode = "W_OPERATOR_COMPARATOR"
)

// Diagnostic parameter names.
const (
	paramField    = "field"
	paramExpected = "expected"
	paramLimit    = "limit"
	paramName     = "name"
	paramValue    = "value"
)

// Expected JSON types, used as diagnostic parameters.
const (
	typeObject  = "object"
	typeArray   = "array"
	typeString  = "string"
	typeInteger = "integer"
	typeBoolean = "boolean"
	typeLiteral = "string, number, boolean or a non-empty list of them"
)
