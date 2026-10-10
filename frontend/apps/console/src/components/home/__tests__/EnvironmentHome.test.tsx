// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import NextStepsSection from '../NextStepsSection';
import StartBuildingSection from '../StartBuildingSection';

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useEnvironment: () => ({
    environments: [{id: 'gw-2', name: 'Prod'}],
    selected: {id: 'gw-2', name: 'Prod'},
    select: vi.fn(),
  }),
}));

vi.mock('@thunderid/configure-applications', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-applications')>()),
  useGetApplications: () => ({data: {totalResults: 2, count: 1, applications: [{id: 'app-1', name: 'Orders'}]}}),
}));

vi.mock('@thunderid/configure-users', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-users')>()),
  useGetUsers: () => ({data: {totalResults: 0, users: []}, isLoading: false}),
}));

vi.mock('@wso2/oxygen-ui', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@wso2/oxygen-ui')>()),
  useMediaQuery: () => true,
}));

describe('the home page while an environment is shown', () => {
  it('offers to view what the environment runs, not to make anything', () => {
    renderWithProviders(
      <>
        <StartBuildingSection />
        <NextStepsSection />
      </>,
    );

    expect(screen.getByRole('button', {name: 'View 2 applications'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Create Application/})).not.toBeInTheDocument();
    expect(screen.queryByText('Start with a Template')).not.toBeInTheDocument();
    ['View Users', 'View Design', 'View Flows', 'View Connections'].forEach((label: string) => {
      expect(screen.getByRole('button', {name: label})).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', {name: 'Add User'})).not.toBeInTheDocument();
  });
});
