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

import {fireEvent, render, screen} from '@testing-library/react';
import {type ReactNode, useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionDetailPage from '../ConnectionDetailPage';

const updateMock = vi.fn();
const deleteMock = vi.fn((_id: string, opts: {onSuccess: () => void}) => opts.onSuccess());
const navigateMock = vi.fn();

const CONNECTION = {
  id: 'g1',
  type: 'google',
  name: 'Google',
  clientId: 'cid',
  clientSecret: '******',
  redirectUri: 'https://id.acme.io/oauth/callback/google',
  scopes: ['openid'],
  attributeConfiguration: undefined,
};

vi.mock('react-i18next', () => ({useTranslation: () => ({t: (key: string) => key})}));
vi.mock('react-router', () => ({useNavigate: () => navigateMock, useParams: () => ({type: 'google', id: 'g1'})}));
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({getServerUrl: () => 'https://id.acme.io'}),
  useToast: () => ({showToast: vi.fn()}),
}));
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, children}: {title: string; children: ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
  UnsavedChangesBar: ({onSave, saveLabel}: {onSave: () => void; saveLabel: string}) => (
    <button type="button" data-testid="save-bar" onClick={onSave}>
      {saveLabel}
    </button>
  ),
}));

vi.mock('../../api/useConnection', () => ({default: () => ({data: CONNECTION, isLoading: false, isError: false})}));
vi.mock('../../api/useConnectionInstances', () => ({default: () => ({data: [], isLoading: false})}));
vi.mock('../../api/useUpdateConnection', () => ({default: () => ({mutate: updateMock, isPending: false})}));
vi.mock('../../api/useDeleteConnection', () => ({default: () => ({mutate: deleteMock, isPending: false})}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onChange}: {onChange: (s: unknown) => void}) {
    useEffect(() => {
      onChange({
        values: {
          name: 'Google',
          clientId: 'changed',
          clientSecret: '',
          redirectUri: CONNECTION.redirectUri,
          scopes: 'openid',
        },
        secretReplacing: false,
        valid: true,
      });
    }, [onChange]);
    return <div data-testid="stub-connection-form" />;
  },
}));

vi.mock('../../components/AttributeMappingSection', () => ({
  default: function StubAttributeMappingSection({onChange}: {onChange: (c: unknown, v: boolean) => void}) {
    useEffect(() => {
      onChange(undefined, true);
    }, [onChange]);
    return <div data-testid="stub-attribute-mapping" />;
  },
}));

describe('ConnectionDetailPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders the general tab with quick-copy, the credentials form, and a danger-zone delete', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByTestId('connection-id-copy')).toBeInTheDocument();
    expect(screen.getByDisplayValue('g1')).toBeInTheDocument();
    expect(screen.getByText('detail.connectionId.hint')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByTestId('connection-delete-button')).toBeInTheDocument();
  });

  it('shows the sticky save bar when dirty and saves the merged payload', async () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(await screen.findByTestId('save-bar'));
    expect(updateMock).toHaveBeenCalledTimes(1);
    expect(updateMock.mock.calls[0][0]).toMatchObject({name: 'Google', clientId: 'changed'});
  });

  it('deletes the connection and returns to the list', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-delete-button'));
    fireEvent.click(screen.getByTestId('connection-delete-confirm'));
    expect(deleteMock).toHaveBeenCalledWith('g1', expect.anything());
    expect(navigateMock).toHaveBeenCalledWith('/connections');
  });
});
