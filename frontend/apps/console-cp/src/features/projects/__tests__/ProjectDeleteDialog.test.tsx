// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ProjectDeleteDialog from '../ProjectDeleteDialog';
import type {Project} from '../useGetProjects';

const mockMutate = vi.fn<(id: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => void>();

vi.mock('../useDeleteProject', () => ({
  default: () => ({mutate: mockMutate, isPending: false, reset: vi.fn()}),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    // Keys translate to themselves, except where a default value is given: there the key is
    // treated as missing from the locale, as the API's own error keys are.
    t: (key: string, options?: unknown) =>
      typeof options === 'object' && options !== null && 'defaultValue' in options
        ? (options as {defaultValue: string}).defaultValue
        : key,
  }),
}));

const project: Project = {
  id: 'p1',
  handle: 'shop',
  displayName: 'Shop',
  rootOuId: 'ou1',
  createdAt: '2026-10-01T00:00:00Z',
};

describe('ProjectDeleteDialog', () => {
  beforeEach(() => {
    mockMutate.mockReset();
  });

  it('is closed while no project is set', () => {
    render(<ProjectDeleteDialog project={null} onClose={vi.fn()} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('deletes the project and closes', async () => {
    mockMutate.mockImplementation((_id, options) => options.onSuccess?.());
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<ProjectDeleteDialog project={project} onClose={onClose} />);

    await user.click(screen.getByRole('button', {name: 'common:actions.delete'}));

    expect(mockMutate).toHaveBeenCalledWith('p1', expect.anything());
    expect(onClose).toHaveBeenCalled();
  });

  it('stays open and shows the error when the delete fails', async () => {
    mockMutate.mockImplementation((_id, options) => options.onError?.(new Error('Forbidden')));
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<ProjectDeleteDialog project={project} onClose={onClose} />);

    await user.click(screen.getByRole('button', {name: 'common:actions.delete'}));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getAllByRole('alert')).toHaveLength(2);
  });

  it('shows the reason the API gives for refusing the delete', async () => {
    const description =
      'Cannot delete an organization unit that has child organization units, users, groups, user types, agent types, connections, or sharing policies it issued';
    mockMutate.mockImplementation((_id, options) =>
      options.onError?.(
        Object.assign(new Error('Request failed with status code 409'), {
          response: {
            data: {
              code: 'OU-1006',
              message: {
                key: 'error.ouservice.organization_unit_has_children',
                defaultValue: 'Organization unit has children',
              },
              description: {
                key: 'error.ouservice.organization_unit_has_children_description',
                defaultValue: description,
              },
            },
          },
        }),
      ),
    );
    const user = userEvent.setup();
    render(<ProjectDeleteDialog project={project} onClose={vi.fn()} />);

    await user.click(screen.getByRole('button', {name: 'common:actions.delete'}));

    expect(screen.getByText(description)).toBeInTheDocument();
  });

  it('closes without deleting when cancelled', async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<ProjectDeleteDialog project={project} onClose={onClose} />);

    await user.click(screen.getByRole('button', {name: 'common:actions.cancel'}));

    expect(mockMutate).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });
});
