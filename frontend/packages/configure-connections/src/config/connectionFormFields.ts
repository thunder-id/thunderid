// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type ConnectionType, ConnectionTypes} from '../models/connection';

export type ConnectionFieldKind =
  | 'text'
  | 'url'
  | 'secret'
  | 'scopes'
  | 'readonly-copy'
  | 'switch'
  | 'select'
  | 'key-value';

/** Which form mode renders a field. Defaults to 'both' when unset. */
export type ConnectionFieldVisibility = 'create' | 'edit' | 'both';

/** One choice of a 'select' field. Values are sent to the API verbatim. */
export interface ConnectionFieldOption {
  value: string;
  label: string;
}

export interface ConnectionFieldDef {
  /** Request payload property this field maps to. */
  name: string;
  /** i18n key under the connections namespace. */
  labelKey: string;
  hintKey?: string;
  kind: ConnectionFieldKind;
  /** Required on create. Secret fields are required on create but optional (omit-to-keep) on edit. */
  required?: boolean;
  placeholder?: string;
  /** Choices of a 'select' field. */
  options?: ConnectionFieldOption[];
  /** i18n key for the add-row button of a 'key-value' field. */
  addLabelKey?: string;
  /** Value prefilled on create (and used when the API returns none). */
  defaultValue?: string;
  /** Format the value must match (checked only when non-empty). Mirrors backend validation. */
  pattern?: RegExp;
  /** i18n key for the error shown when {@link pattern} does not match. */
  patternErrorKey?: string;
  /** i18n key for a group sub-heading rendered above this field. */
  section?: string;
  /** Renders only when the named switch field's value is truthy. */
  revealedBy?: string;
  /** Becomes required when the named switch field's value is truthy. */
  requiredWhen?: string;
  /** Which form mode renders this field (default 'both'). Optional fields are edit-only to keep create simple. */
  visibility?: ConnectionFieldVisibility;
}

const NAME_FIELD = (placeholder: string): ConnectionFieldDef => ({
  name: 'name',
  labelKey: 'connections:form.fields.name.label',
  hintKey: 'connections:form.fields.name.hint',
  kind: 'text',
  required: true,
  placeholder,
});

const PROMPT_FIELD: ConnectionFieldDef = {
  name: 'prompt',
  labelKey: 'connections:form.fields.prompt.label',
  hintKey: 'connections:form.fields.prompt.hint',
  kind: 'text',
  placeholder: 'select_account',
  visibility: 'edit',
};

const oauthFields = (
  namePlaceholder: string,
  clientIdPlaceholder: string,
  scopesHintKey = 'connections:form.fields.scopes.hint',
  scopesPlaceholder = 'openid email profile',
): ConnectionFieldDef[] => [
  NAME_FIELD(namePlaceholder),
  {
    name: 'clientId',
    labelKey: 'connections:form.fields.clientId.label',
    hintKey: 'connections:form.fields.clientId.hint',
    kind: 'text',
    required: true,
    placeholder: clientIdPlaceholder,
  },
  {
    name: 'clientSecret',
    labelKey: 'connections:form.fields.clientSecret.label',
    hintKey: 'connections:form.fields.clientSecret.hint',
    kind: 'secret',
    required: true,
  },
  {
    name: 'redirectUri',
    labelKey: 'connections:form.fields.redirectUri.label',
    kind: 'readonly-copy',
    visibility: 'edit',
  },
  {
    name: 'scopes',
    labelKey: 'connections:form.fields.scopes.label',
    hintKey: scopesHintKey,
    kind: 'scopes',
    placeholder: scopesPlaceholder,
    visibility: 'edit',
  },
  PROMPT_FIELD,
];

const TOKEN_EXCHANGE_ENABLED_FIELD: ConnectionFieldDef = {
  name: 'tokenExchangeEnabled',
  labelKey: 'connections:form.fields.tokenExchangeEnabled.label',
  hintKey: 'connections:form.fields.tokenExchangeEnabled.hint',
  kind: 'switch',
  section: 'connections:form.sections.federation',
  visibility: 'edit',
};

const TRUSTED_TOKEN_AUDIENCE_FIELD: ConnectionFieldDef = {
  name: 'trustedTokenAudience',
  labelKey: 'connections:form.fields.trustedTokenAudience.label',
  hintKey: 'connections:form.fields.trustedTokenAudience.hint',
  kind: 'text',
  placeholder: 'my-external-client-id',
  revealedBy: 'tokenExchangeEnabled',
  visibility: 'edit',
};

