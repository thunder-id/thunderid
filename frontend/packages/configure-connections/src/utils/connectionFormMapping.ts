// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {parseKeyValuePairs} from './keyValuePairs';
import type {ConnectionFieldDef} from '../config/connectionFormFields';
import {AuthenticationMethods, type AuthenticationMethod} from '../models/authentication-methods';
import type {ConnectionRequest, ConnectionResponse, OutboundAuthentication} from '../models/connection';

/** The placeholder value the API returns for stored secrets. Must never be sent back. */
export const MASKED_SECRET = '******';

function matchesRequiredWhenValue(
  values: ConnectionFormValues,
  requiredWhenValue: ConnectionFieldDef['requiredWhenValue'],
): boolean {
  return requiredWhenValue?.field !== undefined && values[requiredWhenValue.field] === requiredWhenValue.value;
}

/** Flat string-keyed form state shared by all per-vendor forms. */
export type ConnectionFormValues = Record<string, string>;

function valueAtPath(source: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((value, key) => {
    if (typeof value !== 'object' || value === null) {
      return undefined;
    }
    return (value as Record<string, unknown>)[key];
  }, source);
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
    const raw: unknown = valueAtPath(response, field.responsePath ?? field.name);
    if (field.kind === 'switch') {
      values[field.name] = raw === true ? 'true' : 'false';
      continue;
    }
    if (typeof raw === 'string' && raw !== '') {
      values[field.name] = raw;
    } else if (typeof raw === 'number' && Number.isFinite(raw)) {
      values[field.name] = String(raw);
    } else {
      values[field.name] = field.defaultValue ?? '';
    }
  }
  return values;
}

/** Convert flat authentication form values into the structured outbound-authentication API contract. */
export function outboundAuthenticationFromFormValues(values: ConnectionFormValues): OutboundAuthentication {
  const scheme = (values['authenticationScheme'] as AuthenticationMethod | undefined) ?? AuthenticationMethods.NONE;
  if (scheme === AuthenticationMethods.BEARER) {
    return {scheme, bearer: {token: (values['bearerToken'] ?? '').trim()}};
  }
  if (scheme === AuthenticationMethods.BASIC) {
    return {
      scheme,
      basic: {
        username: (values['basicUsername'] ?? '').trim(),
        password: (values['basicPassword'] ?? '').trim(),
      },
    };
  }
  if (scheme === AuthenticationMethods.API_KEY) {
    return {scheme, apiKey: {headers: parseKeyValuePairs(values['httpHeaders'] ?? '')}};
  }
  return {scheme: AuthenticationMethods.NONE};
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
 * Scopes are split on whitespace/commas into an array (omitted when empty). Empty optional
 * fields are omitted rather than sent as empty strings.
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

    // Always include required fields and any non-empty value; omit empty optional fields.
    if (field.required || raw !== '') {
      payload[field.name] = field.kind === 'number' ? Number(raw) : raw;
    }
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

function isLoopbackHost(hostname: string): boolean {
  const normalizedHostname = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  if (normalizedHostname === 'localhost' || normalizedHostname === '::1') {
    return true;
  }
  const octets = normalizedHostname.split('.');
  return (
    octets.length === 4 &&
    octets.every((octet) => /^\d+$/.test(octet) && Number(octet) <= 255) &&
    Number(octets[0]) === 127
  );
}

function isSecureAuthenticatedEndpoint(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || (url.protocol === 'http:' && isLoopbackHost(url.hostname));
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
      const requiredByValue = matchesRequiredWhenValue(values, field.requiredWhenValue);
      if (mode === 'create' && (field.required || requiredByValue) && raw === '') {
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
    const isRequiredByValue = matchesRequiredWhenValue(values, field.requiredWhenValue);

    if ((isRequired || isRequiredByValue) && raw === '') {
      errors[field.name] = 'connections:validation.required';
      continue;
    }

    if (field.kind === 'url' && raw !== '' && !isValidHttpUrl(raw)) {
      errors[field.name] = 'connections:validation.url';
      continue;
    }

    const authenticationConfigured =
      values['authenticationScheme'] !== undefined && values['authenticationScheme'] !== AuthenticationMethods.NONE;
    if (
      authenticationConfigured &&
      field.requiresHttpsWhenAuthenticated &&
      raw !== '' &&
      !isSecureAuthenticatedEndpoint(raw)
    ) {
      errors[field.name] = 'connections:validation.authenticatedEndpointHttps';
      continue;
    }

    if (field.pattern && raw !== '' && !field.pattern.test(raw)) {
      errors[field.name] = field.patternErrorKey ?? 'connections:validation.required';
    }
  }

  return errors;
}
