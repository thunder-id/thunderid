// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
  SMS_GATEWAY: 'sms-gateway',
} as const;

export type ConnectionType = (typeof ConnectionTypes)[keyof typeof ConnectionTypes];

/**
 * Presentation categories owned entirely by the frontend (drive filter chips + card tags).
 * `trusted-idp` is synthesized directly from connection instances (not a vendor catalog entry)
 * — see `buildConnectionCards`.
 */
export type ConnectionCategory =
  | 'social-login'
  | 'enterprise'
  | 'sms'
  | 'email'
  | 'identity-verification'
  | 'crm'
  | 'data-store'
  | 'trusted-idp'
  | 'custom';

/**
 * Functional categories served by the backend /connections?category= filter.
 */
export const ConnectionInstanceCategories = {
  IDENTITY_PROVIDER: 'identity-provider',
  SMS_PROVIDER: 'sms-provider',
} as const;

export type ConnectionInstanceCategory =
  (typeof ConnectionInstanceCategories)[keyof typeof ConnectionInstanceCategories];

/**
 * One entry of GET /connections — a configured connection instance.
 */
export interface ConnectionInstance {
  id: string;
  name: string;
  description?: string;
  type: ConnectionType;
  categories: ConnectionInstanceCategory[];
  /**
   * Present only for trust-only OIDC instances (trusted issuers); absent for plain federation
   * OIDC connections. See `buildConnectionCards`, which uses this to render trusted issuers as
   * their own card variant.
   */
  idJagEnabled?: boolean;
}

/**
 * A pagination link on a list response.
 */
export interface ConnectionListLink {
  href: string;
  rel: string;
}

/**
 * Paginated response of GET /connections.
 */
export interface ConnectionListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  connections: ConnectionInstance[];
  links: ConnectionListLink[];
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
 * Resolves which local user type a federated identity maps to (selecting its attribute-mapping
 * profile). `default` is the fixed fallback type. When `externalAttribute` and `valueMapping` are
 * set, the type is derived from the
 * value of that external attribute (`valueMapping` maps an external value to a local user type),
 * falling back to `default`.
 */
export interface UserTypeResolution {
  default: string;
  externalAttribute?: string;
  valueMapping?: Record<string, string>;
}

/**
 * Attribute mapping profile for a single local user type.
 */
export interface UserTypeAttributeMapping {
  userType: string;
  attributes: AttributeMapping[];
}

/**
 * Resolves a returning federated identity to an existing local account when its subject identifier
 * does not match an existing local subject. The listed external attributes are matched together (AND)
 * to identify a unique account.
 */
export interface AccountLinking {
  attributes: string[];
}

/**
 * External-to-local attribute mapping configuration for an authentication provider.
 */
export interface AttributeConfiguration {
  userTypeResolution: UserTypeResolution;
  userTypeAttributeMappings?: UserTypeAttributeMapping[];
  accountLinking?: AccountLinking;
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
  issuer?: string;
  tokenExchangeEnabled?: boolean;
  trustedTokenAudience?: string;
  /** Whether this connection is exposed as an ID-JAG issuer. Managed by the Trusted Issuers feature. */
  idJagEnabled?: boolean;
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
 * Request payload for generic OAuth 2 connections — no OpenID Connect discovery and no id_token, so
 * user attributes come from the provider's own profile API (userInfoEndpoint). It is optional:
 * providers without one carry the subject in a JWT access token.
 */
export interface OAuth2ConnectionRequest extends OAuthConnectionRequest {
  authorizationEndpoint: string;
  tokenEndpoint: string;
  userInfoEndpoint?: string;
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

/**
 * Request payload for a generic HTTP SMS gateway connection — a webhook ThunderID calls to
 * deliver the message, for SMS providers without a dedicated vendor integration.
 */
export interface SMSGatewayConnectionRequest {
  name: string;
  description?: string;
  /** The HTTP endpoint called to send an SMS. */
  url: string;
  httpMethod: string;
  contentType: string;
  /** Comma-separated "Key: value" pairs sent with every request. */
  httpHeaders?: string;
}

export type ConnectionRequest =
  | OAuthConnectionRequest
  | OIDCConnectionRequest
  | OAuth2ConnectionRequest
  | TwilioConnectionRequest
  | VonageConnectionRequest
  | SMSGatewayConnectionRequest;

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
  /** SMS gateway fields. */
  url?: string;
  httpMethod?: string;
  contentType?: string;
  httpHeaders?: string;
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
  /** Stable map key (matches backendType for real vendors, e.g. "google"). */
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
  /** i18n key for the create-wizard setup hint (vendors that need an OAuth app registered first). */
  createHintKey?: string;
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
