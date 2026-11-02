// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package integrate lists the platforms the CLI's /integrate-* commands cover.
// Each platform's actual guide content lives on thunderid.dev (indexed at
// https://thunderid.dev/llms.txt), fetched on demand via internal/services/docs —
// adding a platform here is the only change needed to add its /integrate-<key> command.
package integrate

// Platform is one /integrate-<Key> command.
type Platform struct {
	Key   string // command suffix, e.g. "nextjs" for "/integrate-nextjs"
	Label string // display label, e.g. "Next.js"
	Slug  string // docs filename under product.DocsBaseURL, e.g. "node" for Node.js
}

// Platforms lists every platform with an /integrate-* command, in display order.
var Platforms = []Platform{
	{Key: "react", Label: "React", Slug: "react"},
	{Key: "nextjs", Label: "Next.js", Slug: "nextjs"},
	{Key: "express", Label: "Express", Slug: "express"},
	{Key: "vue", Label: "Vue", Slug: "vue"},
	{Key: "nuxt", Label: "Nuxt", Slug: "nuxt"},
	{Key: "nodejs", Label: "Node.js", Slug: "node"},
	{Key: "javascript", Label: "JavaScript", Slug: "browser"},
	{Key: "ios", Label: "iOS", Slug: "ios"},
	{Key: "android", Label: "Android", Slug: "android"},
	{Key: "flutter", Label: "Flutter", Slug: "flutter"},
}
