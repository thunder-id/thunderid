// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ConnectionFieldDef} from '../config/connectionFormFields';
import type {ConnectionRequest, ConnectionResponse, OutboundAuthentication} from '../models/connection';

/** The placeholder value the API returns for stored secrets. Must never be sent back. */
export const MASKED_SECRET = '******';

/** Flat string-keyed form state shared by all per-vendor forms. */
export type ConnectionFormValues = Record<string, string>;

/**
 * The authentication method that sends no credentials. Selecting it discards whatever the
 * previous method had stored, which the backend enforces too.
 */
export const AUTH_TYPE_NONE = 'none';

/** Form key holding the selected authentication method. */
export const AUTH_TYPE_FIELD = 'authentication.type';

/** Prefix of the form keys holding the selected method's field values. */
const AUTH_PROPERTY_PREFIX = 'authentication.properties.';

/** Form key holding the transport security mode of a vendor that secures its own transport. */
const TLS_FIELD = 'tls';

/** The transport security mode that sends over an unencrypted connection. */
const TLS_DISABLED = 'none';

/**
 * Form key holding one authentication field's value. Authentication values share the flat form
 * state with the static fields, so they are namespaced to keep a method's field from colliding
 * with a vendor field of the same name.
 */
export function authPropertyField(name: string): string {
  return `${AUTH_PROPERTY_PREFIX}${name}`;
}

/**
 * Nest the namespaced authentication form values into the request object. Returns undefined
 * when the form carries no authentication at all, which is how a vendor without the section
 * leaves the payload unchanged.
 *
 * A blank value is omitted rather than sent. That is what "keep the stored credential" looks
 * like: a credential is blanked when the response is loaded and only becomes non-empty once the
 * user replaces it, so no separate replaced-or-not flag is needed here.
 */
function authenticationFromFormValues(values: ConnectionFormValues): OutboundAuthentication | undefined {
  const type: string = values[AUTH_TYPE_FIELD] ?? '';
  if (type === '') {
    return undefined;
  }
  if (type === AUTH_TYPE_NONE) {
    return {type};
  }

  const properties: Record<string, string> = {};
  for (const [key, value] of Object.entries(values)) {
    if (!key.startsWith(AUTH_PROPERTY_PREFIX)) {
      continue;
    }
    const raw: string = (value ?? '').trim();
    // The mask is display-only; sending it back would store the literal asterisks.
    if (raw === '' || raw === MASKED_SECRET) {
      continue;
    }
    properties[key.slice(AUTH_PROPERTY_PREFIX.length)] = raw;
  }

  return Object.keys(properties).length > 0 ? {properties, type} : {type};
}

/**
 * Flatten a response's authentication object into namespaced form values. A credential comes
 * back masked, so it is blanked here the same way a static secret field is.
 */
export function authenticationToFormValues(response: ConnectionResponse): ConnectionFormValues {
  const values: ConnectionFormValues = {};
  const authentication: OutboundAuthentication | undefined = response.authentication;
  if (!authentication) {
    return values;
  }

  values[AUTH_TYPE_FIELD] = authentication.type || AUTH_TYPE_NONE;
  for (const [name, value] of Object.entries(authentication.properties ?? {})) {
    values[authPropertyField(name)] = value === MASKED_SECRET ? '' : value;
  }
  return values;
}

/**
 * Build empty form values for a create form (all fields blank except the derived redirect URI
 * and fields carrying a default value).
 */
export function emptyFormValues(fields: ConnectionFieldDef[], redirectUri: string): ConnectionFormValues {
  const values: ConnectionFormValues = {};
  for (const field of fields) {
    values[field.name] = field.name === 'redirectUri' ? redirectUri : (field.defaultValue ?? '');
  }
  return values;
}

/**
 * Map a fetched connection response into editable form values. Secrets are never prefilled
 * (the masked "******" is display-only and handled by the secret field's "stored" state).
 * The redirect URI falls back to the derived value if the API didn't store one; other fields
 * fall back to their default value.
 */
export function responseToFormValues(
  response: ConnectionResponse,
  fields: ConnectionFieldDef[],
  redirectUri: string,
): ConnectionFormValues {
  const values: ConnectionFormValues = {};
  for (const field of fields) {
    if (field.kind === 'secret') {
      values[field.name] = '';
      continue;
    }
    if (field.kind === 'scopes') {
      values[field.name] = (response.scopes ?? []).join(' ');
      continue;
    }
    if (field.name === 'redirectUri') {
      values[field.name] = response.redirectUri || redirectUri;
      continue;
    }
    const raw: unknown = (response as unknown as Record<string, unknown>)[field.name];
    if (field.kind === 'switch') {
      values[field.name] = raw === true ? 'true' : 'false';
      continue;
    }
    values[field.name] = typeof raw === 'string' && raw !== '' ? raw : (field.defaultValue ?? '');
  }
  // Authentication is described by the server rather than by the static field list, so its
  // values are merged in separately.
  return {...values, ...authenticationToFormValues(response)};
}

