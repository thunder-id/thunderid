// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import RollbackExecutorProperties from '../RollbackExecutorProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

function makeResource(properties?: Record<string, unknown>): Resource {
  return {
    data: {action: {executor: {name: 'OUDeleteExecutor'}, type: 'EXECUTOR'}, properties},
    id: 'rollback_1',
  } as unknown as Resource;
}

describe('RollbackExecutorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should show forwardFailure unchecked by default', () => {
    render(<RollbackExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(screen.getByRole('checkbox')).not.toBeChecked();
  });

  it('should show forwardFailure checked when the property is true', () => {
    render(<RollbackExecutorProperties resource={makeResource({forwardFailure: true})} onChange={mockOnChange} />);

    expect(screen.getByRole('checkbox')).toBeChecked();
  });

  it('should report the new value when toggled', async () => {
    const user = userEvent.setup();
    const resource = makeResource();
    render(<RollbackExecutorProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('checkbox'));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.forwardFailure', true, resource);
  });
});
