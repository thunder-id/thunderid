// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

// maxLinkingCandidates bounds how many accounts one identity's account-linking lookups can match.
const maxLinkingCandidates = 10

// auditReasonAccountSuspended is the audit reason of an enrollment refused by a suspension of the identity.
const auditReasonAccountSuspended = "account_suspended"
