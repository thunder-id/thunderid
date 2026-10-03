// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import KeyValueListAdapter, {type KeyValueListElement} from '../KeyValueListAdapter';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

describe('KeyValueListAdapter', () => {
  const createResource = (source?: string): KeyValueListElement =>
    ({
      id: 'key-value-list-1',
      label: 'Matched account',
      ...(source !== undefined ? {source} : {}),
    }) as unknown as KeyValueListElement;

  it('should render the label and the bound source key', () => {
    render(<KeyValueListAdapter resource={createResource('linkingPromptDetails')} />);

    expect(screen.getByText('Matched account')).toBeInTheDocument();
    expect(screen.getByText('{{linkingPromptDetails}}')).toBeInTheDocument();
  });

  it('should indicate when no source is bound', () => {
    render(<KeyValueListAdapter resource={createResource()} />);

    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });

  it('should render without a resource', () => {
    render(<KeyValueListAdapter />);

    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });
});
