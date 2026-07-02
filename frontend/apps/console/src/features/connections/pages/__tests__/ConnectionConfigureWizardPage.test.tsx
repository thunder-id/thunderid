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
import {useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionConfigureWizardPage from '../ConnectionConfigureWizardPage';

const mutateMock = vi.fn();
const navigateMock = vi.fn();

vi.mock('react-i18next', () => ({useTranslation: () => ({t: (key: string) => key})}));
vi.mock('react-router', () => ({useNavigate: () => navigateMock, useParams: () => ({type: 'google'})}));
vi.mock('@thunderid/contexts', () => ({useConfig: () => ({getServerUrl: () => 'https://id.acme.io'})}));
vi.mock('../../api/useCreateConnection', () => ({default: () => ({mutate: mutateMock, isPending: false})}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onChange}: {onChange: (s: unknown) => void}) {
    useEffect(() => {
      onChange({values: {clientId: 'x', clientSecret: 's', redirectUri: 'r'}, secretReplacing: false, valid: true});
    }, [onChange]);
    return <div data-testid="stub-connection-form" />;
  },
}));

vi.mock('../../components/create-connection/ConnectionAttributeMappingStep', () => ({
  default: function StubAttributeMappingStep({
    onChange,
    onCreate,
  }: {
    onChange: (c: unknown, v: boolean) => void;
    onCreate: () => void;
  }) {
    useEffect(() => {
      onChange(undefined, true);
    }, [onChange]);
    return (
      <button type="button" data-testid="wizard-create" onClick={onCreate}>
        create
      </button>
    );
  },
}));

describe('ConnectionConfigureWizardPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('walks configure → attribute mapping and creates with the fixed vendor name', () => {
    render(<ConnectionConfigureWizardPage />);

    // Step 1: the credentials form is shown; Continue is enabled once valid.
    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByText('configure.heading')).toBeInTheDocument();
    expect(screen.queryByText('wizard.steps.configure')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('wizard-continue'));

    // Step 2: create.
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    const payload = mutateMock.mock.calls[0][0] as {name: string; clientId: string; attributeConfiguration?: unknown};
    expect(payload).toMatchObject({name: 'Google', clientId: 'x', clientSecret: 's'});
    expect(payload.attributeConfiguration).toBeUndefined();
  });
});
