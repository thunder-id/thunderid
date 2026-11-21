// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

// HitPolicy decides which rule of a decision selects the arm.
type HitPolicy string

// DiagnosticCode identifies a problem found in a decision block.
type DiagnosticCode string

// Severity is the severity of a diagnostic.
type Severity string

// Severities of a diagnostic.
const (
	// SeverityError refuses a save.
	SeverityError Severity = "ERROR"
	// SeverityWarning is reported, and the save goes ahead.
	SeverityWarning Severity = "WARNING"
)

// Truth is the three-valued result of a condition. The order F < U < T makes AND the minimum,
// OR the maximum and NOT the reflection around U.
type Truth uint8

// Truth values.
const (
	// TruthFalse is false.
	TruthFalse Truth = iota
	// TruthUnknown is unknown: a value is missing, its source failed, or it cannot be compared.
	TruthUnknown
	// TruthTrue is true.
	TruthTrue
)

// Reason explains the truth of a leaf condition.
type Reason string

// Reasons of a leaf condition.
const (
	// ReasonMatch is a true leaf.
	ReasonMatch Reason = "MATCH"
	// ReasonNoMatch is a false leaf.
	ReasonNoMatch Reason = "NO_MATCH"
	// ReasonUnresolved is an unknown leaf whose signal's source failed.
	ReasonUnresolved Reason = "UNRESOLVED"
	// ReasonAbsent is an unknown leaf whose signal has no value.
	ReasonAbsent Reason = "ABSENT"
	// ReasonIncomparable is an unknown leaf whose values no comparator can compare.
	ReasonIncomparable Reason = "INCOMPARABLE"
	// ReasonNotEvaluated is a leaf that was skipped, or whose signal was not fetched.
	ReasonNotEvaluated Reason = "NOT_EVALUATED"
)

// SelectedBy says what selected the arm of a decision.
type SelectedBy string

// Ways an arm is selected.
const (
	// SelectedByRule is a rule that evaluated to true.
	SelectedByRule SelectedBy = "RULE"
	// SelectedByDefault is the default arm, taken when every rule is false.
	SelectedByDefault SelectedBy = "DEFAULT"
	// SelectedByUnknown is the unknown arm, taken when the first rule that is not false is unknown.
	SelectedByUnknown SelectedBy = "UNKNOWN"
)

// CostClass orders signals by how expensive they are to fetch.
type CostClass uint8

// Cost classes, cheapest first.
const (
	// CostLocal is a value already in memory.
	CostLocal CostClass = iota
	// CostStore is a value that needs a store read.
	CostStore
	// CostRemote is a value that needs a network call.
	CostRemote
)

// ComparatorName names a way of reading two values to compare them.
type ComparatorName string

// Comparator names.
const (
	// ComparatorScale reads values as members of a declared ordered scale.
	ComparatorScale ComparatorName = "scale"
	// ComparatorInstant reads values as dates or timestamps.
	ComparatorInstant ComparatorName = "instant"
	// ComparatorDuration reads values as ISO 8601 durations.
	ComparatorDuration ComparatorName = "duration"
	// ComparatorNumber reads values as decimal numbers.
	ComparatorNumber ComparatorName = "number"
	// ComparatorBoolean reads values as true or false.
	ComparatorBoolean ComparatorName = "boolean"
	// ComparatorText reads values as text.
	ComparatorText ComparatorName = "text"
)

// OperatorKind is the shape of condition an operator forms.
type OperatorKind string

// Operator kinds.
const (
	// OperatorKindComparison compares a signal with a value or another signal.
	OperatorKindComparison OperatorKind = "COMPARISON"
	// OperatorKindPresence tests a signal on its own.
	OperatorKindPresence OperatorKind = "PRESENCE"
	// OperatorKindGroup combines two or more conditions.
	OperatorKindGroup OperatorKind = "GROUP"
	// OperatorKindNegation negates one condition.
	OperatorKindNegation OperatorKind = "NEGATION"
)

