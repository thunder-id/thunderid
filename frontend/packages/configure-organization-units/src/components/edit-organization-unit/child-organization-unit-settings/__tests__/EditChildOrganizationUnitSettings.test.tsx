/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {screen, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import EditChildOrganizationUnitSettings from '../EditChildOrganizationUnitSettings';

// Mock child component
vi.mock(
  '@/components/edit-organization-unit/child-organization-unit-settings/ManageChildOrganizationUnitSection',
  () => ({
    default: ({
      organizationUnitId,
      organizationUnitName,
    }: {
      organizationUnitId: string;
      organizationUnitName: string;
    }) => (
      <div data-testid="manage-child-ous-section">
        ManageChildOUsSection - {organizationUnitId} - {organizationUnitName}
      </div>
    ),
  }),
);

describe('EditChildOrganizationUnitSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render ManageChildOUsSection', () => {
    renderWithProviders(
      <EditChildOrganizationUnitSettings organizationUnitId="ou-123" organizationUnitName="Engineering" />,
    );

    expect(screen.getByTestId('manage-child-ous-section')).toBeInTheDocument();
  });

  it('should pass organizationUnitId to ManageChildOUsSection', () => {
    renderWithProviders(
      <EditChildOrganizationUnitSettings organizationUnitId="ou-456" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText(/ManageChildOUsSection - ou-456/)).toBeInTheDocument();
  });

  it('should pass organizationUnitName to ManageChildOUsSection', () => {
    renderWithProviders(
      <EditChildOrganizationUnitSettings organizationUnitId="ou-123" organizationUnitName="Product Team" />,
    );

    expect(screen.getByText(/Product Team/)).toBeInTheDocument();
  });

  it('should handle different organization unit IDs and names', () => {
    const {rerender} = renderWithProviders(
      <EditChildOrganizationUnitSettings organizationUnitId="ou-123" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText('ManageChildOUsSection - ou-123 - Engineering')).toBeInTheDocument();

    rerender(<EditChildOrganizationUnitSettings organizationUnitId="ou-789" organizationUnitName="Design" />);

    expect(screen.getByText('ManageChildOUsSection - ou-789 - Design')).toBeInTheDocument();
  });
});
