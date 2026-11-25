// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The most characters a variable identifier can have, which the server enforces on save.
 */
export const MAX_IDENTIFIER_LENGTH = 64;

/**
 * What a variable identifier looks like, as a regular expression source: a letter, then letters, digits and
 * underscores, up to {@link MAX_IDENTIFIER_LENGTH} characters. It is the same grammar the server accepts, so one
 * source builds every pattern that reads an identifier.
 */
export const IDENTIFIER_SOURCE = `[A-Za-z][A-Za-z0-9_]{0,${MAX_IDENTIFIER_LENGTH - 1}}`;

/**
 * Matches a whole variable identifier.
 */
export const IDENTIFIER_PATTERN = new RegExp(`^${IDENTIFIER_SOURCE}$`);

/**
 * Matches a property holding only a `{{var(identifier)}}` placeholder, and captures the identifier.
 */
export const VARIABLE_PLACEHOLDER_PATTERN = new RegExp(`^\\{\\{\\s*var\\(\\s*(${IDENTIFIER_SOURCE})\\s*\\)\\s*\\}\\}$`);
