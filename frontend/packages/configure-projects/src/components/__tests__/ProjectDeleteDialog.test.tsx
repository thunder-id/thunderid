// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../../models/project';
import ProjectDeleteDialog from '../ProjectDeleteDialog';

const mockMutate = vi.fn<(id: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => void>();

vi.mock('../../api/useDeleteProject', () => ({
  default: () => ({mutate: mockMutate, isPending: false, reset: vi.fn()}),
}));

const catalog: Record<string, string> = {
  'common:errors.PRJ-1013': 'This project still has resources.',
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    // Keys in the catalog below translate to their message. Other keys translate to themselves, except
    // where a default value is given: there the key is treated as missing from the locale.
    t: (key: string, options?: unknown) =>
      catalog[key] ??
      (typeof options === 'object' && options !== null && 'defaultValue' in options
        ? (options as {defaultValue: string}).defaultValue
        : key),
  }),
}));

const project: Project = {
  id: 'p1',
  handle: 'shop',
  displayName: 'Shop',
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

  it('shows the console message for the code the API refuses the delete with', async () => {
    const description = "Delete or move the project's applications and agents before deleting the project";
    mockMutate.mockImplementation((_id, options) =>
      options.onError?.(
        Object.assign(new Error('Request failed with status code 409'), {
          response: {
            data: {
              code: 'PRJ-1013',
              message: {
                key: 'error.projectservice.project_not_empty',
                defaultValue: 'Project is not empty',
              },
              description: {
                key: 'error.projectservice.project_not_empty_description',
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

    expect(screen.getByText('This project still has resources.')).toBeInTheDocument();
    expect(screen.queryByText(description)).not.toBeInTheDocument();
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
