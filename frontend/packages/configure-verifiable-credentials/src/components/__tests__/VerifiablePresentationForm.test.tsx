// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, afterEach} from 'vitest';
import type {VerifiablePresentation} from '../../models/vp';
import VerifiablePresentationForm, {type VerifiablePresentationFormProps} from '../VerifiablePresentationForm';

vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: () => null,
  useHasMultipleOUs: () => ({hasMultipleOUs: false, ouList: [{id: 'ou-1'}]}),
}));

const PEM = ['-----BEGIN CERTIFICATE-----', 'MIIBszCCAVmgAwIBAgIUQm9ndXNDZXJ0', '-----END CERTIFICATE-----'].join('\n');

const definition: VerifiablePresentation = {
  id: 'vp-1',
  handle: 'eudi-pid',
  ouId: 'ou-1',
  ouHandle: 'default',
  name: 'EUDI PID',
  vct: 'urn:eudi:pid:1',
  format: 'dc+sd-jwt',
  mandatoryClaims: ['given_name'],
  enforceTrustedIssuer: true,
  trustedAuthorities: ['Existing CA'],
};

const onSubmit = vi.fn();

function renderForm(props: Partial<VerifiablePresentationFormProps> = {}) {
  const defaults: VerifiablePresentationFormProps = {
    initial: definition,
    name: definition.name ?? '',
    description: '',
    onNameChange: vi.fn(),
    onDescriptionChange: vi.fn(),
    submitting: false,
    submitLabel: 'Save',
    onSubmit,
  };
  const merged = {...defaults, ...props};
  const result = render(<VerifiablePresentationForm {...merged} />);

  return {
    ...result,
    rerenderWith: (next: Partial<VerifiablePresentationFormProps>) =>
      result.rerender(<VerifiablePresentationForm {...merged} {...next} />),
  };
}

async function openIssuerTrustTab(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(screen.getByRole('tab', {name: /issuer trust/i}));
}

async function addCertificate(
  user: ReturnType<typeof userEvent.setup>,
  certificateName: string,
  certificate: string,
): Promise<void> {
  await user.click(screen.getByRole('button', {name: 'Add certificate'}));
  const dialog = await screen.findByRole('dialog');
  if (certificateName) {
    await user.type(within(dialog).getByLabelText(/name/i), certificateName);
  }
  if (certificate) {
    await user.click(within(dialog).getByLabelText(/^certificate/i));
    await user.paste(certificate);
  }
  await user.click(within(dialog).getByRole('button', {name: 'Add'}));
}

async function addValidCertificate(user: ReturnType<typeof userEvent.setup>, certificateName: string): Promise<void> {
  await addCertificate(user, certificateName, PEM);
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
}

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('VerifiablePresentationForm capabilities', () => {
  it('omits the Advanced tab and disables Save when the admin may only view', () => {
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <VerifiablePresentationForm
          initial={definition}
          name="Renamed"
          description=""
          onNameChange={vi.fn()}
          onDescriptionChange={vi.fn()}
          submitting={false}
          submitLabel="Save"
          onSubmit={onSubmit}
          onDelete={vi.fn()}
        />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('tab', {name: 'Advanced'})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
  });
});

