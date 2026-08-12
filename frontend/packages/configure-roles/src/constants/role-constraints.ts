// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Length rules the API enforces on role fields, mirrored here so the console can validate before
 * the request is sent. Keep in sync with `backend/internal/role/model.go`.
 */
const RoleConstraints = {
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 100,
} as const;

export default RoleConstraints;
