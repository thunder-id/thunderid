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

import type {JSX} from 'react';

/**
 * Backend connection types served by the /connections API.
 */
export const ConnectionTypes = {
  GOOGLE: 'google',
  GITHUB: 'github',
  OIDC: 'oidc',
  OAUTH: 'oauth',
  TWILIO: 'twilio',
  VONAGE: 'vonage',
} as const;

export type ConnectionType = (typeof ConnectionTypes)[keyof typeof ConnectionTypes];

/**
 * Presentation categories owned entirely by the frontend (drive filter chips + card tags).
 */
export type ConnectionCategory =
  | 'social-login'
  | 'enterprise'
  | 'sms'
  | 'email'
  | 'identity-verification'
  | 'crm'
  | 'data-store';

/**
 * One entry of GET /connections — structural data only.
 */
export interface ConnectionTypeSummary {
  type: ConnectionType;
  configured: boolean;
  instanceCount: number;
}

export interface ConnectionListResponse {
  connections: ConnectionTypeSummary[];
}

/**
 * A single resource that references a connection (e.g. a flow that uses it).
 */
export interface ConnectionUsage {
  resourceType: string;
  id: string;
  displayName: string;
  behaviorOnDelete: 'fallback' | 'cascade' | 'restrict';
}

/**
 * Response for the connection usages endpoint (GET /connections/{type}/{id}/usages).
 * totalResults is null when usage data is unavailable; 0 means confirmed empty.
 */
export interface ConnectionUsagesResponse {
  totalResults: number | null;
  count: number;
  summary: Record<string, number> | null;
  usages: ConnectionUsage[];
}

/**
 * Lightweight configured instance (GET /connections/{type}).
 */
export interface ConnectionInstanceSummary {
  id: string;
  name: string;
  description?: string;
}

/**
 * Maps a single external IdP claim to a local user attribute. `externalAttribute` may be a
 * dot-notation path into a nested claim (e.g. "address.email").
 */
export interface AttributeMapping {
  externalAttribute: string;
  localAttribute: string;
}

/**
 * Resolves which local user type a federated identity is provisioned as.
 */
export interface UserTypeResolution {
  default: string;
}

/**
 * Attribute mapping profile for a single local user type.
 */
export interface UserTypeAttributeMapping {
  userType: string;
  attributes: AttributeMapping[];
}

/**
 * External-to-local attribute mapping configuration for an authentication provider.
 */
export interface AttributeConfiguration {
  userTypeResolution: UserTypeResolution;
  userTypeAttributeMappings?: UserTypeAttributeMapping[];
}

/**
 * Request payload shared by google/github connections.
 */
export interface OAuthConnectionRequest {
  name: string;
  description?: string;
  clientId: string;
  /** Write-only. Omit to keep the stored value on update; required when creating. */
  clientSecret?: string;
  redirectUri: string;
  scopes?: string[];
  prompt?: string;
  /** External-to-local attribute mapping (authentication providers only). */
  attributeConfiguration?: AttributeConfiguration;
}

/**
 * Request payload for oidc connections — adds endpoint configuration.
 */
export interface OIDCConnectionRequest extends OAuthConnectionRequest {
  authorizationEndpoint: string;
  tokenEndpoint: string;
  userInfoEndpoint?: string;
  jwksEndpoint?: string;
  logoutEndpoint?: string;
  issuer?: string;
  tokenExchangeEnabled?: boolean;
  trustedTokenAudience?: string;
}

/**
 * Request payload for a Twilio SMS connection.
 */
export interface TwilioConnectionRequest {
  name: string;
  description?: string;
  accountSid: string;
  /** Write-only. Omit to keep the stored value on update; required when creating. */
  authToken?: string;
  senderId: string;
}

/**
 * Request payload for generic OAuth 2.0 connections — no OpenID Connect discovery and no
 * id_token, so the user profile is always fetched from userInfoEndpoint (required, unlike OIDC).
 */
export interface OAuth2ConnectionRequest extends OAuthConnectionRequest {
  authorizationEndpoint: string;
  tokenEndpoint: string;
  userInfoEndpoint: string;
  logoutEndpoint?: string;
}

/**
 * Request payload for a Vonage SMS connection.
 */
export interface VonageConnectionRequest {
  name: string;
  description?: string;
  apiKey: string;
  /** Write-only. Omit to keep the stored value on update; required when creating. */
  apiSecret?: string;
  senderId: string;
}

export type ConnectionRequest =
  | OAuthConnectionRequest
  | OIDCConnectionRequest
  | OAuth2ConnectionRequest
  | TwilioConnectionRequest
  | VonageConnectionRequest;

/**
 * Vendor response — secrets returned masked as "******". A superset carrying every vendor's
 * fields (IdP + SMS); the shared form mapping reads only the fields relevant to each type.
 */
export interface ConnectionResponse extends OIDCConnectionRequest {
  id: string;
  type: ConnectionType;
  /** SMS (Twilio) fields. */
  accountSid?: string;
  authToken?: string;
  /** SMS (Vonage) fields. */
  apiKey?: string;
  apiSecret?: string;
  /** SMS (shared) field. */
  senderId?: string;
}

/**
 * Where a vendor sits in the catalog.
 * - branded: a real catalog tile backed by a connection type (always visible).
 * - custom: backed by a connection type but configured only through the wizard; each
 *   instance renders as its own card (not a catalog tile).
 * - coming-soon: a placeholder tile for a not-yet-wired vendor (no API calls).
 */
export type ConnectionPresentation = 'branded' | 'custom' | 'coming-soon';

/**
 * Frontend-owned presentation metadata for a vendor.
 */
export interface ConnectionVendorMeta {
  /** Stable map key (matches backendType for real vendors, e.g. "google", or "custom-sms"). */
  key: string;
  /** The backend /connections type, when this vendor maps to one. */
  backendType?: ConnectionType;
  displayName: string;
  descriptionKey: string;
  logo: JSX.Element;
  categories: ConnectionCategory[];
  presentation: ConnectionPresentation;
  comingSoon?: boolean;
  /** Whether this connection provisions users and therefore exposes attribute mapping (IdPs only). */
  supportsAttributeMapping?: boolean;
}

/**
 * A single card the listing grid renders, after merging summaries + meta + instances.
 */
export interface ConnectionCardModel {
  /** Unique React key (vendor key, or vendor key + instance id for custom cards). */
  id: string;
  vendorKey: string;
  backendType?: ConnectionType;
  displayName: string;
  descriptionKey: string;
  logo: JSX.Element;
  categories: ConnectionCategory[];
  status: 'configured' | 'not-configured';
  comingSoon: boolean;
  /** Route to navigate to when the card is activated; null for coming-soon. */
  navTarget: string | null;
}
