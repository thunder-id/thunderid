// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Reports whether a pattern is anchored at both ends. Mirrors the backend's `isRegexAnchored`, which
 * logs a warning for unanchored entries: the matcher searches the raw `Origin` header rather than
 * matching it in full, so an unanchored pattern also allows any origin that merely contains it.
 *
 * This is a textual check rather than a parse: `\A` and `\z` are RE2 anchors with no JavaScript
 * equivalent, but a pattern using them is still anchored for the server that runs it. An anchor the
 * pattern escapes into literal text does not count, so `^foo\$` is reported as unanchored.
 *
 * @param pattern - The regex pattern to test
 * @returns Whether the pattern starts and ends with an anchor
 *
 * @public
 */
export default function isRegexAnchored(pattern: string): boolean {
  const trimmed = pattern.trim();
  const starts = trimmed.startsWith('^') || trimmed.startsWith('\\A');
  return starts && (endsWithAnchor(trimmed, '$') || endsWithAnchor(trimmed, '\\z'));
}

/**
 * Reports whether a pattern ends with the given anchor as an anchor rather than as literal text. An
 * odd number of backslashes in front of it escapes it, so `^foo\$` ends with a literal `$` and is not
 * anchored, while `^foo\\$` ends with an escaped backslash followed by a real anchor.
 *
 * @param pattern - The trimmed pattern to test
 * @param anchor - The end anchor to look for
 * @returns Whether the pattern ends with an unescaped occurrence of the anchor
 */
function endsWithAnchor(pattern: string, anchor: '$' | '\\z'): boolean {
  if (!pattern.endsWith(anchor)) {
    return false;
  }
  let backslashes = 0;
  for (let index = pattern.length - anchor.length - 1; pattern[index] === '\\'; index -= 1) {
    backslashes += 1;
  }
  return backslashes % 2 === 0;
}
