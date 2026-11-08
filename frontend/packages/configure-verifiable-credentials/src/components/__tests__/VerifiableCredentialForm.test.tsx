// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {VerifiableCredential} from '../../models/vc';
import VerifiableCredentialForm, {type VerifiableCredentialFormProps} from '../VerifiableCredentialForm';

vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: () => null,
  useHasMultipleOUs: () => ({hasMultipleOUs: false, ouList: [{id: 'ou-1'}]}),
}));

const credential: VerifiableCredential = {
  id: 'vc-1',
  handle: 'lounge-pass',
  ouId: 'ou-1',
  ouHandle: 'default',
  name: 'Lounge Pass',
  vct: 'urn:lounge-pass',
  format: 'dc+sd-jwt',
};

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

const props: VerifiableCredentialFormProps = {
  initial: credential,
  name: 'Renamed',
  description: '',
  onNameChange: vi.fn(),
  onDescriptionChange: vi.fn(),
  submitting: false,
  submitLabel: 'Save',
  onSubmit: vi.fn(),
  onDelete: vi.fn(),
};

describe('VerifiableCredentialForm', () => {
  it('offers the Advanced tab and an enabled Save by default', () => {
    render(<VerifiableCredentialForm {...props} />);

    expect(screen.getByRole('tab', {name: 'Advanced'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save'})).toBeEnabled();
  });

  it('omits the Advanced tab and disables Save when the admin may only view', () => {
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <VerifiableCredentialForm {...props} />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('tab', {name: 'Advanced'})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
  });
});