/**
 * Ordered field definitions per connection type, driving the shared {@link ConnectionForm}
 * and its dynamically-built validation schema.
 */
export const CONNECTION_FORM_FIELDS: Record<ConnectionType, ConnectionFieldDef[]> = {
  [ConnectionTypes.GOOGLE]: oauthFields('Google Workspace', '1234567890-abc.apps.googleusercontent.com'),
  [ConnectionTypes.GITHUB]: oauthFields(
    'GitHub OAuth',
    'Iv1.0123456789abcdef',
    'connections:form.fields.scopes.githubHint',
    'user:email',
  ),
  [ConnectionTypes.OIDC]: [
    NAME_FIELD('Acme Workforce OIDC'),
    {
      name: 'clientId',
      labelKey: 'connections:form.fields.clientId.label',
      hintKey: 'connections:form.fields.clientId.hint',
      kind: 'text',
      required: true,
      placeholder: 'acme-console',
    },
    {
      name: 'clientSecret',
      labelKey: 'connections:form.fields.clientSecret.label',
      hintKey: 'connections:form.fields.clientSecret.hint',
      kind: 'secret',
      required: true,
    },
    {
      name: 'authorizationEndpoint',
      labelKey: 'connections:form.fields.authorizationEndpoint.label',
      hintKey: 'connections:form.fields.authorizationEndpoint.hint',
      kind: 'url',
      required: true,
      placeholder: 'https://idp.example.com/authorize',
    },
    {
      name: 'tokenEndpoint',
      labelKey: 'connections:form.fields.tokenEndpoint.label',
      hintKey: 'connections:form.fields.tokenEndpoint.hint',
      kind: 'url',
      required: true,
      placeholder: 'https://idp.example.com/token',
    },
    {
      name: 'issuer',
      labelKey: 'connections:form.fields.issuer.label',
      hintKey: 'connections:form.fields.issuer.hint',
      kind: 'url',
      placeholder: 'https://idp.example.com',
      requiredWhen: 'tokenExchangeEnabled',
      visibility: 'edit',
    },
    {
      name: 'userInfoEndpoint',
      labelKey: 'connections:form.fields.userInfoEndpoint.label',
      hintKey: 'connections:form.fields.userInfoEndpoint.hint',
      kind: 'url',
      placeholder: 'https://idp.example.com/userinfo',
      visibility: 'edit',
    },
    {
      name: 'jwksEndpoint',
      labelKey: 'connections:form.fields.jwksEndpoint.label',
      hintKey: 'connections:form.fields.jwksEndpoint.hint',
      kind: 'url',
      placeholder: 'https://idp.example.com/.well-known/jwks.json',
      requiredWhen: 'tokenExchangeEnabled',
      visibility: 'edit',
    },
    {
      name: 'redirectUri',
      labelKey: 'connections:form.fields.redirectUri.label',
      kind: 'readonly-copy',
      visibility: 'edit',
    },
    {
      name: 'scopes',
      labelKey: 'connections:form.fields.scopes.label',
      hintKey: 'connections:form.fields.scopes.hint',
      kind: 'scopes',
      placeholder: 'openid email profile',
      visibility: 'edit',
    },
    PROMPT_FIELD,
    TOKEN_EXCHANGE_ENABLED_FIELD,
    TRUSTED_TOKEN_AUDIENCE_FIELD,
  ],
  [ConnectionTypes.OAUTH]: [
    NAME_FIELD('Acme OAuth2'),
    {
      name: 'clientId',
      labelKey: 'connections:form.fields.clientId.label',
      hintKey: 'connections:form.fields.clientId.hint',
      kind: 'text',
      required: true,
      placeholder: 'acme-console',
    },
    {
      name: 'clientSecret',
      labelKey: 'connections:form.fields.clientSecret.label',
      hintKey: 'connections:form.fields.clientSecret.hint',
      kind: 'secret',
      required: true,
    },
    {
      name: 'authorizationEndpoint',
      labelKey: 'connections:form.fields.authorizationEndpoint.label',
      hintKey: 'connections:form.fields.authorizationEndpoint.hint',
      kind: 'url',
      required: true,
      placeholder: 'https://idp.example.com/authorize',
    },
    {
      name: 'tokenEndpoint',
      labelKey: 'connections:form.fields.tokenEndpoint.label',
      hintKey: 'connections:form.fields.tokenEndpoint.hint',
      kind: 'url',
      required: true,
      placeholder: 'https://idp.example.com/token',
    },
    {
      name: 'userInfoEndpoint',
      labelKey: 'connections:form.fields.userProfileEndpoint.label',
      hintKey: 'connections:form.fields.userProfileEndpoint.hint',
      kind: 'url',
      placeholder: 'https://api.example.com/user',
    },
    {
      name: 'redirectUri',
      labelKey: 'connections:form.fields.redirectUri.label',
      kind: 'readonly-copy',
      visibility: 'edit',
    },
    {
      name: 'scopes',
      labelKey: 'connections:form.fields.scopes.label',
      hintKey: 'connections:form.fields.scopes.hint',
      kind: 'scopes',
      placeholder: 'read:user email',
      visibility: 'edit',
    },
    PROMPT_FIELD,
  ],
  [ConnectionTypes.TWILIO]: [
    NAME_FIELD('Twilio SMS'),
    {
      name: 'accountSid',
      labelKey: 'connections:form.fields.accountSid.label',
      hintKey: 'connections:form.fields.accountSid.hint',
      kind: 'text',
      required: true,
      placeholder: 'ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
      pattern: /^AC[0-9a-fA-F]{32}$/,
      patternErrorKey: 'connections:validation.accountSid',
    },
    {
      name: 'authToken',
      labelKey: 'connections:form.fields.authToken.label',
      hintKey: 'connections:form.fields.authToken.hint',
      kind: 'secret',
      required: true,
    },
    {
      name: 'senderId',
      labelKey: 'connections:form.fields.senderId.label',
      hintKey: 'connections:form.fields.senderId.hint',
      kind: 'text',
      required: true,
      placeholder: '+15005550006',
    },
  ],
  [ConnectionTypes.VONAGE]: [
    NAME_FIELD('Vonage SMS'),
    {
      name: 'apiKey',
      labelKey: 'connections:form.fields.apiKey.label',
      hintKey: 'connections:form.fields.apiKey.hint',
      kind: 'text',
      required: true,
      placeholder: 'a1b2c3d4',
    },
    {
      name: 'apiSecret',
      labelKey: 'connections:form.fields.apiSecret.label',
      hintKey: 'connections:form.fields.apiSecret.hint',
      kind: 'secret',
      required: true,
    },
    {
      name: 'senderId',
      labelKey: 'connections:form.fields.senderId.label',
      hintKey: 'connections:form.fields.senderId.hint',
      kind: 'text',
      required: true,
      placeholder: '+15005550006',
    },
  ],
  [ConnectionTypes.SMS_GATEWAY]: [
    NAME_FIELD('Custom SMS Sender'),
    {
      name: 'url',
      labelKey: 'connections:form.fields.smsGatewayUrl.label',
      hintKey: 'connections:form.fields.smsGatewayUrl.hint',
      kind: 'url',
      required: true,
      placeholder: 'https://sms.example.com/send',
    },
    {
      name: 'httpMethod',
      labelKey: 'connections:form.fields.httpMethod.label',
      hintKey: 'connections:form.fields.httpMethod.hint',
      kind: 'select',
      defaultValue: 'POST',
      options: [
        {value: 'POST', label: 'POST'},
        {value: 'GET', label: 'GET'},
      ],
    },
    {
      name: 'contentType',
      labelKey: 'connections:form.fields.contentType.label',
      hintKey: 'connections:form.fields.contentType.hint',
      kind: 'select',
      defaultValue: 'JSON',
      options: [
        {value: 'JSON', label: 'JSON'},
        {value: 'FORM', label: 'FORM'},
      ],
    },
    {
      name: 'httpHeaders',
      labelKey: 'connections:form.fields.httpHeaders.label',
      hintKey: 'connections:form.fields.httpHeaders.hint',
      kind: 'key-value',
      placeholder: 'X-API-Key',
      addLabelKey: 'connections:form.fields.httpHeaders.add',
    },
  ],
};

/**
 * Field definitions rendered (and validated) for the given form mode. Fields hidden at create
 * are still passed to the payload mapper via {@link CONNECTION_FORM_FIELDS} directly, so derived
 * values (e.g. redirectUri) are still sent and empty optional values are still omitted.
 */
export function fieldsForMode(type: ConnectionType, mode: 'create' | 'edit'): ConnectionFieldDef[] {
  return CONNECTION_FORM_FIELDS[type].filter(
    (field) => (field.visibility ?? 'both') === 'both' || field.visibility === mode,
  );
}
