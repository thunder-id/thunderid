// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package plane names the ThunderID distributions a release can ship: the quickstart, which runs a
// Control Plane and a managed Data Plane already wired together, and the Data Plane on its own.
package plane

import (
	"fmt"
	"strings"
)

// Plane is a distribution of a release.
type Plane string

const (
	// Auto picks the quickstart when the release ships one, and the Data Plane otherwise, which is
	// the only distribution a 1.x release has.
	Auto Plane = ""
	// Quickstart runs a Control Plane, the Data Plane it manages, and the auth-dp the Control Plane
	// console signs in through.
	Quickstart Plane = "quickstart"
	// Data is the Data Plane on its own.
	Data Plane = "dp"
)

// Parse reads a --plane value.
func Parse(value string) (Plane, error) {
	switch p := Plane(strings.ToLower(strings.TrimSpace(value))); p {
	case Quickstart, Data:
		return p, nil
	default:
		return Auto, fmt.Errorf("--plane %q is not a plane; pass quickstart or dp", value)
	}
}

// Label names the plane in output.
func (p Plane) Label() string {
	if p == Quickstart {
		return "quickstart"
	}
	return "Data Plane"
}