export interface ToRequestOptions {
  mode: 'create' | 'edit';
  /** On edit, whether the user chose to replace the stored secret. */
  secretReplaced?: boolean;
}

/**
 * Convert form values into a vendor request payload.
 *
 * Secret handling (the single guard preventing the stored secret from being overwritten):
 * - create → include the secret as entered.
 * - edit → include the secret only when the user replaced it with a non-empty value;
 *   otherwise omit it so the backend keeps the stored value. Never send the "******" mask.
 *
 * Scopes are split on whitespace/commas into an array (omitted when empty). Number fields are
 * coerced so an integer API field is not sent as a string. Empty optional fields are omitted
 * rather than sent as empty strings.
 */
export function formValuesToRequest(
  values: ConnectionFormValues,
  fields: ConnectionFieldDef[],
  options: ToRequestOptions,
): ConnectionRequest {
  const payload: Record<string, unknown> = {};

  for (const field of fields) {
    const raw: string = (values[field.name] ?? '').trim();

    if (field.kind === 'secret') {
      const keep: boolean = options.mode === 'edit' && (!options.secretReplaced || raw === '');
      if (!keep && raw !== '' && raw !== MASKED_SECRET) {
        payload[field.name] = raw;
      }
      continue;
    }

    if (field.kind === 'scopes') {
      const scopes: string[] = raw.split(/[\s,]+/).filter(Boolean);
      if (scopes.length > 0) {
        payload['scopes'] = scopes;
      }
      continue;
    }

    if (field.kind === 'switch') {
      payload[field.name] = raw === 'true';
      continue;
    }

    // Numeric API fields must not be sent as strings.
    if (field.kind === 'number') {
      const parsed = Number(raw);
      if (raw !== '' && Number.isFinite(parsed)) {
        payload[field.name] = parsed;
      }
      continue;
    }

    // Always include required fields and any non-empty value; omit empty optional fields.
    if (field.required || raw !== '') {
      payload[field.name] = raw;
    }
  }

  const authentication: OutboundAuthentication | undefined = authenticationFromFormValues(values);
  if (authentication) {
    payload['authentication'] = authentication;
  }

  return payload as unknown as ConnectionRequest;
}

function isValidHttpUrl(value: string): boolean {
  try {
    const url: URL = new URL(value);
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
}

/**
 * Validate form values against the field config. Returns a map of field name → i18n error key
 * (empty when valid). The secret is only required on create (omit-to-keep on edit).
 */
export function validateConnectionForm(
  values: ConnectionFormValues,
  fields: ConnectionFieldDef[],
  mode: 'create' | 'edit',
): Record<string, string> {
  const errors: Record<string, string> = {};

  for (const field of fields) {
    if (field.revealedBy && values[field.revealedBy] !== 'true') {
      continue;
    }

    const raw: string = (values[field.name] ?? '').trim();

    if (field.kind === 'secret') {
      if (mode === 'create' && field.required && raw === '') {
        errors[field.name] = 'connections:validation.required';
      }
      continue;
    }

    if (field.kind === 'readonly-copy' || field.kind === 'scopes' || field.kind === 'switch') {
      continue;
    }

    const requiredWhen: string | undefined = field.requiredWhen;
    const isRequired: boolean =
      Boolean(field.required) || (requiredWhen !== undefined && values[requiredWhen] === 'true');

    if (isRequired && raw === '') {
      errors[field.name] = 'connections:validation.required';
      continue;
    }

    if (field.kind === 'url' && raw !== '' && !isValidHttpUrl(raw)) {
      errors[field.name] = 'connections:validation.url';
      continue;
    }

    if (field.pattern && raw !== '' && !field.pattern.test(raw)) {
      errors[field.name] = field.patternErrorKey ?? 'connections:validation.required';
    }
  }

  // Transport policy mirrored from the backend: credentials must not travel in the clear. The
  // server rejects the combination with a message the console cannot surface, so the form has to
  // catch it. Keyed off the transport security field so it only applies to vendors that have one.
  const authType: string = values[AUTH_TYPE_FIELD] ?? '';
  const securesTransport: boolean = fields.some((field) => field.name === TLS_FIELD);
  if (securesTransport && authType !== '' && authType !== AUTH_TYPE_NONE && values[TLS_FIELD] === TLS_DISABLED) {
    errors[TLS_FIELD] = 'connections:validation.tlsRequiredForAuthentication';
  }

  return errors;
}
