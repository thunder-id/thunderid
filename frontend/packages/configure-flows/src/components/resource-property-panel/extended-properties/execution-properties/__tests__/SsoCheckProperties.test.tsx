// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Node} from '@xyflow/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import SsoCheckProperties from '../SsoCheckProperties';

const {mockFlowNodes} = vi.hoisted(() => ({
  mockFlowNodes: {value: [] as unknown[]},
}));

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

vi.mock('../../../../../hooks/useFlowConfig', () => ({
  default: () => ({flowNodes: mockFlowNodes.value}),
}));

function makeExecutionNode(id: string, executorName: string): Node {
  return {
    data: {action: {executor: {name: executorName}, type: 'EXECUTOR'}},
    id,
    position: {x: 0, y: 0},
    type: 'TASK_EXECUTION',
  };
}

function makeSsoCheckResource(checkpointRef: string): Resource {
  return {
    data: {
      action: {executor: {name: 'SSOCheckExecutor'}, type: 'EXECUTOR'},
      properties: {checkpointRef},
    },
    id: 'sso_check_1',
  } as unknown as Resource;
}

describe('SsoCheckProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockFlowNodes.value = [
      makeExecutionNode('session_1', 'SessionExecutor'),
      makeExecutionNode('session_2', 'SessionExecutor'),
      makeExecutionNode('credentials_auth', 'CredentialsAuthExecutor'),
    ];
  });

  it('should render the checkpoint picker label', () => {
    render(<SsoCheckProperties resource={makeSsoCheckResource('session_1')} onChange={mockOnChange} />);

    expect(screen.getByText('Session checkpoint')).toBeInTheDocument();
    expect(screen.queryByText(/no longer exists/)).not.toBeInTheDocument();
  });

  it('should list only session nodes as options and report a selection', async () => {
    const user = userEvent.setup();
    render(<SsoCheckProperties resource={makeSsoCheckResource('session_1')} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));

    expect(screen.getByRole('option', {name: 'session_2'})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'credentials_auth'})).not.toBeInTheDocument();

    await user.click(screen.getByRole('option', {name: 'session_2'}));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.checkpointRef', 'session_2', expect.anything());
  });

  it('should warn when the referenced session step no longer exists', () => {
    render(<SsoCheckProperties resource={makeSsoCheckResource('session_gone')} onChange={mockOnChange} />);

    expect(
      screen.getByText('The referenced session step no longer exists. Select a valid session step.'),
    ).toBeInTheDocument();
  });

  it('should default to no checkpoint selected and no warning when the node has none configured', () => {
    const resource = {
      data: {
        action: {executor: {name: 'SSOCheckExecutor'}, type: 'EXECUTOR'},
        properties: {},
      },
      id: 'sso_check_1',
    } as unknown as Resource;

    render(<SsoCheckProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.queryByText(/no longer exists/)).not.toBeInTheDocument();
    expect(screen.getByRole('combobox')).not.toHaveTextContent(/session_/);
  });
});
