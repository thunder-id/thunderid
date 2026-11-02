// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const generateResourceId = (prefix = 'resource', charCount = 4): string =>
  `${prefix}_${Math.random()
    .toString(36)
    .substring(2, 2 + charCount)}`;

export default generateResourceId;
