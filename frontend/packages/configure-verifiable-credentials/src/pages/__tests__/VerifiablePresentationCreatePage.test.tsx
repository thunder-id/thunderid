// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {fireEvent, render, screen, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import VerifiablePresentationCreatePage from '../VerifiablePresentationCreatePage';

const {mockUseHasMultipleOUs, mockUseGetOrganizationUnit, mockMutate} = vi.hoisted(() => ({
  mockUseHasMultipleOUs: vi.fn(),
  mockUseGetOrganizationUnit: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    Navigate: ({to}: {to: string}) => <div data-testid="navigate" data-to={to} />,
  };
});

vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  useHasMultipleOUs: (projectId?: string): unknown => mockUseHasMultipleOUs(projectId),
  useGetOrganizationUnit: (id: string, enabled: boolean): unknown => mockUseGetOrganizationUnit(id, enabled),
  OrganizationUnitPickerScreen: ({
    projectId = undefined,
    onChange,
    onContinue,
  }: {
    projectId?: string;
    onChange: (ouId: string) => void;
    onContinue: () => void;
  }) => (
    <div data-testid="ou-step" data-project-id={projectId}>
      <button type="button" onClick={() => onChange('system-ou')}>
        pick system ou
      </button>
      <button type="button" onClick={() => onChange('ou-picked')}>
        pick ou
      </button>
      <button type="button" onClick={onContinue}>
        ou continue
      </button>
    </div>
  ),
}));

vi.mock('../../components/create-verifiable-presentation/ConfigureName', () => ({
  default: ({
    organizationUnitName = undefined,
    onNameChange,
    onHandleChange,
  }: {
    organizationUnitName?: string;
    onNameChange: (value: string) => void;
    onHandleChange: (value: string) => void;
  }) => (
    <div>
      <span data-testid="ou-name">{organizationUnitName}</span>
      <button
        type="button"
        onClick={() => {
          onNameChange('Person ID');
          onHandleChange('pid');
        }}
      >
        fill name
      </button>
    </div>
  ),
}));

vi.mock('../../components/PresentationClaimsEditor', () => ({
  default: ({onChange}: {onChange: (rows: unknown[]) => void}) => (
    <button
      type="button"
      onClick={() => onChange([{id: 'claim-1', name: 'given_name', requirement: 'mandatory', values: []}])}
    >
      fill claims
    </button>
  ),
}));

vi.mock('../../api/useCreateVerifiablePresentation', () => ({
  default: () => ({mutate: mockMutate, isPending: false, isError: false, reset: vi.fn()}),
}));

const PEM = ['-----BEGIN CERTIFICATE-----', 'MIIBszCCAVmgAwIBAgIUQm9ndXNDZXJ0', '-----END CERTIFICATE-----'].join('\n');

function renderInProject(): void {
  render(
    <ProjectScopeProvider project={{id: 'project-1'}}>
      <VerifiablePresentationCreatePage createInProjectsOnly />
    </ProjectScopeProvider>,
  );
}

function goToClaimsStep(): void {
  fireEvent.click(screen.getByRole('button', {name: 'fill name'}));
  fireEvent.click(screen.getByRole('button', {name: 'Continue'}));
  fireEvent.change(document.getElementById('vp-vct')!, {target: {value: 'urn:eudi:pid:de:1'}});
  fireEvent.click(screen.getByRole('button', {name: 'Continue'}));
  fireEvent.click(screen.getByRole('button', {name: 'fill claims'}));
}

async function addCertificate(certificateName: string): Promise<void> {
  fireEvent.click(screen.getByRole('button', {name: 'Add certificate'}));
  const dialog = await screen.findByRole('dialog');
  fireEvent.change(document.getElementById('vp-trusted-certificate-name')!, {target: {value: certificateName}});
  fireEvent.change(document.getElementById('vp-trusted-certificate-pem')!, {target: {value: PEM}});
  fireEvent.click(within(dialog).getByRole('button', {name: 'Add'}));
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
}

/** Moves from the claims step to the trusted issuer step, adds a certificate, and creates the definition. */
async function addCertificateAndCreate(): Promise<Record<string, unknown>> {
  fireEvent.click(screen.getByRole('button', {name: 'Continue'}));
  await addCertificate('Issuer CA');
  fireEvent.click(screen.getByRole('button', {name: 'Create'}));

  return mockMutate.mock.calls[0][0] as Record<string, unknown>;
}

async function createFromNameStep(): Promise<Record<string, unknown>> {
  goToClaimsStep();

  return addCertificateAndCreate();
}

