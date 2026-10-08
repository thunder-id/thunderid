// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import getBackchannelLogoutUriServerError from '../getBackchannelLogoutUriServerError';

const t = (key: string, options?: Record<string, unknown>): string =>
  typeof options?.['defaultValue'] === 'string' ? `${key}|${options['defaultValue']}` : key;

function apiError(descriptionKey?: string): Error {
  return {
    response: {
      data: {
        code: 'APP-1024',
        description: descriptionKey ? {key: descriptionKey, defaultValue: 'server text'} : undefined,
      },
    },
  } as unknown as Error;
}

describe('getBackchannelLogoutUriServerError', () => {
  it.each([
    ['error.applicationservice.backchannel_logout_uri_private_host_description', 'privateHost'],
    ['error.agentservice.backchannel_logout_uri_private_host_description', 'privateHost'],
    ['error.applicationservice.backchannel_logout_uri_requires_https_description', 'serverRequiresHttps'],
    ['error.agentservice.backchannel_logout_uri_requires_https_description', 'serverRequiresHttps'],
    ['error.applicationservice.invalid_backchannel_logout_uri_description', 'serverInvalid'],
    ['error.agentservice.invalid_backchannel_logout_uri_description', 'serverInvalid'],
  ])('maps %s to the %s message', (descriptionKey, expectedKey) => {
    const message = getBackchannelLogoutUriServerError(apiError(descriptionKey), t);

    expect(message).toContain(`applications:edit.general.backchannelLogoutUri.error.${expectedKey}`);
    expect(message).toContain('The server refused the back-channel logout URI');
  });

  it.each([
    ['another OAuth configuration error', apiError('error.applicationservice.invalid_grant_type_description')],
    ['an error without a description', apiError()],
    ['an error that is not an API response', new Error('network down')],
  ])('returns undefined for %s', (_name, error) => {
    expect(getBackchannelLogoutUriServerError(error, t)).toBeUndefined();
  });
});
