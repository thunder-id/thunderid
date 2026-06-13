/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {renderWithProviders, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, afterEach} from 'vitest';
import type {ResourcePermissions} from '../../../models/resource-server';
import SelectedScopesField from '../SelectedScopesField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (_key: string, fallback?: string) => fallback ?? _key}),
}));

afterEach(() => {
  vi.clearAllMocks();
});

describe('SelectedScopesField', () => {
  it('should render the scope string with all selected permissions space-separated', () => {
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']}];
    renderWithProviders(<SelectedScopesField selected={selected} />);
    expect(screen.getByDisplayValue('bookings bookings:create')).toBeInTheDocument();
  });

  it('should render an empty value when nothing is selected', () => {
    renderWithProviders(<SelectedScopesField selected={[]} />);
    expect(screen.getByRole('textbox')).toHaveValue('');
  });

  it('should show the placeholder when nothing is selected', () => {
    renderWithProviders(<SelectedScopesField selected={[]} />);
    expect(screen.getByPlaceholderText('No permissions selected')).toBeInTheDocument();
  });

  it('should disable the copy button when nothing is selected', () => {
    renderWithProviders(<SelectedScopesField selected={[]} />);
    expect(screen.getByRole('button', {name: /copy scopes/i})).toBeDisabled();
  });

  it('should enable the copy button when permissions are selected', () => {
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings']}];
    renderWithProviders(<SelectedScopesField selected={selected} />);
    expect(screen.getByRole('button', {name: /copy scopes/i})).not.toBeDisabled();
  });

  it('should copy the scope string to the clipboard', async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {value: {writeText}, writable: true, configurable: true});
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings']}];
    renderWithProviders(<SelectedScopesField selected={selected} />);
    await user.click(screen.getByRole('button', {name: /copy scopes/i}));
    expect(writeText).toHaveBeenCalledWith('bookings');
  });

  it('should flatten permissions from multiple resource servers', () => {
    const selected: ResourcePermissions[] = [
      {resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']},
      {resourceServerId: 'rs-2', permissions: ['payments:refund']},
    ];
    renderWithProviders(<SelectedScopesField selected={selected} />);
    expect(screen.getByDisplayValue('bookings bookings:create payments:refund')).toBeInTheDocument();
  });

  it('should render the field as readonly', () => {
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings']}];
    renderWithProviders(<SelectedScopesField selected={selected} />);
    const input = screen.getByRole('textbox');
    expect(input).toHaveAttribute('readonly');
  });
});
