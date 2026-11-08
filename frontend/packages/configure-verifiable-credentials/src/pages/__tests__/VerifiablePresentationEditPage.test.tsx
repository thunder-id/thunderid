// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import VerifiablePresentationEditPage from '../VerifiablePresentationEditPage';

const {mockUseGetPresentation} = vi.hoisted(() => ({mockUseGetPresentation: vi.fn()}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useParams: () => ({vpId: 'vp-1'}),
    Navigate: ({to}: {to: string}) => <div data-testid="navigate" data-to={to} />,
  };
});

vi.mock('../../api/useGetVerifiablePresentation', () => ({default: mockUseGetPresentation}));
vi.mock('../../api/useUpdateVerifiablePresentation', () => ({
  default: () => ({mutate: vi.fn(), isPending: false, isError: false, reset: vi.fn()}),
}));
vi.mock('../../components/VerifiablePresentationForm', () => ({
  default: () => <div data-testid="presentation-form" />,
}));
vi.mock('../../components/VerifiablePresentationDeleteDialog', () => ({default: () => null}));

describe('VerifiablePresentationEditPage', () => {
  beforeEach(() => {
    mockUseGetPresentation.mockReturnValue({
      data: {id: 'vp-1', handle: 'pid', ouId: 'ou-1', projectId: 'project-2', vct: 'v', format: 'f'},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it('returns to the list for a definition of another project than the selected one', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <VerifiablePresentationEditPage />
      </ProjectScopeProvider>,
    );

    expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/verifiable-presentations');
    expect(screen.queryByTestId('presentation-form')).not.toBeInTheDocument();
  });

  it('opens a definition of the selected project', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-2'}}>
        <VerifiablePresentationEditPage />
      </ProjectScopeProvider>,
    );

    expect(screen.getByTestId('presentation-form')).toBeInTheDocument();
  });
});