describe('VerifiablePresentationForm issuer trust', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('lists the trusted issuer certificate names of the definition', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    const list = screen.getByRole('list', {name: 'Trusted issuer certificates'});
    expect(within(list).getByText('Existing CA')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Remove certificate Existing CA'})).toBeInTheDocument();
    expect(screen.getByRole('switch', {name: 'Enforce trusted issuer'})).toBeEnabled();
    expect(screen.getByRole('switch', {name: 'Enforce trusted issuer'})).toBeChecked();
  });

  it('disables the enforce toggle while the definition has no certificates', async () => {
    const user = userEvent.setup();
    renderForm({initial: {...definition, enforceTrustedIssuer: false, trustedAuthorities: undefined}});
    await openIssuerTrustTab(user);

    expect(screen.getByRole('switch', {name: 'Enforce trusted issuer'})).toBeDisabled();
    expect(screen.getByText('Add a trusted issuer certificate before enforcing issuer trust.')).toBeInTheDocument();
    expect(screen.getByText('No trusted issuer certificates added yet.')).toBeInTheDocument();
  });

  it('adds a pasted certificate and sends existing entries by name and new ones with their certificate', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    await addValidCertificate(user, 'New CA');

    const list = screen.getByRole('list', {name: 'Trusted issuer certificates'});
    expect(within(list).getByText('New CA')).toBeInTheDocument();
    expect(within(list).getByText('Unsaved')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        enforceTrustedIssuer: true,
        trustedAuthorities: [{name: 'Existing CA'}, {name: 'New CA', certificate: PEM}],
      }),
    );
  });

  it('enables the enforce toggle once a certificate is added', async () => {
    const user = userEvent.setup();
    renderForm({initial: {...definition, enforceTrustedIssuer: false, trustedAuthorities: undefined}});
    await openIssuerTrustTab(user);

    await addValidCertificate(user, 'New CA');

    expect(screen.getByRole('switch', {name: 'Enforce trusted issuer'})).toBeEnabled();
  });

  it('reads an uploaded certificate file into the certificate field', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    await user.click(screen.getByRole('button', {name: 'Add certificate'}));
    const dialog = await screen.findByRole('dialog');
    await user.upload(
      within(dialog).getByTestId('vp-trusted-certificate-file'),
      new File([PEM], 'issuer.pem', {type: 'application/x-pem-file'}),
    );

    await expect.poll(() => within(dialog).getByLabelText(/^certificate/i)).toHaveValue(PEM);
  });

  it('requires a name and a certificate', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    await addCertificate(user, '', '');

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Enter a name for the certificate.')).toBeInTheDocument();
    expect(within(dialog).getByText('Paste or upload a certificate.')).toBeInTheDocument();
  });

  it('rejects a duplicate name and a certificate that is not PEM', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    await addCertificate(user, 'Existing CA', 'not a certificate');

    const dialog = screen.getByRole('dialog');
    expect(
      within(dialog).getByText('A certificate with this name already exists in this presentation definition.'),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        'Enter a single PEM-encoded X.509 certificate, including its BEGIN CERTIFICATE and END CERTIFICATE lines.',
      ),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('removes a certificate and turns enforcement off when the last one is removed', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    await user.click(screen.getByRole('button', {name: 'Remove certificate Existing CA'}));

    expect(screen.queryByRole('list', {name: 'Trusted issuer certificates'})).not.toBeInTheDocument();
    const toggle = screen.getByRole('switch', {name: 'Enforce trusted issuer'});
    expect(toggle).toBeDisabled();
    expect(toggle).not.toBeChecked();

    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({enforceTrustedIssuer: false, trustedAuthorities: undefined}),
    );
  });

  it('restores the initial certificates on reset', async () => {
    const user = userEvent.setup();
    renderForm();
    await openIssuerTrustTab(user);

    await user.click(screen.getByRole('button', {name: 'Remove certificate Existing CA'}));
    await addValidCertificate(user, 'New CA');
    await user.click(screen.getByRole('button', {name: 'Reset'}));

    const list = screen.getByRole('list', {name: 'Trusted issuer certificates'});
    expect(within(list).getByText('Existing CA')).toBeInTheDocument();
    expect(within(list).queryByText('New CA')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Save'})).not.toBeInTheDocument();
  });

  it('keeps only the names of new certificates after a successful save', async () => {
    const user = userEvent.setup();
    const {rerenderWith} = renderForm();
    await openIssuerTrustTab(user);

    await addValidCertificate(user, 'New CA');
    rerenderWith({submitting: true});
    rerenderWith({submitting: false, initial: {...definition, trustedAuthorities: ['Existing CA', 'New CA']}});

    expect(screen.queryByText('Unsaved')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Save'})).not.toBeInTheDocument();
  });
});

describe('VerifiablePresentationForm subject claim', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  const withClaims: VerifiablePresentation = {
    ...definition,
    mandatoryClaims: ['given_name', 'national_id'],
    optionalClaims: ['email'],
  };

  async function openClaimsTab(user: ReturnType<typeof userEvent.setup>): Promise<void> {
    await user.click(screen.getByRole('tab', {name: 'Claims'}));
  }

  it('offers only the mandatory claims and sends the chosen one', async () => {
    const user = userEvent.setup();
    renderForm({initial: withClaims});
    await openClaimsTab(user);

    expect(screen.getByRole('combobox', {name: 'Subject claim'})).toHaveTextContent('Credential subject (sub)');
    await user.click(screen.getByRole('combobox', {name: 'Subject claim'}));
    const listbox = await screen.findByRole('listbox');
    expect(
      within(listbox)
        .getAllByRole('option')
        .map((option: HTMLElement) => option.textContent),
    ).toEqual(['Credential subject (sub)', 'given_name', 'national_id']);
    await user.click(within(listbox).getByRole('option', {name: 'national_id'}));
    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({subjectClaim: 'national_id'}));
  });

  it('falls back to the credential sub when the chosen claim is removed', async () => {
    const user = userEvent.setup();
    renderForm({initial: {...withClaims, subjectClaim: 'national_id'}});
    await openClaimsTab(user);

    expect(screen.getByRole('combobox', {name: 'Subject claim'})).toHaveTextContent('national_id');
    await user.click(screen.getAllByRole('button', {name: 'Remove claim'})[1]);
    expect(screen.getByRole('combobox', {name: 'Subject claim'})).toHaveTextContent('Credential subject (sub)');

    await user.click(screen.getByRole('button', {name: 'Save'}));
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({subjectClaim: undefined}));
  });

  it('restores the stored subject claim on reset', async () => {
    const user = userEvent.setup();
    renderForm({initial: {...withClaims, subjectClaim: 'national_id'}});
    await openClaimsTab(user);

    await user.click(screen.getByRole('combobox', {name: 'Subject claim'}));
    await user.click(within(await screen.findByRole('listbox')).getByRole('option', {name: 'given_name'}));
    await user.click(screen.getByRole('button', {name: 'Reset'}));

    expect(screen.getByRole('combobox', {name: 'Subject claim'})).toHaveTextContent('national_id');
    expect(screen.queryByRole('button', {name: 'Save'})).not.toBeInTheDocument();
  });
});
