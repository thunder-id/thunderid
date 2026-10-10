// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, AdministrationProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import AgentAddRoute from '../AgentAddRoute';

vi.mock('../AgentAddFormPage', () => ({
  default: () => <div data-testid="agent-add-form-page" />,
}));

vi.mock('../AgentOnboardPage', () => ({
  default: () => <div data-testid="agent-onboard-flow-page" />,
}));

describe('AgentAddRoute', () => {
  it('runs the onboarding flow when no administration mode is declared', () => {
    render(<AgentAddRoute />);

    expect(screen.getByTestId('agent-onboard-flow-page')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-add-form-page')).not.toBeInTheDocument();
  });

  it('runs the onboarding flow in flow mode', () => {
    render(
      <AdministrationProvider administration={{mode: AdministrationModes.FLOW}}>
        <AgentAddRoute />
      </AdministrationProvider>,
    );

    expect(screen.getByTestId('agent-onboard-flow-page')).toBeInTheDocument();
  });

  it('shows the form in native mode', () => {
    render(
      <AdministrationProvider administration={{mode: AdministrationModes.NATIVE}}>
        <AgentAddRoute />
      </AdministrationProvider>,
    );

    expect(screen.getByTestId('agent-add-form-page')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-onboard-flow-page')).not.toBeInTheDocument();
  });

  it('follows a feature-level override over the console-wide mode', () => {
    render(
      <AdministrationProvider
        administration={{mode: AdministrationModes.NATIVE, features: {agents: {mode: AdministrationModes.FLOW}}}}
      >
        <AgentAddRoute />
      </AdministrationProvider>,
    );

    expect(screen.getByTestId('agent-onboard-flow-page')).toBeInTheDocument();
  });
});
