// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import generateResourceId from './generateResourceId';
import type {JsonArray, JsonObject, JsonValue} from '../models/json';

interface Replacer {
  key?: string;
  placeholder?: string;
  type?: string;
  value?: string;
  /**
   * Prefix for generated ids. Node ids surface as the step title on the
   * canvas, so a purposeful prefix (e.g. `recovery_call`) keeps otherwise
   * identical nodes tellable apart.
   */
  prefix?: string;
  [key: string]: unknown;
}

/**
 * Matches a `{{KEY}}` placeholder, whether it is the whole string or embedded in a longer one,
 * e.g. the `data-action-ref` sentinel inside a rich text's HTML label. Deliberately restricted to
 * identifier characters so the runtime template literals (`{{ t(...) }}`,
 * `{{meta(application.url)}}`) never match.
 */
const PLACEHOLDER = /\{\{([A-Za-z0-9_]+)\}\}/g;

const updateTemplatePlaceholderReferences = <T = JsonValue>(
  obj: T,
  replacers: Replacer[],
): [T, Map<string, string>] => {
  const placeholderCache = new Map<string, string>();

  // Resolves one placeholder key to its replacement, minting generated ids at most once so
  // every occurrence of a key (the `action.ref` and the label's `data-action-ref`, say) ends
  // up with the same value. Returns undefined when the key has no replacement to apply, which
  // leaves the placeholder as authored.
  const resolvePlaceholder = (placeholderKey: string): string | undefined => {
    if (placeholderCache.has(placeholderKey)) {
      return placeholderCache.get(placeholderKey);
    }

    const replacer = replacers?.find((r) => r.key === placeholderKey || r.placeholder === placeholderKey);

    if (!replacer) {
      return undefined;
    }

    let replacementValue: string;

    if (replacer.type === 'ID') {
      replacementValue = generateResourceId(replacer.prefix ?? replacer.type);
    } else if (replacer.value !== undefined) {
      replacementValue = replacer.value;
    } else {
      return undefined;
    }

    placeholderCache.set(placeholderKey, replacementValue);

    return replacementValue;
  };

  const replacePlaceholders = (input: JsonValue): JsonValue => {
    if (typeof input === 'string') {
      // Only a written-out `{{KEY}}` is substituted, so a literal that happens to read like a
      // replacer key, such as the key of a replacer definition itself, is left alone.
      return input.replace(PLACEHOLDER, (match, placeholderKey: string) => resolvePlaceholder(placeholderKey) ?? match);
    }
    if (Array.isArray(input)) {
      return input.map((value) => replacePlaceholders(value)) as JsonArray;
    }
    if (typeof input === 'object' && input !== null) {
      return Object.fromEntries(
        Object.entries(input).map(([key, value]) => [key, replacePlaceholders(value)]),
      ) as JsonObject;
    }

    return input;
  };

  return [replacePlaceholders(obj as unknown as JsonValue) as unknown as T, placeholderCache];
};

export default updateTemplatePlaceholderReferences;
