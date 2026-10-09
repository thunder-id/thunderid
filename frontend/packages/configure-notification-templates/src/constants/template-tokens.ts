// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A placeholder token a user can insert into a template's subject or body.
 *
 * @public
 */
export interface TemplateToken {
  /** Human-readable label shown in the catalog, e.g. `Primary color`. */
  label: string;
  /** The exact placeholder inserted into the editor, e.g. `{{design(palette.primary.main)}}`. */
  insert: string;
}

/**
 * Design tokens resolved from the theme at render time. Email body only.
 *
 * Uses the backend `{{design(key)}}` grammar; hardcoded until a design-token catalog API exists.
 *
 * @public
 */
export const DESIGN_TOKENS: readonly TemplateToken[] = [
  {label: 'Primary color', insert: '{{design(palette.primary.main)}}'},
  {label: 'Primary contrast text', insert: '{{design(palette.primary.contrastText)}}'},
  {label: 'Secondary color', insert: '{{design(palette.secondary.main)}}'},
  {label: 'Text primary', insert: '{{design(palette.text.primary)}}'},
  {label: 'Background default', insert: '{{design(palette.background.default)}}'},
  {label: 'Background paper', insert: '{{design(palette.background.paper)}}'},
];

/**
 * Context tokens substituted with runtime values when a notification is sent. Both channels.
 *
 * Hardcoded here until a context-placeholder catalog API exists.
 *
 * @public
 */
export const CONTEXT_TOKENS: readonly TemplateToken[] = [
  {label: 'App name', insert: '{{ctx(app.name)}}'},
  {label: 'OTP code', insert: '{{ctx(otp.code)}}'},
  {label: 'User first name', insert: '{{ctx(user.profile.name.first)}}'},
  {label: 'User email address', insert: '{{ctx(user.profile.email.primary.address)}}'},
  {label: 'Expiry duration (minutes)', insert: '{{ctx(notification.expiry.duration.minutes)}}'},
  {label: 'Request city', insert: '{{ctx(request.device.location.city)}}'},
];
