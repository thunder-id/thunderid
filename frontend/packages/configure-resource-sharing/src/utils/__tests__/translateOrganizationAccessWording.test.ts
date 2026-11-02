// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import translateOrganizationAccessWording, {
  organizationAccessWordingKey,
  type OrganizationAccessTranslateFn,
} from '../translateOrganizationAccessWording';

// A translation function over a fixed catalog that returns the default for a key it does not hold.
const catalog: Record<string, string> = {
  'resourceSharing:who.title': 'Who can use {{resource}}',
  'users:organizationAccess.title': 'Where {{resource}} is affiliated',
};
const t: OrganizationAccessTranslateFn = (key, options = {}) =>
  (catalog[key] ?? String(options.defaultValue)).replace(/{{(\w+)}}/g, (_match, name: string) =>
    typeof options[name] === 'string' ? options[name] : '',
  );

describe('translateOrganizationAccessWording', () => {
  it('uses the generic sentence when the resource has no wording namespace', () => {
    expect(translateOrganizationAccessWording(t, undefined, 'title', {resource: 'Storefront'})).toBe(
      'Who can use Storefront',
    );
  });

  it("uses the resource's own sentence when its namespace sets one", () => {
    expect(translateOrganizationAccessWording(t, 'users', 'title', {resource: 'Alice'})).toBe(
      'Where Alice is affiliated',
    );
  });

  it('falls back to the generic sentence for a key the namespace leaves out', () => {
    expect(translateOrganizationAccessWording(t, 'users', 'stopSharingTitle', {resource: 'Alice'})).toBe(
      'Stop sharing Alice?',
    );
  });

  it("names the resource's own key, or the generic one without a namespace", () => {
    expect(organizationAccessWordingKey('users', 'loadFailedMessage')).toBe(
      'users:organizationAccess.loadFailedMessage',
    );
    expect(organizationAccessWordingKey(undefined, 'loadFailedMessage')).toBe('resourceSharing:loadError.message');
  });
});
