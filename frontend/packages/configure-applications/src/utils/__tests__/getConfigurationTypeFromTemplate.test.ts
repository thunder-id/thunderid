// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {ApplicationCreateFlowConfiguration} from '../../models/application-create-flow';
import getConfigurationTypeFromTemplate from '../getConfigurationTypeFromTemplate';

const makeTemplate = (name: string, redirectUris?: string[]) => ({
  defaults: {
    name,
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          grantTypes: ['authorization_code'],
          responseTypes: ['code'],
          redirectUris: redirectUris ?? [],
        },
      },
    ],
  },
});

describe('getConfigurationTypeFromTemplate', () => {
  it('returns NONE for null template config', () => {
    expect(getConfigurationTypeFromTemplate(null)).toBe(ApplicationCreateFlowConfiguration.NONE);
  });

  it('returns NONE when redirectUris is already populated', () => {
    const template = makeTemplate('Browser App', ['https://example.com/callback']);

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.NONE);
  });

  it('returns DEEPLINK for mobile applications', () => {
    const template = makeTemplate('Mobile App');

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.DEEPLINK);
  });

  it('returns URL for browser applications', () => {
    const template = makeTemplate('Browser Application');

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.URL);
  });

  it('returns URL for server applications', () => {
    const template = makeTemplate('Server Application');

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.URL);
  });

  it('returns NONE for backend applications', () => {
    const template = makeTemplate('Backend Application');

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.NONE);
  });

  it('returns URL as default for unknown application types', () => {
    const template = makeTemplate('Unknown App');

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.URL);
  });

  it('returns URL as default for an empty template with no defaults', () => {
    expect(getConfigurationTypeFromTemplate({})).toBe(ApplicationCreateFlowConfiguration.URL);
  });

  it('handles case-insensitive template names', () => {
    const template = makeTemplate('MOBILE APPLICATION');

    expect(getConfigurationTypeFromTemplate(template)).toBe(ApplicationCreateFlowConfiguration.DEEPLINK);
  });
});
