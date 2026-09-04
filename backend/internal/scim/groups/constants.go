// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

// SCIM PATCH operation values (RFC 7644 §3.5.2). Groups is the only resource
// with PATCH support today.
const (
	scimPatchOpAdd     = "add"
	scimPatchOpRemove  = "remove"
	scimPatchOpReplace = "replace"
)
