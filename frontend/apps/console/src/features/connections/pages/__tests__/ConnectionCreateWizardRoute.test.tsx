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

import {render, screen} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, expect, it, vi} from 'vitest';
import ConnectionCreateWizardRoute from '../ConnectionCreateWizardRoute';

vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  ConnectionCreateWizardPage: ({
    customConfigureSteps = undefined,
  }: {
    customConfigureSteps?: Record<string, ReactNode>;
  }) => <div data-testid="stub-wizard">{customConfigureSteps?.['trusted-idp']}</div>,
}));

vi.mock('../../../trusted-issuers/pages/TrustedIssuerWizardStep', () => ({
  default: function StubTrustedIssuerWizardStep() {
    return <div data-testid="stub-trusted-issuer-wizard-step" />;
  },
}));

describe('ConnectionCreateWizardRoute', () => {
  it('should wire the trusted-idp custom configure step to TrustedIssuerWizardStep', () => {
    render(<ConnectionCreateWizardRoute />);

    expect(screen.getByTestId('stub-wizard')).toBeInTheDocument();
    expect(screen.getByTestId('stub-trusted-issuer-wizard-step')).toBeInTheDocument();
  });
});
