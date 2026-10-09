// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import CopyableTextAdapter, {type CopyableTextElement} from '../CopyableTextAdapter';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

describe('CopyableTextAdapter', () => {
  const createResource = (source?: string): CopyableTextElement =>
    ({
      id: 'copyable-1',
      label: 'Invite link',
      ...(source !== undefined ? {source} : {}),
    }) as unknown as CopyableTextElement;

  it('should render the label and the bound source key', () => {
    render(<CopyableTextAdapter resource={createResource('inviteLink')} />);

    expect(screen.getByText('Invite link')).toBeInTheDocument();
    expect(screen.getByText('{{inviteLink}}')).toBeInTheDocument();
  });

  it('should indicate when no source is bound', () => {
    render(<CopyableTextAdapter resource={createResource()} />);

    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });

  it('should render without a resource', () => {
    render(<CopyableTextAdapter />);

    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });
});
