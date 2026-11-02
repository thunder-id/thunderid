// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  decodeKeyValuePairs,
  encodeKeyValuePairs,
  isCompleteKeyValuePair,
  type KeyValuePair,
  parseKeyValuePairs,
  serializeKeyValuePairs,
} from './keyValuePairs';
import type {ConnectionFieldDef} from '../config/connectionFormFields';
import {
  AUTHENTICATION_METHOD_FIELD_NAMES,
  AuthenticationMethods,
  type AuthenticationMethod,
} from '../models/authentication-methods';
import type {
  ConnectionRequest,
  ConnectionResponse,
  OutboundAuthConfig,
  OutboundAuthentication,
  OutboundAuthMethod,
} from '../models/connection';

/** The placeholder value the API returns for stored secrets. */
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
 * The authentication method that sends no credentials. Selecting it discards whatever the
 * previous method had stored, which the backend enforces too.
 */
export const AUTH_TYPE_NONE = 'none';

/** Form key holding the selected authentication method. */
export const AUTH_TYPE_FIELD = 'authentication.type';

/** Prefix of the form keys holding the selected method's field values. */
const AUTH_PROPERTY_PREFIX = 'authentication.properties.';

/**
 * Form key holding, as a pairs list, the properties whose names the administrator chooses, such
 * as API-key headers. Kept outside {@link AUTH_PROPERTY_PREFIX} so a header name cannot collide
 * with a field key.
 */
export const AUTH_ADDITIONAL_PROPERTIES_FIELD = 'authentication.additionalProperties';

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
 *
 * With the methods known, only the selected method's own fields are sent, so a value typed under
 * another method is not sent with this one. Its additional properties are sent as entered; a
 * stored one still holds the mask, which the server resolves to the stored value.
 */
function authenticationFromFormValues(
  values: ConnectionFormValues,
  methods?: OutboundAuthMethod[],
): OutboundAuthConfig | undefined {
  const type: string = values[AUTH_TYPE_FIELD] ?? '';
  if (type === '') {
    return undefined;
  }
  if (type === AUTH_TYPE_NONE) {
    return {type};
  }

  const method: OutboundAuthMethod | undefined = methods?.find((candidate) => candidate.type === type);
  const properties: Record<string, string> = {};
  for (const [key, value] of Object.entries(values)) {
    if (!key.startsWith(AUTH_PROPERTY_PREFIX)) {
      continue;
    }
    const name: string = key.slice(AUTH_PROPERTY_PREFIX.length);
    if (method && !(method.fields ?? []).some((field) => field.key === name)) {
      continue;
    }
    const raw: string = (value ?? '').trim();
    // The mask is display-only; sending it back would store the literal asterisks.
    if (raw === '' || raw === MASKED_SECRET) {
      continue;
    }
    properties[name] = raw;
  }
  if (method?.additionalProperties) {
    for (const pair of decodeKeyValuePairs(values[AUTH_ADDITIONAL_PROPERTIES_FIELD] ?? '', 'pairs')) {
      if (isCompleteKeyValuePair(pair)) {
        properties[pair.name.trim()] = pair.value.trim();
      }
    }
  }

  return Object.keys(properties).length > 0 ? {properties, type} : {type};
}

/**
 * Error key for the selected method's additional properties, or undefined when they are valid or
 * the method takes none. A half-filled row is reported rather than dropped, so a cleared value
 * never deletes its header unnoticed.
 */
export function authenticationAdditionalPropertiesError(
  values: ConnectionFormValues,
  methods: OutboundAuthMethod[],
): string | undefined {
  const type: string = values[AUTH_TYPE_FIELD] || AUTH_TYPE_NONE;
  if (!methods.find((method) => method.type === type)?.additionalProperties) {
    return undefined;
  }
  const pairs: KeyValuePair[] = decodeKeyValuePairs(values[AUTH_ADDITIONAL_PROPERTIES_FIELD] ?? '', 'pairs');
  if (!pairs.every(isCompleteKeyValuePair)) {
    return 'connections:validation.keyValuePair';
  }
  const names: string[] = pairs.map((pair) => pair.name.trim().toLowerCase());
  if (new Set(names).size !== names.length) {
    return 'connections:validation.httpEndpointHeaderDuplicate';
  }
  return undefined;
}

/**
 * Flatten a response's authentication object into namespaced form values. A credential comes
 * back masked, so it is blanked here the same way a static secret field is. A property that is
 * not one of the method's fields belongs to its additional properties and keeps its mask, so the
 * row shows as stored.
 */
