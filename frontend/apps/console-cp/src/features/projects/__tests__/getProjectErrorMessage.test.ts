// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import getProjectErrorMessage from '../getProjectErrorMessage';

/** Stands in for i18next: known keys translate, anything else falls back to its default value. */
function translator(messages: Record<string, string> = {}) {
  return (key: string, options?: Record<string, unknown>): string => {
    const template = messages[key] ?? (options?.defaultValue as string | undefined) ?? key;
    return template.replace(/{{(\w+)}}/g, (_match, name: string) => (options?.[name] as string | undefined) ?? '');
  };
}

function apiError(data: unknown): Error {
  return Object.assign(new Error('Request failed'), {response: {data}});
}

const OU_HAS_CHILDREN = {
  code: 'OU-1006',
  message: {
    key: 'error.ouservice.organization_unit_has_children',
    defaultValue: 'Organization unit has children',
  },
  description: {
    key: 'error.ouservice.organization_unit_has_children_description',
    defaultValue:
      'Cannot delete an organization unit that has child organization units, users, groups, user types, agent types, connections, or sharing policies it issued',
  },
};

describe('getProjectErrorMessage', () => {
  it('shows the description the API sent', () => {
    expect(getProjectErrorMessage(apiError(OU_HAS_CHILDREN), translator(), 'fallback', 'Failed')).toBe(
      OU_HAS_CHILDREN.description.defaultValue,
    );
  });

  it('prefers a console message for the error code', () => {
    const t = translator({'common:errors.OU-1006': 'Remove what is in the project first.'});

    expect(getProjectErrorMessage(apiError(OU_HAS_CHILDREN), t, 'fallback', 'Failed')).toBe(
      'Remove what is in the project first.',
    );
  });

  it('translates the description by its key and fills in its params', () => {
    const t = translator({'error.projectservice.in_use': 'Used by {{count}} applications'});
    const error = apiError({
      code: 'PRJ-1010',
      message: 'In use',
      description: {key: 'error.projectservice.in_use', defaultValue: 'In use', params: {count: '3'}},
    });

    expect(getProjectErrorMessage(error, t, 'fallback', 'Failed')).toBe('Used by 3 applications');
  });

  it('shows a plain string description as it is', () => {
    const error = apiError({code: 'PRJ-1001', message: 'Bad request', description: 'Handle is invalid'});

    expect(getProjectErrorMessage(error, translator(), 'fallback', 'Failed')).toBe('Handle is invalid');
  });

  it('falls back to the given message when the response carries no reason', () => {
    expect(getProjectErrorMessage(new Error('Network Error'), translator(), 'fallback', 'Failed')).toBe('Failed');
  });
});