// Decision is the decision block of a DECISION node: ordered rules and the two mandatory arms.
type Decision struct {
	GrammarVersion int             `json:"grammarVersion"`
	HitPolicy      HitPolicy       `json:"hitPolicy"`
	Rules          []Rule          `json:"rules"`
	Default        Arm             `json:"default"`
	Unknown        Arm             `json:"unknown"`
	Tests          []DecisionTest  `json:"tests,omitempty"`
	Source         *DecisionSource `json:"source,omitempty"`
}

// Rule is one rule of a decision: a condition and the node to go to when it is true.
type Rule struct {
	ID       string     `json:"id"`
	Label    string     `json:"label,omitempty"`
	When     *Predicate `json:"when"`
	NextNode string     `json:"nextNode"`
}

// Arm is an exit of a decision that is not a rule.
type Arm struct {
	Label    string `json:"label,omitempty"`
	NextNode string `json:"nextNode"`
}

// DecisionTest is a scenario a decision must route as expected.
type DecisionTest struct {
	Name       string             `json:"name"`
	Context    *TestContext       `json:"context,omitempty"`
	Facts      map[string]Literal `json:"facts,omitempty"`
	Unresolved []string           `json:"unresolved,omitempty"`
	Expect     string             `json:"expect"`
}

// TestContext is the context a decision test runs in.
type TestContext struct {
	FlowType      string `json:"flowType,omitempty"`
	ApplicationID string `json:"applicationId,omitempty"`
	OUID          string `json:"ouId,omitempty"`
}

// DecisionSource records the authoring front end that produced a decision.
type DecisionSource struct {
	Frontend        string `json:"frontend,omitempty"`
	FrontendVersion int    `json:"frontendVersion,omitempty"`
}

// Diagnostic is one problem found in a decision block.
type Diagnostic struct {
	Path     string            `json:"path"`
	Code     DiagnosticCode    `json:"code"`
	Severity Severity          `json:"severity"`
	Params   map[string]string `json:"params,omitempty"`
}

// SignalSpec is what the compiler needs to know about a signal.
type SignalSpec struct {
	Name       string
	Comparator ComparatorName
	Scale      []string
	CostClass  CostClass
}

// Outcome is the result of evaluating a decision.
type Outcome struct {
	SelectedBy SelectedBy `json:"selectedBy"`
	RuleID     string     `json:"ruleId,omitempty"`
	NextNode   string     `json:"nextNode"`
	Settled    bool       `json:"-"`
}

// LeafRecord is the result of one leaf condition in an evaluation.
type LeafRecord struct {
	Path       string         `json:"path"`
	Signal     string         `json:"signal"`
	Operator   string         `json:"op"`
	Truth      Truth          `json:"truth"`
	Reason     Reason         `json:"reason"`
	Comparator ComparatorName `json:"comparator,omitempty"`
}

// RightSpec says which right-hand operands an operator accepts.
type RightSpec struct {
	Value bool `json:"value"`
	List  bool `json:"list"`
	Ref   bool `json:"ref"`
}

// Phrases are the words used for an operator in plain-language summaries.
type Phrases struct {
	Positive string `json:"positive"`
	Negative string `json:"negative"`
}

// OperatorDescriptor describes an operator to clients.
type OperatorDescriptor struct {
	Name       string       `json:"name"`
	Since      int          `json:"since"`
	Kind       OperatorKind `json:"kind"`
	Right      *RightSpec   `json:"right,omitempty"`
	Qualifiers []string     `json:"qualifiers"`
	Phrases    Phrases      `json:"phrases"`
}

// ComparatorDescriptor describes a comparator to clients.
type ComparatorDescriptor struct {
	Name      ComparatorName `json:"name"`
	Inferred  bool           `json:"inferred"`
	Order     int            `json:"order,omitempty"`
	Ordering  bool           `json:"ordering"`
	TextMatch bool           `json:"textMatch"`
}
