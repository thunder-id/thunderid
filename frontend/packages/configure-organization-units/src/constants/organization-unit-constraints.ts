// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Length rules the API enforces on organization unit fields, mirrored here so the console can
 * validate before the request is sent. Keep in sync with `backend/internal/ou/model.go`.
 */
const OrganizationUnitConstraints = {
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 100,
  HANDLE_MIN_LENGTH: 1,
  HANDLE_MAX_LENGTH: 100,
} as const;

export default OrganizationUnitConstraints;