export function authenticationToFormValues(
  response: ConnectionResponse,
  methods?: OutboundAuthMethod[],
): ConnectionFormValues {
  const values: ConnectionFormValues = {};
  const authentication: Partial<OutboundAuthConfig> | undefined = response.authentication;
  // An AuthZEN PDP returns the scheme-based shape, which carries no type and is mapped by its
  // own fields, so only the generic shape is flattened here.
  if (!authentication?.type) {
    return values;
  }

  values[AUTH_TYPE_FIELD] = authentication.type;
  const method: OutboundAuthMethod | undefined = methods?.find((candidate) => candidate.type === authentication.type);
  const additional: KeyValuePair[] = [];
  for (const [name, value] of Object.entries(authentication.properties ?? {})) {
    if (method?.additionalProperties && !(method.fields ?? []).some((field) => field.key === name)) {
      additional.push({name, value});
      continue;
    }
    values[authPropertyField(name)] = value === MASKED_SECRET ? '' : value;
  }
  if (additional.length > 0) {
    values[AUTH_ADDITIONAL_PROPERTIES_FIELD] = encodeKeyValuePairs(additional, 'pairs');
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
  authenticationMethods?: OutboundAuthMethod[],
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
    if (field.name === 'apiKeyHeaders' || field.name === 'httpHeaders') {
      const properties = response.authentication?.type === 'api_key' ? response.authentication.properties : undefined;
      values[field.name] = serializeKeyValuePairs(
        Object.entries(properties ?? {}).map(([name, value]) => ({name, value})),
      );
      continue;
    }
    if (field.name === 'smsHttpHeaders') {
      values[field.name] = response.httpHeaders ?? '';
      continue;
    }
    if (field.name === 'authenticationScheme') {
      values[field.name] = response.authentication?.type?.toUpperCase() ?? AuthenticationMethods.NONE;
      continue;
    }
    if (field.name === 'redirectUri') {
      values[field.name] = response.redirectUri?.trim() ? response.redirectUri : redirectUri;
      continue;
    }
    const raw: unknown = valueAtPath(response, field.responsePath ?? field.name);
    if (field.kind === 'key-value' && field.keyValueFormat === 'pairs') {
      values[field.name] = Array.isArray(raw) ? encodeKeyValuePairs(raw as KeyValuePair[], 'pairs') : '';
      continue;
    }
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
  // Authentication is described by the server rather than by the static field list, so its
  // values are merged in separately.
  return fields.some((field) => field.name === 'apiKeyHeaders')
    ? values
    : {...values, ...authenticationToFormValues(response, authenticationMethods)};
}

/** Convert flat authentication form values into the structured outbound-authentication API contract. */
export function outboundAuthenticationFromFormValues(values: ConnectionFormValues): OutboundAuthentication {
  const scheme = (values['authenticationScheme'] as AuthenticationMethod | undefined) ?? AuthenticationMethods.NONE;
  if (scheme === AuthenticationMethods.BEARER) {
    return {type: 'bearer', properties: {token: (values['bearerToken'] ?? '').trim()}};
  }
  if (scheme === AuthenticationMethods.BASIC) {
    return {
      type: 'basic',
      properties: {
        username: (values['basicUsername'] ?? '').trim(),
        password: (values['basicPassword'] ?? '').trim(),
      },
    };
  }
  if (scheme === AuthenticationMethods.API_KEY) {
    const headers = parseKeyValuePairs(values['httpHeaders'] ?? '');
    if (headers.length === 0) {
      return {type: 'none'};
    }
    return {
      type: 'api_key',
      properties: Object.fromEntries(headers.map(({name, value}) => [name, value])),
    };
  }
  return {type: 'none'};
}

/**
 * Whether two URLs differ in scheme, host or port, with default ports normalized. A URL that does
 * not parse counts as changed unless the text is identical.
 */
export function urlOriginChanged(previous: string, next: string): boolean {
  if (previous === next) {
    return false;
  }
  try {
    return new URL(previous).origin !== new URL(next).origin;
  } catch {
    return true;
  }
}

export interface ToRequestOptions {
  mode: 'create' | 'edit';
  /** On edit, whether the user chose to replace the stored secret. */
  secretReplaced?: boolean;
  /** The caller handles authentication separately from metadata-driven fields. */
  excludeGenericAuthentication?: boolean;
  /** The vendor's authentication methods, so only the selected method's properties are sent. */
  authenticationMethods?: OutboundAuthMethod[];
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
    if (AUTHENTICATION_METHOD_FIELD_NAMES.has(field.name)) {
      continue;
    }
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

    if (field.name === 'apiKeyHeaders') {
      const headers = parseKeyValuePairs(raw);
      if (headers.length > 0) {
        payload['authentication'] = {
          type: 'api_key',
          properties: Object.fromEntries(headers.map(({name, value}) => [name, value])),
        };
      } else if (options.mode === 'edit') {
        payload['authentication'] = {type: 'none'};
      }
      continue;
    }
    if (field.name === 'smsHttpHeaders') {
      if (raw !== '' || options.mode === 'edit') {
        payload['httpHeaders'] = raw;
      }
      continue;
    }

    if (field.kind === 'switch') {
      payload[field.name] = raw === 'true';
      continue;
    }

    // Sent on every edit, even when empty, so removing the last pair clears the stored list.
    // Validation blocks a half-filled row before this point; it is never sent.
    if (field.kind === 'key-value' && field.keyValueFormat === 'pairs') {
      const pairs: KeyValuePair[] = decodeKeyValuePairs(raw, 'pairs').filter(isCompleteKeyValuePair);
      if (options.mode === 'edit' || pairs.length > 0) {
        payload[field.name] = pairs;
      }
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

  const authentication: OutboundAuthConfig | undefined = authenticationFromFormValues(
    values,
    options.authenticationMethods,
  );
  if (
    authentication &&
    !options.excludeGenericAuthentication &&
    !fields.some((field) => field.name === 'apiKeyHeaders')
  ) {
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

function isSecureAuthenticatedEndpoint(value: string, allowLoopbackHttp: boolean): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || (allowLoopbackHttp && url.protocol === 'http:' && isLoopbackHost(url.hostname));
  } catch {
    return false;
  }
}

/** Error key for a URL that must be https because authentication is set. */
export const HTTPS_REQUIRED_WITH_AUTHENTICATION_ERROR = 'connections:validation.httpsRequiredWithAuthentication';

function hasUserInfo(value: string): boolean {
  try {
    const url = new URL(value);
    return url.username !== '' || url.password !== '';
  } catch {
    return false;
  }
}

function keyValueNameError(field: ConnectionFieldDef, raw: string): string | undefined {
  const names: string[] = decodeKeyValuePairs(raw, field.keyValueFormat).map((pair) => pair.name.toLowerCase());
  const reserved = new Set<string>((field.reservedNames ?? []).map((name) => name.toLowerCase()));
  if (names.some((name) => reserved.has(name))) {
    return 'connections:validation.httpEndpointHeaderReserved';
  }
  if (field.uniqueNames && new Set(names).size !== names.length) {
    return 'connections:validation.httpEndpointHeaderDuplicate';
  }
  return undefined;
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

    if (field.kind === 'key-value' && raw !== '') {
      const pairs: KeyValuePair[] = decodeKeyValuePairs(raw, field.keyValueFormat);
      if (!pairs.every(isCompleteKeyValuePair)) {
        errors[field.name] = 'connections:validation.keyValuePair';
        continue;
      }
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

    if (field.kind === 'url' && field.rejectsUserInfo && hasUserInfo(raw)) {
      errors[field.name] = 'connections:validation.httpEndpointUrlUserInfo';
      continue;
    }

    if (field.kind === 'key-value') {
      const nameError: string | undefined = keyValueNameError(field, raw);
      if (nameError) {
        errors[field.name] = nameError;
        continue;
      }
    }

    const authenticationConfigured =
      (values['authenticationScheme'] !== undefined && values['authenticationScheme'] !== AuthenticationMethods.NONE) ||
      ((values[AUTH_TYPE_FIELD] ?? '') !== '' && values[AUTH_TYPE_FIELD] !== AUTH_TYPE_NONE);
    if (
      authenticationConfigured &&
      field.requiresHttpsWhenAuthenticated &&
      raw !== '' &&
      !isSecureAuthenticatedEndpoint(raw, !field.rejectsLoopbackHttp)
    ) {
      errors[field.name] = field.rejectsLoopbackHttp
        ? HTTPS_REQUIRED_WITH_AUTHENTICATION_ERROR
        : 'connections:validation.authenticatedEndpointHttps';
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
