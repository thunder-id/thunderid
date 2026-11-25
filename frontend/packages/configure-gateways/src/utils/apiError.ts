// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The control plane's error code, such as `GTW-1030`, from a failed call, if the answer carried one. */
export function errorCodeOf(error: unknown): string | undefined {
  return (error as {response?: {data?: {code?: string}}} | undefined)?.response?.data?.code;
}

/** Why a call failed, as the answer says it, or the error's own message. */
export function errorReasonOf(error: unknown): string | undefined {
  const data = (error as {response?: {data?: unknown}} | undefined)?.response?.data;
  if (typeof data === 'string' && data.trim()) return data.trim();
  const body = (data ?? {}) as {message?: unknown; description?: unknown; error?: unknown; code?: unknown};
  for (const field of [body.description, body.message, body.error, body.code]) {
    if (typeof field === 'string' && field.trim()) return field.trim();
  }
  return error instanceof Error && error.message ? error.message : undefined;
}

/** The gateway section is set in the deployment configuration, so it cannot be changed at runtime. */
export const SETTINGS_DECLARATIVE = 'SCF-1003';

/** The apply would remove more resources than the deployment's threshold and was not confirmed. */
export const REMOVALS_NEED_CONFIRMATION = 'GTW-1030';

/** Whether a failed call was refused for want of a permission. */
export function isForbidden(error: unknown): boolean {
  return (error as {response?: {status?: number}} | undefined)?.response?.status === 403;
}

/** A refusal's interpolation params, each resource key in them named as the caller names that resource. */
function namedParams(error: Error, nameOf?: (key: string) => string | undefined): Record<string, string> {
  const data = (error as {response?: {data?: {description?: unknown; message?: unknown}}}).response?.data;
  const raw =
    (data?.description as {params?: unknown} | undefined)?.params ??
    (data?.message as {params?: unknown} | undefined)?.params;
  const params: Record<string, string> = {};
  if (!raw || typeof raw !== 'object') return params;
  for (const [name, value] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof value === 'string' || typeof value === 'number') params[name] = withNames(String(value), nameOf);
  }
  return params;
}

/**
 * A text with each resource key it names, such as `group/0190…0040`, replaced by what the caller calls
 * that resource. A key the caller does not know is left as it is, so nothing else in the text is touched.
 */
function withNames(text: string, nameOf?: (key: string) => string | undefined): string {
  if (!nameOf) return text;
  return text.replace(
    /(^|[^\w/:.-])([a-z][a-z_]*\/[A-Za-z0-9][A-Za-z0-9._-]*)/g,
    (whole, before: string, key: string) => {
      const named = nameOf(key);
      return named ? `${before}${named}` : whole;
    },
  );
}

/**
 * Why a call failed, for the user, from the error catalog alone: a refusal that names what it refused
 * (its params) reads `errors.<CODE>.details`, which says which resources, each named as the caller names
 * it; otherwise `errors.<CODE>`, then the shared `common:errors.<CODE>`. A code the catalog does not
 * know, or a failure with no code, reads the fallback. Server-returned text is never shown.
 */
export function failureText(
  error: Error,
  translate: (key: string, options?: Record<string, unknown>) => string,
  fallback: string,
  nameOf?: (key: string) => string | undefined,
): string {
  const code = errorCodeOf(error);
  if (!code) return fallback;
  const params = namedParams(error, nameOf);
  // A key the catalog does not hold translates to the empty default, so a translation is present when
  // what it reads is not empty.
  const read = (key: string, values: Record<string, string> = params): string =>
    translate(key, {...values, defaultValue: ''});
  if (Object.keys(params).length > 0) {
    // A refusal names either what is missing or what still refers to the resource; the other reads empty.
    const detailed = read(`errors.${code}.details`, {missing: '', referenced: '', source: '', ...params});
    if (detailed) return detailed;
  }
  return read(`errors.${code}`) || read(`common:errors.${code}`) || fallback;
}
