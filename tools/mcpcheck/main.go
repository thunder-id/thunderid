// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// mcpcheck fails a change that adds an API operation or a feature specification without an MCP
// exposure decision.
//
// Usage:
//
//	mcpcheck spec <spec.md>...
//	mcpcheck api -api <dir> -tools <dir> [-standalone <file>] [-base-api <dir> -base-tools <dir>]
//
// It prints one line per problem and exits 1 when there is any, or 2 on a usage or read error.
package main

import (
	"fmt"
	"os"
)

var tiers = map[string]bool{"read": true, "write": true, "destructive": true}

// problem is one failure. key identifies it across branches, so it never carries a line number or
// a directory that differs between the head and the base checkout.
type problem struct {
	key, msg string
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var problems []problem
	var err error
	switch os.Args[1] {
	case "spec":
		problems, err = runSpec(os.Args[2:])
	case "api":
		problems, err = runAPI(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Println(p.msg)
	}
	if len(problems) > 0 {
		fmt.Printf("%d problem(s)\n", len(problems))
		os.Exit(1)
	}
	fmt.Println("OK")
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:\n"+
		"  mcpcheck spec <spec.md>...\n"+
		"  mcpcheck api -api <dir> -tools <dir> [-standalone <file>] [-base-api <dir> -base-tools <dir>]")
	os.Exit(2)
}
