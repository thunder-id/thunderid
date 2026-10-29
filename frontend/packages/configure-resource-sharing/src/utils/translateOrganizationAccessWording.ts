// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The translation function, in the shape `useTranslation`'s `t` satisfies.
 */
export type OrganizationAccessTranslateFn = (key: string, options?: Record<string, unknown>) => string;

/**
 * The sentences of the Organization Access screen that depend on what the resource is: a user is
 * affiliated with a unit, an application is available in it. A resource sets its own under
 * `<wordingNamespace>:organizationAccess.<key>`; any key it leaves out falls back to the generic text.
 */
export type OrganizationAccessWordingKey =
  | 'title'
  | 'description'
  | 'ownerOnly'
  | 'stopSharingTitle'
  | 'stopSharingIntro'
  | 'stopSharingConfirm'
  | 'saved'
  | 'loadFailedTitle'
  | 'loadFailedMessage';

/**
 * Values every wording key may use: the resource's display name and the owning unit's name.
 */
export interface OrganizationAccessWordingValues {
  resource?: string;
  owner?: string;
}

/**
 * The generic text each wording key falls back to.
 */
export const ORGANIZATION_ACCESS_WORDING_FALLBACKS: Record<
  OrganizationAccessWordingKey,
  {key: string; defaultValue: string}
> = {
  title: {key: 'resourceSharing:who.title', defaultValue: 'Who can use {{resource}}'},
  description: {
    key: 'resourceSharing:who.description',
    defaultValue: 'Users and agents of a granted organization unit can sign in to {{resource}}.',
  },
  ownerOnly: {
    key: 'resourceSharing:scope.owner.description',
    defaultValue: 'No child unit and no other tree can use it.',
  },
  stopSharingTitle: {key: 'resourceSharing:switch.titleOwner', defaultValue: 'Stop sharing {{resource}}?'},
  stopSharingIntro: {
    key: 'resourceSharing:switch.introOwner',
    defaultValue: 'Only {{owner}} will be able to use {{resource}}. This clears:',
  },
  stopSharingConfirm: {key: 'resourceSharing:switch.confirmOwner', defaultValue: 'Stop sharing'},
  saved: {key: 'resourceSharing:save.success', defaultValue: 'Organization access updated.'},
  loadFailedTitle: {key: 'resourceSharing:loadError.title', defaultValue: 'Failed to load organization access'},
  // Shown through QueryErrorNotice's fallback key, which interpolates nothing, so it takes no values.
  loadFailedMessage: {
    key: 'resourceSharing:loadError.message',
    defaultValue: 'Something went wrong while loading who can use this resource.',
  },
};

/**
 * The i18n key a resource sets for a wording key, or the generic key when the resource has no wording.
 */
export function organizationAccessWordingKey(
  wordingNamespace: string | undefined,
  key: OrganizationAccessWordingKey,
): string {
  return wordingNamespace
    ? `${wordingNamespace}:organizationAccess.${key}`
    : ORGANIZATION_ACCESS_WORDING_FALLBACKS[key].key;
}

/**
 * Translates one wording key: the resource's own sentence when its namespace sets one, the generic
 * sentence otherwise.
 *
 * @param t - The translation function
 * @param wordingNamespace - The i18n namespace holding the resource's wording, e.g. `users`
 * @param key - The sentence to translate
 * @param values - The resource and owner names the sentence may use
 * @returns The translated sentence
 */
export default function translateOrganizationAccessWording(
  t: OrganizationAccessTranslateFn,
  wordingNamespace: string | undefined,
  key: OrganizationAccessWordingKey,
  values: OrganizationAccessWordingValues = {},
): string {
  const fallback = ORGANIZATION_ACCESS_WORDING_FALLBACKS[key];
  const generic = t(fallback.key, {defaultValue: fallback.defaultValue, ...values});
  return wordingNamespace
    ? t(organizationAccessWordingKey(wordingNamespace, key), {defaultValue: generic, ...values})
    : generic;
}
