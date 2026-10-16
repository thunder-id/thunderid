// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {Element as FlowElement} from '../../../../../../models/elements';
import UserSelectAdapter from '../UserSelectAdapter';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (value: string | undefined) => value,
  }),
}));

vi.mock('../../../hint', () => ({
  Hint: ({hint}: {hint: string}) => <span data-testid="hint">{hint}</span>,
}));

const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
  ({
    id: 'user-select-1',
    type: 'USER_SELECT',
    category: 'FIELD',
    config: {},
    label: 'Owner',
    placeholder: 'Select a user',
    ...overrides,
  }) as FlowElement;

describe('UserSelectAdapter', () => {
  it('should render the same preview as a SELECT: label, placeholder and hint', () => {
    render(<UserSelectAdapter resource={createMockElement({hint: 'Who owns the agent'})} />);

    expect(screen.getByText('Owner')).toBeInTheDocument();
    expect(screen.getByText('Select a user')).toBeInTheDocument();
    expect(screen.getByTestId('hint')).toHaveTextContent('Who owns the agent');
  });

  it('should mark the field as required when it is required', () => {
    const {container} = render(<UserSelectAdapter resource={createMockElement({required: true})} />);

    expect(container.querySelector('.MuiFormLabel-asterisk')).toBeInTheDocument();
  });

  it('should say that the entries are the system users loaded page by page', () => {
    render(<UserSelectAdapter resource={createMockElement()} />);

    expect(screen.getByText('flows:core.elements.userSelect.caption')).toBeInTheDocument();
  });
});
