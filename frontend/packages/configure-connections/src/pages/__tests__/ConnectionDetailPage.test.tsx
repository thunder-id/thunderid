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

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {type ReactNode, useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionDetailPage from '../ConnectionDetailPage';

const updateMock = vi.fn().mockResolvedValue({});
const refetchMock = vi.fn().mockResolvedValue({});
const deleteMock = vi.fn((_id: string, opts: {onSuccess: () => void}) => opts.onSuccess());
const navigateMock = vi.fn();

const ATTR_CONFIG = {
  userTypeResolution: {default: 'employee'},
  userTypeAttributeMappings: [
    {userType: 'employee', attributes: [{externalAttribute: 'email', localAttribute: 'mail'}]},
  ],
};

const CONNECTION = {
  id: 'g1',
  type: 'google',
  name: 'Google',
  clientId: 'cid',
  clientSecret: '******',
  redirectUri: 'https://id.acme.io/oauth/callback/google',
  scopes: ['openid'],
  attributeConfiguration: ATTR_CONFIG,
};

const TWILIO_CONNECTION = {
  id: 'tw1',
  type: 'twilio',
  name: 'Twilio',
  accountSid: 'AC00000000000000000000000000000000',
  authToken: '******',
  senderId: '+15005550006',
};

const mockParams: {type: string; id: string} = {type: 'google', id: 'g1'};
const mockConn: {data: Record<string, unknown>} = {data: CONNECTION};

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
  useParams: () => mockParams,
}));
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getGateCallbackUrl: () => 'https://id.acme.io/gate/callback'}),
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

vi.mock('../../api/useConnection', () => ({
  default: () => ({data: mockConn.data, isLoading: false, isError: false, refetch: refetchMock}),
}));
vi.mock('../../api/useConnectionInstances', () => ({default: () => ({data: [], isLoading: false})}));
vi.mock('../../api/useUpdateConnection', () => ({default: () => ({mutateAsync: updateMock, isPending: false})}));
vi.mock('../../api/useDeleteConnection', () => ({default: () => ({mutate: deleteMock, isPending: false})}));
vi.mock('../../api/useGetConnectionUsages', () => ({
  default: () => ({data: {totalResults: 0, count: 0, summary: {}, usages: []}, isLoading: false}),
}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onFieldChange}: {onFieldChange: (name: string, value: string) => void}) {
    return (
      <div data-testid="stub-connection-form">
        <button type="button" data-testid="edit-client-id" onClick={() => onFieldChange('clientId', 'changed')}>
          edit
        </button>
      </div>
    );
  },
}));

vi.mock('../../components/AttributeMappingSection', () => ({
  default: function StubAttributeMappingSection({onChange}: {onChange: (c: unknown, v: boolean) => void}) {
    useEffect(() => {
      onChange(ATTR_CONFIG, true);
    }, [onChange]);
    return <div data-testid="stub-attribute-mapping" />;
  },
}));

describe('ConnectionDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.type = 'google';
    mockParams.id = 'g1';
    mockConn.data = CONNECTION;
  });

  it('renders the general tab with quick-copy, the credentials form, and a danger-zone delete', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByTestId('connection-id-copy')).toBeInTheDocument();
    expect(screen.getByDisplayValue('g1')).toBeInTheDocument();
    expect(screen.getByText('Unique identifier for this connection.')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByTestId('connection-delete-button')).toBeInTheDocument();
  });

  it('hides the save bar until a field is edited', () => {
    render(<ConnectionDetailPage />);
    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('edit-client-id'));
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();
  });

  it('saves the merged payload, preserves stored attribute mappings, refetches, then clears the dirty bar', async () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as {name: string; clientId: string; attributeConfiguration?: unknown};
    expect(payload).toMatchObject({name: 'Google', clientId: 'changed'});
    // General-tab-only edit must not wipe the stored attribute configuration.
    expect(payload.attributeConfiguration).toEqual(ATTR_CONFIG);

    await waitFor(() => expect(refetchMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument());
  });

  it('deletes the connection and returns to the list', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-delete-button'));
    fireEvent.click(screen.getByTestId('connection-delete-confirm'));
    expect(deleteMock).toHaveBeenCalledWith('g1', expect.anything());
    expect(navigateMock).toHaveBeenCalledWith('/connections');
  });

  it('SMS vendor: hides the attribute-mapping tab and save omits attributeConfiguration', () => {
    mockParams.type = 'twilio';
    mockParams.id = 'tw1';
    mockConn.data = TWILIO_CONNECTION;
    render(<ConnectionDetailPage />);

    expect(screen.getByTestId('connection-tab-general')).toBeInTheDocument();
    expect(screen.queryByTestId('connection-tab-attributes')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'Twilio',
      accountSid: 'AC00000000000000000000000000000000',
      senderId: '+15005550006',
    });
    expect('attributeConfiguration' in payload).toBe(false);
  });
});
