// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import VerifiableCredentialCreatePage from '../VerifiableCredentialCreatePage';

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
  useHasMultipleOUs: () => ({hasMultipleOUs: true, isLoading: false, ouList: [{id: 'ou-a'}, {id: 'ou-b'}]}),
  useGetOrganizationUnit: () => ({data: undefined, isLoading: false}),
  OrganizationUnitPickerScreen: () => <div data-testid="ou-step" />,
}));

vi.mock('../../api/useCreateVerifiableCredential', () => ({
  default: () => ({mutate: vi.fn(), isPending: false, isError: false, reset: vi.fn()}),
}));

describe('VerifiableCredentialCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('asks for an organization unit outside a project', () => {
    render(<VerifiableCredentialCreatePage />);

    expect(screen.getByTestId('ou-step')).toBeInTheDocument();
  });

  it('skips the organization unit step inside a project', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <VerifiableCredentialCreatePage createInProjectsOnly />
      </ProjectScopeProvider>,
    );

    expect(screen.queryByTestId('ou-step')).not.toBeInTheDocument();
    expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
  });

  it('returns to the list outside a project when configurations are created in projects only', () => {
    render(<VerifiableCredentialCreatePage createInProjectsOnly />);

    expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/verifiable-credentials');
  });
});
