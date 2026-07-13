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

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ConnectionUsagesResponse} from '../../models/connection';
import ConnectionDeleteDialog from '../ConnectionDeleteDialog';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, arg?: unknown): string => {
      const translations: Record<string, string> = {
        'delete.title': 'Delete connection',
        'delete.usages.loading': 'Checking affected resources…',
        'delete.blocking.title':
          'This connection cannot be deleted until the following resources are updated or removed:',
        'common:actions.cancel': 'Cancel',
        'common:actions.delete': 'Delete',
      };
      if (translations[key]) return translations[key];
      if (arg && typeof arg === 'object') {
        const obj = arg as {name?: string; count?: number};
        if (key === 'delete.message') return `Are you sure you want to delete “${obj.name}”?`;
        if (key === 'delete.usages.more') return `+${obj.count} more`;
      }
      return key;
    },
  }),
}));

const {getUsagesMock} = vi.hoisted(() => ({
  getUsagesMock: vi.fn<() => {data: ConnectionUsagesResponse | undefined; isLoading: boolean}>(),
}));
vi.mock('../../api/useGetConnectionUsages', () => ({
  default: () => getUsagesMock(),
}));

describe('ConnectionDeleteDialog', () => {
  const onConfirm = vi.fn();
  const onClose = vi.fn();

  const renderDialog = (): void => {
    render(
      <ConnectionDeleteDialog
        open
        connectionType="google"
        connectionId="g1"
        connectionName="My Google"
        isPending={false}
        onConfirm={onConfirm}
        onClose={onClose}
      />,
    );
  };

  beforeEach(() => {
    vi.clearAllMocks();
    getUsagesMock.mockReturnValue({data: {totalResults: 0, count: 0, summary: {}, usages: []}, isLoading: false});
  });

  it('confirms deletion when there are no blocking usages', () => {
    renderDialog();

    const confirm = screen.getByTestId('connection-delete-confirm');
    expect(confirm).not.toBeDisabled();
    fireEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('disables delete and shows a loading alert while usages are fetched', () => {
    getUsagesMock.mockReturnValue({data: undefined, isLoading: true});
    renderDialog();

    expect(screen.getByText('Checking affected resources…')).toBeInTheDocument();
    expect(screen.getByTestId('connection-delete-confirm')).toBeDisabled();
  });

  it('lists blocking resources and disables delete', () => {
    const usages: ConnectionUsagesResponse = {
      totalResults: 2,
      count: 2,
      summary: {flow: 2},
      usages: [
        {resourceType: 'flow', id: 'flow-1', displayName: 'Login Flow', behaviorOnDelete: 'restrict'},
        {resourceType: 'flow', id: 'flow-2', displayName: 'Signup Flow', behaviorOnDelete: 'restrict'},
      ],
    };
    getUsagesMock.mockReturnValue({data: usages, isLoading: false});
    renderDialog();

    expect(
      screen.getByText('This connection cannot be deleted until the following resources are updated or removed:'),
    ).toBeInTheDocument();
    expect(screen.getByText('Login Flow')).toBeInTheDocument();
    expect(screen.getByText('Signup Flow')).toBeInTheDocument();
    expect(screen.getByTestId('connection-delete-confirm')).toBeDisabled();
  });

  it('shows a "+N more" row when blocking usages exceed the visible limit', () => {
    const usages: ConnectionUsagesResponse = {
      totalResults: 7,
      count: 7,
      summary: {flow: 7},
      usages: Array.from({length: 7}, (_, i) => ({
        resourceType: 'flow',
        id: `flow-${i}`,
        displayName: `Flow ${i}`,
        behaviorOnDelete: 'restrict' as const,
      })),
    };
    getUsagesMock.mockReturnValue({data: usages, isLoading: false});
    renderDialog();

    expect(screen.getByText('+2 more')).toBeInTheDocument();
  });

  it('does not block deletion when usage data is unknown', () => {
    getUsagesMock.mockReturnValue({data: {totalResults: null, count: 0, summary: null, usages: []}, isLoading: false});
    renderDialog();

    expect(screen.getByTestId('connection-delete-confirm')).not.toBeDisabled();
  });
});
