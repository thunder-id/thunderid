// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, it, expect, vi} from 'vitest';
import AttestationSection from '../AttestationSection';
import type {AttestationConfig} from '@thunderid/configure-applications';

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
