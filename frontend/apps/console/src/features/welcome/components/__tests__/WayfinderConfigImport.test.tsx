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

import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    CheckCircle: () => <span data-testid="icon-check-circle" />,
    Database: () => <span data-testid="icon-database" />,
    RefreshCw: () => <span data-testid="icon-refresh-cw" />,
    XCircle: () => <span data-testid="icon-x-circle" />,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => (opts ? `${key}:${JSON.stringify(opts)}` : key),
  }),
}));

const {mockUseImportConfiguration, mockMutateAsync, mockUseGetSampleBundle} = vi.hoisted(() => ({
  mockUseImportConfiguration: vi.fn(),
  mockMutateAsync: vi.fn(),
  mockUseGetSampleBundle: vi.fn(),
}));

vi.mock('../../../import-export/api/useImportConfiguration', () => ({
  default: mockUseImportConfiguration,
}));

vi.mock('../../api/useGetSampleBundles', () => ({
  useGetSampleBundle: mockUseGetSampleBundle,
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({config: {brand: {product_name: 'ThunderID'}}}),
  };
});

import WayfinderConfigImport from '../WayfinderConfigImport';

const RESOLVED_IMPORTED_KEY = 'thunderid:wayfinder-config-imported';

describe('WayfinderConfigImport', () => {
  const mockSessionStorageGetItem = vi.fn();
  const mockSessionStorageSetItem = vi.fn();

  beforeEach(() => {
    mockSessionStorageGetItem.mockReturnValue(null);
    vi.stubGlobal('sessionStorage', {
      getItem: mockSessionStorageGetItem,
      setItem: mockSessionStorageSetItem,
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
    mockUseImportConfiguration.mockReturnValue({mutateAsync: mockMutateAsync});
    mockUseGetSampleBundle.mockReturnValue({
      configs: {declarative: 'yaml content', env: 'KEY=value'},
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('renders the import button when idle', () => {
    render(<WayfinderConfigImport />);
    expect(
      screen.getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}'),
    ).toBeInTheDocument();
  });

  it('disables the button when the bundle is missing declarative content', () => {
    mockUseGetSampleBundle.mockReturnValueOnce(undefined);
    render(<WayfinderConfigImport />);
    const button = screen
      .getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}')
      .closest('button');
    expect(button).toBeDisabled();
  });

  it('imports the inlined bundle on click and reports success', async () => {
    mockMutateAsync.mockResolvedValueOnce({
      summary: {imported: 5, failed: 0},
      results: [],
    });
    const onSuccess = vi.fn();
    render(<WayfinderConfigImport onSuccess={onSuccess} />);

    await userEvent.click(
      screen.getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}'),
    );

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        content: 'yaml content',
        variables: {KEY: 'value'},
        options: {upsert: true},
      });
    });
    expect(mockSessionStorageSetItem).toHaveBeenCalledWith(RESOLVED_IMPORTED_KEY, expect.any(String));
    expect(onSuccess).toHaveBeenCalled();
  });

  it('shows the already-imported state when sessionStorage has a previous timestamp', () => {
    mockSessionStorageGetItem.mockReturnValue('1700000000000');
    render(<WayfinderConfigImport />);
    expect(
      screen.getByText('common:welcome.wayfinderFolderImport.status.alreadyDone:{"productName":"ThunderID"}'),
    ).toBeInTheDocument();
  });

  it('reports error when the import call rejects', async () => {
    mockMutateAsync.mockRejectedValueOnce(new Error('boom'));
    render(<WayfinderConfigImport />);

    await userEvent.click(
      screen.getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}'),
    );

    await waitFor(() => {
      expect(screen.getByText('common:welcome.wayfinderFolderImport.errors.importFailed')).toBeInTheDocument();
    });
  });

  it('reports partial failure when the import response includes failed resources', async () => {
    mockMutateAsync.mockResolvedValueOnce({
      summary: {imported: 3, failed: 2},
      results: [
        {status: 'failed', resourceType: 'application', resourceName: 'foo', message: 'duplicate'},
        {status: 'failed', resourceType: 'role', resourceName: 'bar', message: 'invalid'},
        {status: 'success', resourceType: 'user', resourceName: 'baz', message: 'ok'},
      ],
    });
    const onSuccess = vi.fn();
    render(<WayfinderConfigImport onSuccess={onSuccess} />);

    await userEvent.click(
      screen.getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}'),
    );

    await waitFor(() => {
      expect(
        screen.getByText('common:welcome.wayfinderFolderImport.errors.partialFailure:{"count":2}'),
      ).toBeInTheDocument();
    });
    expect(screen.getByText(/application · foo: duplicate/)).toBeInTheDocument();
    expect(screen.getByText(/role · bar: invalid/)).toBeInTheDocument();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('renders the last-imported date caption in the already-done state', () => {
    mockSessionStorageGetItem.mockReturnValue('1700000000000');
    render(<WayfinderConfigImport />);
    expect(screen.getByText(/common:welcome.wayfinderFolderImport.status.lastImported:/)).toBeInTheDocument();
  });

  it('resets to idle when the user clicks Reconfigure from already-done state', async () => {
    mockSessionStorageGetItem.mockReturnValue('1700000000000');
    render(<WayfinderConfigImport />);

    await userEvent.click(screen.getByText('common:welcome.wayfinderFolderImport.actions.reconfigure'));

    expect(
      screen.getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}'),
    ).toBeInTheDocument();
  });

  it('skips comments and blank lines when parsing the env bundle', async () => {
    mockUseGetSampleBundle.mockReturnValue({
      configs: {
        declarative: 'yaml content',
        env: '# comment line\n\nKEY=value\nORPHAN_LINE_NO_EQUALS\nOTHER=2',
      },
    });
    mockMutateAsync.mockResolvedValueOnce({summary: {imported: 1, failed: 0}, results: []});
    render(<WayfinderConfigImport />);

    await userEvent.click(
      screen.getByText('common:welcome.wayfinderFolderImport.actions.importConfig:{"productName":"ThunderID"}'),
    );

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        content: 'yaml content',
        variables: {KEY: 'value', OTHER: '2'},
        options: {upsert: true},
      });
    });
  });
});
