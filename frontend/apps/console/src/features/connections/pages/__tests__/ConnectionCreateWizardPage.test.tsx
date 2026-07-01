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
import ConnectionCreateWizardPage from '../ConnectionCreateWizardPage';

const mutateMock = vi.fn();
const navigateMock = vi.fn();

vi.mock('react-i18next', () => ({useTranslation: () => ({t: (key: string) => key})}));
vi.mock('react-router', () => ({useNavigate: () => navigateMock}));
vi.mock('@thunderid/contexts', () => ({useConfig: () => ({getServerUrl: () => 'https://id.acme.io'})}));
vi.mock('../../api/useCreateConnection', () => ({default: () => ({mutate: mutateMock, isPending: false})}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onChange}: {onChange: (s: unknown) => void}) {
    useEffect(() => {
      onChange({
        values: {name: 'Acme Workforce OIDC', clientId: 'x', clientSecret: 's', redirectUri: 'r', scopes: 'openid'},
        secretReplacing: false,
        valid: true,
      });
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

describe('ConnectionCreateWizardPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('shows the type heading without the redundant step label', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
    expect(screen.getByText('wizard.type.heading')).toBeInTheDocument();
    expect(screen.getAllByText('wizard.steps.type')).toHaveLength(1);
  });

  it('shows the configure heading without the redundant step label after continuing', () => {
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-oidc'));
    fireEvent.click(screen.getByTestId('wizard-continue'));

    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByText('wizard.configure.heading')).toBeInTheDocument();
    expect(screen.queryByText('wizard.steps.configure')).not.toBeInTheDocument();
  });
});
