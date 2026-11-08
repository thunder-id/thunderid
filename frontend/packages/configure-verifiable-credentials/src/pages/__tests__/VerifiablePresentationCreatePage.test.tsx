// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import VerifiablePresentationCreatePage from '../VerifiablePresentationCreatePage';

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

vi.mock('../../api/useCreateVerifiablePresentation', () => ({
  default: () => ({mutate: vi.fn(), isPending: false, isError: false, reset: vi.fn()}),
}));

describe('VerifiablePresentationCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('asks for an organization unit outside a project', () => {
    render(<VerifiablePresentationCreatePage />);

    expect(screen.getByTestId('ou-step')).toBeInTheDocument();
  });

  it('skips the organization unit step inside a project', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <VerifiablePresentationCreatePage createInProjectsOnly />
      </ProjectScopeProvider>,
    );

    expect(screen.queryByTestId('ou-step')).not.toBeInTheDocument();
    expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
  });

  it('returns to the list outside a project when definitions are created in projects only', () => {
    render(<VerifiablePresentationCreatePage createInProjectsOnly />);

    expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/verifiable-presentations');
  });
});
