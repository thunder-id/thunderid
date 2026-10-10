// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

type Fields = Record<string, unknown>;

export const isObject = (value: unknown): value is Fields =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

/**
 * The prefix that stands for an application's or an agent's OAuth 2 settings, which its read keeps
 * in the `oauth2` entry of `inboundAuthConfig`.
 */
const OAUTH = 'oauth';

/** The field a path's first step stands for in the resource itself. */
export function rootOf(path: string): string {
  const [first] = path.split('.');
  return first === OAUTH ? 'inboundAuthConfig' : first;
}

/** Reads the value at a dot path of a resource, such as `oauth.redirectUris` or `theme.shape`. */
export function valueAt(resource: unknown, path: string): unknown {
  const [first, ...rest] = path.split('.');
  let value: unknown;
  if (first === OAUTH) {
    const entries = isObject(resource) ? resource.inboundAuthConfig : undefined;
    const oauth = Array.isArray(entries)
      ? (entries as unknown[]).find((entry: unknown) => isObject(entry) && entry.type === 'oauth2')
      : undefined;
    value = isObject(oauth) ? oauth.config : undefined;
  } else {
    value = isObject(resource) ? resource[first] : undefined;
  }
  for (const step of rest) {
    value = isObject(value) ? value[step] : undefined;
  }
  return value;
}

/** The types a schema's property can have. */
const TYPES: readonly string[] = ['string', 'number', 'integer', 'boolean', 'object', 'array'];

/**
 * Whether a value is a schema as a user type or an agent type holds one: properties by name, each
 * saying its type.
 */
export function isSchema(value: Fields): boolean {
  const entries = Object.values(value);
  return (
    entries.length > 0 &&
    entries.every((entry: unknown) => isObject(entry) && typeof entry.type === 'string' && TYPES.includes(entry.type))
  );
}
