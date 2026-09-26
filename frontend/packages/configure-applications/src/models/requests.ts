// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Application} from '@thunderid/configure-applications';
/**
 * Application Request Model
 *
 * Data structure used when creating or updating an application.
 * This model is used for POST and PUT operations to the /applications endpoint.
 *
 * @public
 * @remarks
 * Applications in the product represent OAuth2/OIDC client applications that can
 * authenticate users and access protected resources. Each application can be
 * configured with:
 * - Basic metadata (name, description, logo, URLs)
 * - Authentication and registration flows
 * - OAuth2/OIDC inbound authentication settings
 * - User attributes to include in tokens
 *
 * The server will generate additional fields (id, clientId, timestamps) upon creation.
 *
 * @example
 * ```typescript
 * // Create a basic web application with OAuth2 authentication
 * const createWebApp: CreateApplicationRequest = {
 *   name: 'My Web Application',
 *   description: 'Customer portal application',
 *   url: 'https://myapp.com',
 *   logoUrl: 'https://myapp.com/logo.png',
 *   tosUri: 'https://myapp.com/terms',
 *   policyUri: 'https://myapp.com/privacy',
 *   contacts: ['admin@myapp.com', 'support@myapp.com'],
 *   authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
 *   registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
 *   isRegistrationFlowEnabled: true,
 *   userAttributes: ['email', 'username', 'roles'],
 *   inboundAuthConfig: [{
 *     type: 'oauth2',
 *     config: {
 *       redirectUris: ['https://myapp.com/callback'],
 *       grantTypes: ['authorization_code', 'refresh_token'],
 *       responseTypes: ['code'],
 *       scopes: ['openid', 'profile', 'email'],
 *       token: {
 *         accessToken: {
 *           userConfig: {
 *             validityPeriod: 3600,
 *             attributes: ['email', 'username']
 *           }
 *         },
 *         idToken: {
 *           validityPeriod: 3600,
 *           userAttributes: ['sub', 'email', 'name'],
 *           scopeClaims: {
 *             profile: ['name', 'picture'],
 *             email: ['email', 'email_verified']
 *           }
 *         }
 *       }
 *     }
 *   }]
 * };
 * ```
 *
 * @example
 * ```typescript
 * // Create a minimal SPA application
 * const createSPA: CreateApplicationRequest = {
 *   name: 'My SPA',
 *   url: 'http://localhost:3000',
 *   inboundAuthConfig: [{
 *     type: 'oauth2',
 *     config: {
 *       redirectUris: ['http://localhost:3000/callback'],
 *       grantTypes: ['authorization_code', 'refresh_token'],
 *       responseTypes: ['code'],
 *       pkceRequired: true,
 *       publicClient: true,
 *       scopes: ['openid', 'profile'],
 *       token: {
 *         accessToken: { userConfig: { validityPeriod: 3600, attributes: [] } },
 *         idToken: { validityPeriod: 3600, userAttributes: ['sub'], scopeClaims: {} }
 *       }
 *     }
 *   }]
 * };
 * ```
 */
export type CreateApplicationRequest = Omit<Application, 'id' | 'createdAt' | 'updatedAt'>;
