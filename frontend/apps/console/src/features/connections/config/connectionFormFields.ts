/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {type ConnectionType, ConnectionTypes} from '../models/connection';

export type ConnectionFieldKind = 'text' | 'url' | 'secret' | 'scopes' | 'readonly-copy';

export interface ConnectionFieldDef {
  /** Request payload property this field maps to. */
  name: string;
  /** i18n key under the connections namespace. */
  labelKey: string;
  hintKey?: string;
  kind: ConnectionFieldKind;
  /** Required on create. Secret fields are required on create but optional (omit-to-keep) on edit. */
  required?: boolean;
  /** Render an "Optional" tag next to the label. */
  optional?: boolean;
  placeholder?: string;
}

const NAME_FIELD = (placeholder: string): ConnectionFieldDef => ({
  name: 'name',
  labelKey: 'connections:form.fields.name.label',
  hintKey: 'connections:form.fields.name.hint',
  kind: 'text',
  required: true,
  placeholder,
});

const oauthFields = (namePlaceholder: string, clientIdPlaceholder: string): ConnectionFieldDef[] => [
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
  {name: 'redirectUri', labelKey: 'connections:form.fields.redirectUri.label', kind: 'readonly-copy'},
  {
    name: 'scopes',
    labelKey: 'connections:form.fields.scopes.label',
    hintKey: 'connections:form.fields.scopes.hint',
    kind: 'scopes',
    placeholder: 'openid email profile',
  },
];

/**
 * Ordered field definitions per connection type, driving the shared {@link ConnectionForm}
 * and its dynamically-built validation schema.
 */
export const CONNECTION_FORM_FIELDS: Record<ConnectionType, ConnectionFieldDef[]> = {
  [ConnectionTypes.GOOGLE]: oauthFields('Google Workspace', '1234567890-abc.apps.googleusercontent.com'),
  [ConnectionTypes.GITHUB]: oauthFields('GitHub OAuth', 'Iv1.0123456789abcdef'),
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
    },
    {
      name: 'userInfoEndpoint',
      labelKey: 'connections:form.fields.userInfoEndpoint.label',
      hintKey: 'connections:form.fields.userInfoEndpoint.hint',
      kind: 'url',
      optional: true,
      placeholder: 'https://idp.example.com/userinfo',
    },
    {
      name: 'jwksEndpoint',
      labelKey: 'connections:form.fields.jwksEndpoint.label',
      hintKey: 'connections:form.fields.jwksEndpoint.hint',
      kind: 'url',
      optional: true,
      placeholder: 'https://idp.example.com/.well-known/jwks.json',
    },
    {name: 'redirectUri', labelKey: 'connections:form.fields.redirectUri.label', kind: 'readonly-copy'},
    {
      name: 'scopes',
      labelKey: 'connections:form.fields.scopes.label',
      hintKey: 'connections:form.fields.scopes.hint',
      kind: 'scopes',
      placeholder: 'openid email profile',
    },
  ],
};
