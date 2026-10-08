// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The subset of an API error response this util reads.
 */
interface ApiErrorWithDescriptionKey {
  description?: {key?: string};
}

// The server reports these refusals under its generic invalid OAuth configuration code, so only the
// description key identifies them. The keys are the same for applications and agents apart from
// the service prefix.
const FIELD_DESCRIPTION_KEY_PART = 'backchannel_logout_uri';
const PRIVATE_HOST_SUFFIX = 'backchannel_logout_uri_private_host_description';
const REQUIRES_HTTPS_SUFFIX = 'backchannel_logout_uri_requires_https_description';

/**
 * Returns a message that names the back-channel logout URI field when the server refused a save
 * because of that field, and `undefined` for any other error. The Console validates the URI itself,
 * but the server can still refuse it: a localhost or private network address depends on server
 * configuration, and the two parsers can disagree on an edge case.
 *
 * @param error - The error thrown by the mutation
 * @param t - A translation function that forwards an explicit `ns:` prefix unchanged
 * @returns The localized message, or `undefined` when the error is not about this field
 *
 * @public
 */
export default function getBackchannelLogoutUriServerError(
  error: Error,
  t: (key: string, options?: Record<string, unknown>) => string,
): string | undefined {
  const descriptionKey = (error as {response?: {data?: ApiErrorWithDescriptionKey}}).response?.data?.description?.key;

  if (!descriptionKey?.includes(FIELD_DESCRIPTION_KEY_PART)) {
    return undefined;
  }
  if (descriptionKey.endsWith(PRIVATE_HOST_SUFFIX)) {
    return t('applications:edit.general.backchannelLogoutUri.error.privateHost', {
      defaultValue:
        'The server refused the back-channel logout URI because it points to localhost or a private network address. Use a publicly reachable address.',
    });
  }
  if (descriptionKey.endsWith(REQUIRES_HTTPS_SUFFIX)) {
    return t('applications:edit.general.backchannelLogoutUri.error.serverRequiresHttps', {
      defaultValue: 'The server refused the back-channel logout URI because a public client must use an https URL.',
    });
  }
  return t('applications:edit.general.backchannelLogoutUri.error.serverInvalid', {
    defaultValue:
      'The server refused the back-channel logout URI. Enter an absolute http or https URL with a host, and no user info, fragment, or wildcard.',
  });
}
