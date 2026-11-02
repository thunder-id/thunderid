// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import OpenID4VPProperties from '../OpenID4VPProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

vi.mock('../../../../../hooks/useResourceFieldError', () => ({
  default: (): string => '',
}));

vi.mock('../../../PresentationDefinitionSelect', () => ({
  default: ({value, onChange}: {value: string; onChange: (value: string) => void}) => (
    <input aria-label="Presentation definition" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

describe('OpenID4VPProperties', () => {
  const mockOnChange = vi.fn();

  const createResource = (properties: Record<string, unknown> = {}): Resource =>
    ({data: {properties}}) as unknown as Resource;

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and the allow-without-local-user checkbox', () => {
    render(<OpenID4VPProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByText('flows:core.executions.openid4vp.description')).toBeInTheDocument();
    expect(screen.getByText('Allow authentication without a local user')).toBeInTheDocument();
  });

  it('should default the presentation definition to empty when the node has no value configured', () => {
    render(<OpenID4VPProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Presentation definition')).toHaveValue('');
  });

  it('should render the configured presentation definition id', () => {
    render(
      <OpenID4VPProperties
        resource={createResource({presentation_definition_id: 'eudi-pid'})}
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByLabelText('Presentation definition')).toHaveValue('eudi-pid');
  });

  it('should toggle allowAuthenticationWithoutLocalUser', async () => {
    const user = userEvent.setup();
    render(<OpenID4VPProperties resource={createResource()} onChange={mockOnChange} />);

    await user.click(screen.getByLabelText('Allow authentication without a local user'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data.properties.allowAuthenticationWithoutLocalUser',
      true,
      expect.anything(),
    );
  });
});
