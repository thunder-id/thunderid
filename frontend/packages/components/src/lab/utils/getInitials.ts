// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const getInitials = (name?: string): string => {
  const normalized = name?.trim();
  if (!normalized) return '?';
  const parts = normalized.split(/\s+/);
  if (parts.length >= 2) return `${Array.from(parts[0])[0]}${Array.from(parts[1])[0]}`.toUpperCase();
  return Array.from(normalized).slice(0, 2).join('').toUpperCase();
};

export default getInitials;
