// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import ProvisioningProperties from '../ProvisioningProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.provisioning.description': 'Configure just-in-time provisioning.',
        'flows:core.executions.federation.allowCrossOUProvisioning.label': 'Allow cross-OU provisioning',
        'flows:core.executions.federation.allowCrossOUProvisioning.hint': 'Provision users into another OU.',
        'flows:core.executions.provisioning.includeOptional.label': 'Include optional attributes',
        'flows:core.executions.provisioning.includeOptional.hint': 'Include optional attributes hint.',
        'flows:core.executions.provisioning.includeOptionalCredentials.label': 'Include optional credentials',
        'flows:core.executions.provisioning.includeOptionalCredentials.hint': 'Include optional credentials hint.',
        'flows:core.executions.provisioning.maxPerPrompt.label': 'Max attributes per prompt',
        'flows:core.executions.provisioning.maxPerPrompt.placeholder': '3',
        'flows:core.executions.provisioning.maxPerPrompt.hint': 'Maximum attributes shown per prompt.',
        'flows:core.executions.provisioning.assignGroup.label': 'Assign group',
        'flows:core.executions.provisioning.assignGroup.placeholder': 'group-id',
        'flows:core.executions.provisioning.assignRole.label': 'Assign role',
        'flows:core.executions.provisioning.assignRole.placeholder': 'role-id',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('ProvisioningProperties', () => {
  const mockOnChange = vi.fn();

  const createResource = (properties: Record<string, unknown> = {}): Resource =>
    ({data: {properties}}) as unknown as Resource;

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and checkbox labels', () => {
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Configure just-in-time provisioning.')).toBeInTheDocument();
    expect(screen.getByText('Allow cross-OU provisioning')).toBeInTheDocument();
  });

  it('should default maxPerPrompt to 0 when the node has no value configured', () => {
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Max attributes per prompt')).toHaveValue(0);
  });

  it('should coerce a string maxPerPrompt value to a number', () => {
    render(<ProvisioningProperties resource={createResource({maxPerPrompt: '5'})} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Max attributes per prompt')).toHaveValue(5);
  });

  it('should render the configured numeric maxPerPrompt value as-is', () => {
    render(<ProvisioningProperties resource={createResource({maxPerPrompt: 7})} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Max attributes per prompt')).toHaveValue(7);
  });

  it('should toggle the allowCrossOUProvisioning checkbox', async () => {
    const user = userEvent.setup();
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    await user.click(screen.getByLabelText('Allow cross-OU provisioning'));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.allowCrossOUProvisioning', true, expect.anything());
  });

  it('should commit a new assignGroup value on blur', () => {
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    const input = screen.getByLabelText('Assign group');
    fireEvent.change(input, {target: {value: 'engineering'}});
    fireEvent.blur(input);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.assignGroup', 'engineering', expect.anything());
  });
});
