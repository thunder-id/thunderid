// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Length rules the API enforces on user type fields, mirrored here so the console can validate
 * before the request is sent. Keep in sync with `backend/internal/entitytype/model.go`.
 */
const UserTypeConstraints = {
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 100,
  HANDLE_MAX_LENGTH: 100,
  HANDLE_PATTERN: /^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$/,
} as const;

export default UserTypeConstraints;
