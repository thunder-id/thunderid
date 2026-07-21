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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {AttestationConfig} from '../../../../models/oauth';
import AttestationSection from '../AttestationSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

// Feeds back whatever the section emits as its next prop, mimicking the edit page's
// config round-trip. Guards against a field becoming un-typeable if the round-trip stalls.
function Harness() {
  const [attestation, setAttestation] = useState<AttestationConfig | null | undefined>(undefined);
  return <AttestationSection attestation={attestation ?? undefined} onAttestationChange={setAttestation} />;
}

async function selectPlatform(user: ReturnType<typeof userEvent.setup>, optionKey: string) {
  await user.click(screen.getByRole('combobox'));
  await user.click(await screen.findByRole('option', {name: optionKey}));
}

describe('AttestationSection round-trip', () => {
  it('lets the user select Android and type the package name, reflecting it back', async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await selectPlatform(user, 'applications:edit.advanced.attestation.platform.android');
    const input = screen.getByLabelText('applications:edit.advanced.attestation.labels.packageName');
    await user.type(input, 'com.example.app');

    expect(input).toHaveValue('com.example.app');
  });

  it('lets the user type the service account credentials', async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await selectPlatform(user, 'applications:edit.advanced.attestation.platform.android');
    const creds = screen.getByLabelText('applications:edit.advanced.attestation.labels.serviceAccountCredentials');
    await user.type(creds, 'abc123');

    expect(creds).toHaveValue('abc123');
  });

  it('lets the user select iOS and type the team and bundle ids, reflecting them back', async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await selectPlatform(user, 'applications:edit.advanced.attestation.platform.apple');
    const teamId = screen.getByLabelText('applications:edit.advanced.attestation.labels.teamId');
    const bundleId = screen.getByLabelText('applications:edit.advanced.attestation.labels.bundleId');
    await user.type(teamId, 'ABCDE12345');
    await user.type(bundleId, 'com.example.myapp');

    expect(teamId).toHaveValue('ABCDE12345');
    expect(bundleId).toHaveValue('com.example.myapp');
  });
});