describe('VerifiablePresentationCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: true,
      isLoading: false,
      ouList: [{id: 'ou-a'}, {id: 'ou-b'}],
    });
    mockUseGetOrganizationUnit.mockImplementation((id: string) => ({
      data: id === 'system-ou' ? {id, name: 'Default'} : undefined,
      isLoading: false,
    }));
  });

  it('asks for an organization unit outside a project', () => {
    render(<VerifiablePresentationCreatePage />);

    expect(mockUseHasMultipleOUs).toHaveBeenCalledWith(undefined);
    expect(screen.getByTestId('ou-step')).not.toHaveAttribute('data-project-id');
  });

  it('creates the definition in the picked organization unit outside a project', async () => {
    render(<VerifiablePresentationCreatePage />);

    fireEvent.click(screen.getByRole('button', {name: 'pick ou'}));
    fireEvent.click(screen.getByRole('button', {name: 'ou continue'}));
    const request = await createFromNameStep();

    expect(request).toMatchObject({handle: 'pid', ouId: 'ou-picked'});
    expect(request).not.toHaveProperty('projectId');
  });

  it('skips the organization unit step inside a project that has no organization units', async () => {
    mockUseHasMultipleOUs.mockReturnValue({hasMultipleOUs: false, isLoading: false, ouList: []});
    renderInProject();

    expect(mockUseHasMultipleOUs).toHaveBeenCalledWith('project-1');
    expect(screen.queryByTestId('ou-step')).not.toBeInTheDocument();
    expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();

    const request = await createFromNameStep();
    expect(request).toMatchObject({handle: 'pid', projectId: 'project-1'});
    expect(request).not.toHaveProperty('ouId');
  });

  it("asks for one of the project's organization units when the project has any", () => {
    renderInProject();

    expect(screen.getByTestId('ou-step')).toHaveAttribute('data-project-id', 'project-1');
  });

  it('creates the definition in the system organization unit the picker selects', async () => {
    renderInProject();

    fireEvent.click(screen.getByRole('button', {name: 'pick system ou'}));
    fireEvent.click(screen.getByRole('button', {name: 'ou continue'}));
    expect(screen.getByTestId('ou-name')).toHaveTextContent('Default');
    const request = await createFromNameStep();

    expect(request).toMatchObject({handle: 'pid', projectId: 'project-1', ouId: 'system-ou'});
  });

  it('creates the definition in the picked project organization unit', async () => {
    renderInProject();

    fireEvent.click(screen.getByRole('button', {name: 'pick ou'}));
    fireEvent.click(screen.getByRole('button', {name: 'ou continue'}));
    const request = await createFromNameStep();

    expect(request).toMatchObject({handle: 'pid', projectId: 'project-1', ouId: 'ou-picked'});
  });

  it('returns to the list outside a project when definitions are created in projects only', () => {
    render(<VerifiablePresentationCreatePage createInProjectsOnly />);

    expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/verifiable-presentations');
  });

  it('creates a definition that cannot sign in by default', async () => {
    mockUseHasMultipleOUs.mockReturnValue({hasMultipleOUs: false, isLoading: false, ouList: []});
    renderInProject();
    goToClaimsStep();

    expect(screen.getByRole('switch', {name: 'Allow sign-in using this definition'})).not.toBeChecked();
    expect(await addCertificateAndCreate()).toMatchObject({subjectClaim: undefined});
  });

  it('requires a subject claim once sign-in is allowed and sends it', async () => {
    mockUseHasMultipleOUs.mockReturnValue({hasMultipleOUs: false, isLoading: false, ouList: []});
    renderInProject();
    goToClaimsStep();

    fireEvent.click(screen.getByRole('switch', {name: 'Allow sign-in using this definition'}));
    expect(screen.getByText('Pick a subject claim.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();

    fireEvent.mouseDown(screen.getByRole('combobox', {name: /Subject claim/}));
    fireEvent.click(within(await screen.findByRole('listbox')).getByRole('option', {name: 'given_name'}));

    expect(await addCertificateAndCreate()).toMatchObject({subjectClaim: 'given_name'});
  });

  it('clears the subject claim when sign-in is turned off', async () => {
    mockUseHasMultipleOUs.mockReturnValue({hasMultipleOUs: false, isLoading: false, ouList: []});
    renderInProject();
    goToClaimsStep();

    fireEvent.click(screen.getByRole('switch', {name: 'Allow sign-in using this definition'}));
    fireEvent.mouseDown(screen.getByRole('combobox', {name: /Subject claim/}));
    fireEvent.click(within(await screen.findByRole('listbox')).getByRole('option', {name: 'given_name'}));
    fireEvent.click(screen.getByRole('switch', {name: 'Allow sign-in using this definition'}));

    expect(await addCertificateAndCreate()).toMatchObject({subjectClaim: undefined});
  });

  it('blocks create until a trusted issuer certificate is added and sends the certificates', async () => {
    mockUseHasMultipleOUs.mockReturnValue({hasMultipleOUs: false, isLoading: false, ouList: []});
    renderInProject();
    goToClaimsStep();
    fireEvent.click(screen.getByRole('button', {name: 'Continue'}));

    expect(screen.getByText('Add at least one trusted issuer certificate.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Create'})).toBeDisabled();

    await addCertificate('Removed CA');
    fireEvent.click(screen.getByRole('button', {name: 'Remove certificate Removed CA'}));
    expect(screen.getByRole('button', {name: 'Create'})).toBeDisabled();

    await addCertificate('Issuer CA');
    await addCertificate('Root CA');
    const list = screen.getByRole('list', {name: 'Trusted issuer certificates'});
    expect(within(list).getByText('Issuer CA')).toBeInTheDocument();
    expect(within(list).queryByText('Unsaved')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Create'}));

    const request = mockMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(request.trustedAuthorities).toEqual([
      {name: 'Issuer CA', certificate: PEM},
      {name: 'Root CA', certificate: PEM},
    ]);
    expect(request).not.toHaveProperty('enforceTrustedIssuer');
  });
});
