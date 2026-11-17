// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserRolesSection from '../UserRolesSection';

interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

const mockHttpRequest = vi.fn<(config: RecordedRequest) => Promise<{data: unknown}>>();

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useThunderID: () => ({http: {request: mockHttpRequest}}),
  };
});

const editor = {id: 'role-editor', name: 'Editor', ouId: 'ou-1', projectId: 'project-1'};
const viewer = {id: 'role-viewer', name: 'Viewer', ouId: 'ou-1', projectId: 'project-1'};
const auditor = {id: 'role-auditor', name: 'Auditor', ouId: 'ou-1'};

let held = [editor, auditor];
let failing: string[] = [];
let mandated: string[] = [];
// The organization unit an assignment is made in, by role ID, where the API reports one.
let assignedIn: Record<string, string> = {};
// Holds back the response of an add until a test releases it.
let releaseAdd: (() => void) | undefined;
let holdAdds = false;

function writes(): RecordedRequest[] {
  return mockHttpRequest.mock.calls.map(([config]) => config).filter((config) => config.method === 'POST');
}

describe('UserRolesSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    held = [editor, auditor];
    failing = [];
    mandated = [];
    assignedIn = {};
    holdAdds = false;
    releaseAdd = undefined;
    mockHttpRequest.mockImplementation((config: RecordedRequest) => {
      const {pathname, searchParams} = new URL(config.url);
      if (config.method === 'POST') {
        if (failing.some((roleId) => pathname.includes(`/roles/${roleId}/`))) {
          return Promise.reject(new Error('forbidden'));
        }
        const role = [editor, viewer, auditor].find((candidate) => pathname.includes(`/roles/${candidate.id}/`))!;
        held = pathname.endsWith('/add') ? [...held, role] : held.filter((candidate) => candidate.id !== role.id);
        if (holdAdds && pathname.endsWith('/add')) {
          return new Promise((resolve) => {
            releaseAdd = () => resolve({data: undefined});
          });
        }
        return Promise.resolve({data: undefined});
      }
      if (pathname.endsWith('/assignments')) {
        const assignments = held
          .filter((role) => pathname.includes(`/roles/${role.id}/`))
          .map((role) => ({
            id: 'user-1',
            type: 'user',
            isReadOnly: mandated.includes(role.id),
            ...(assignedIn[role.id] ? {ouId: assignedIn[role.id]} : {}),
          }));
        return Promise.resolve({data: {totalResults: assignments.length, assignments}});
      }
      if (pathname.endsWith('/roles')) {
        const roles = searchParams.has('projectId') ? [editor, viewer] : [editor, viewer, auditor];
        return Promise.resolve({data: {totalResults: roles.length, count: roles.length, roles}});
      }
      const named = [editor, viewer, auditor].find((candidate) => pathname.endsWith(`/roles/${candidate.id}`));
      return named ? Promise.resolve({data: named}) : Promise.reject(new Error(`unexpected request: ${config.url}`));
    });
  });

  async function openAddDialog(user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> {
    await user.click(await screen.findByRole('button', {name: 'Add'}));
    return screen.findByRole('dialog', {name: 'Add roles'});
  }

  async function pick(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement, name: string): Promise<void> {
    const row = (await within(dialog).findByText(name)).closest('[role="row"]')!;
    await user.click(within(row as HTMLElement).getByRole('checkbox'));
  }

  it('lists every role assigned to the user outside a project, by name and ID', async () => {
    render(<UserRolesSection userId="user-1" />);

    expect(await screen.findByText('Editor')).toBeInTheDocument();
    expect(screen.getByText('role-editor')).toBeInTheDocument();
    expect(screen.getByText('Auditor')).toBeInTheDocument();
    expect(screen.queryByText('Viewer')).not.toBeInTheDocument();
  });

  it("lists and offers only the project's roles inside a project", async () => {
    const user = userEvent.setup();
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <UserRolesSection userId="user-1" />
      </ProjectScopeProvider>,
    );

    expect(await screen.findByText('Editor')).toBeInTheDocument();
    expect(screen.queryByText('Auditor')).not.toBeInTheDocument();

    const dialog = await openAddDialog(user);
    expect(await within(dialog).findByText('Viewer')).toBeInTheDocument();
    expect(within(dialog).queryByText('Editor')).not.toBeInTheDocument();
    expect(within(dialog).queryByText('Auditor')).not.toBeInTheDocument();
  });

  it("offers nothing to add until the user's roles are read", async () => {
    const serve = mockHttpRequest.getMockImplementation()!;
    let release: () => void = () => undefined;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    mockHttpRequest.mockImplementation(async (config: RecordedRequest) => {
      if (config.method === 'GET' && new URL(config.url).pathname.endsWith('/assignments')) {
        await held;
      }
      return serve(config);
    });
    render(<UserRolesSection userId="user-1" />);

    await waitFor(() =>
      expect(mockHttpRequest.mock.calls.some(([config]) => config.url.includes('/assignments'))).toBe(true),
    );
    expect(screen.queryByRole('button', {name: 'Add'})).not.toBeInTheDocument();

    release();
    expect(await screen.findByRole('button', {name: 'Add'})).toBeInTheDocument();
  });

  it('removes a role from the organization unit the assignment is made in', async () => {
    assignedIn = {'role-auditor': 'ou-child'};
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await user.click(await screen.findByRole('button', {name: 'Remove Auditor'}));

    await waitFor(() =>
      expect(writes()).toEqual([
        expect.objectContaining({
          url: expect.stringContaining('/roles/role-auditor/assignments/remove') as string,
          data: {ouId: 'ou-child', assignments: [{id: 'user-1', type: 'user'}]},
        }),
      ]),
    );
  });

  it('keeps the dialog locked when the selection changes while roles are being assigned', async () => {
    held = [];
    holdAdds = true;
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    const dialog = await openAddDialog(user);
    await pick(user, dialog, 'Viewer');
    await user.click(within(dialog).getByRole('button', {name: 'Add (1)'}));
    await waitFor(() => expect(within(dialog).getByRole('button', {name: 'Cancel'})).toBeDisabled());

    await pick(user, dialog, 'Editor');

    expect(within(dialog).getByRole('button', {name: 'Cancel'})).toBeDisabled();
    expect(within(dialog).getByRole('button', {name: 'Add (2)'})).toBeDisabled();
    expect(writes()).toHaveLength(1);

    releaseAdd!();

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(writes()).toHaveLength(1);
  });

  it('assigns the roles picked in the dialog, then closes it', async () => {
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await screen.findByText('Editor');
    const dialog = await openAddDialog(user);
    expect(within(dialog).getByRole('button', {name: 'Add'})).toBeDisabled();
    await pick(user, dialog, 'Viewer');
    await user.click(within(dialog).getByRole('button', {name: 'Add (1)'}));

    await waitFor(() =>
      expect(writes()).toEqual([
        expect.objectContaining({
          url: expect.stringContaining('/roles/role-viewer/assignments/add') as string,
          data: {assignments: [{id: 'user-1', type: 'user'}]},
        }),
      ]),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(await screen.findByText('Viewer')).toBeInTheDocument();
  });

  it('removes a role from the user', async () => {
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await user.click(await screen.findByRole('button', {name: 'Remove Auditor'}));

    await waitFor(() =>
      expect(writes()).toEqual([
        expect.objectContaining({
          url: expect.stringContaining('/roles/role-auditor/assignments/remove') as string,
          data: {assignments: [{id: 'user-1', type: 'user'}]},
        }),
      ]),
    );
    await waitFor(() => expect(screen.queryByText('Auditor')).not.toBeInTheDocument());
  });

  it('names the roles it could not assign and keeps the dialog open', async () => {
    failing = ['role-viewer'];
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await screen.findByText('Editor');
    const dialog = await openAddDialog(user);
    await pick(user, dialog, 'Viewer');
    await user.click(within(dialog).getByRole('button', {name: 'Add (1)'}));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Could not assign the roles Viewer. Try again.');

    // Changing the selection clears the failed attempt.
    await pick(user, dialog, 'Viewer');

    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
  });

  it('says so when a role could not be removed', async () => {
    failing = ['role-auditor'];
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await user.click(await screen.findByRole('button', {name: 'Remove Auditor'}));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Failed to remove the role. Please try again.');
    expect(screen.getByText('Auditor')).toBeInTheDocument();

    await user.click(within(alert).getByRole('button', {name: 'Close'}));

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('marks a role a share mandates read only, with nothing to remove it', async () => {
    mandated = ['role-auditor'];
    render(<UserRolesSection userId="user-1" />);

    expect(await screen.findByRole('button', {name: 'Remove Editor'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Remove Auditor'})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Read only'})).toBeInTheDocument();
  });

  it('offers nothing to add or remove to an admin who may not update roles', async () => {
    render(
      <CapabilitiesProvider
        capabilities={{
          can: (_resource: string, action = 'view') => action === 'view',
          canManageRoot: false,
          isLoading: false,
        }}
      >
        <UserRolesSection userId="user-1" />
      </CapabilitiesProvider>,
    );

    expect(await screen.findByText('Editor')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Remove Editor'})).not.toBeInTheDocument();
  });

  it('says so when the user has no roles', async () => {
    held = [];
    render(<UserRolesSection userId="user-1" />);

    expect(await screen.findByText('This user does not have any roles assigned.')).toBeInTheDocument();
  });

  it('says so when the roles cannot be loaded', async () => {
    mockHttpRequest.mockRejectedValue(new Error('unavailable'));
    render(<UserRolesSection userId="user-1" />);

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add'})).not.toBeInTheDocument();
  });

  it('reads the roles again on retry once they can be loaded', async () => {
    const respond = mockHttpRequest.getMockImplementation()!;
    mockHttpRequest.mockRejectedValue(new Error('unavailable'));
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await screen.findByRole('alert');
    mockHttpRequest.mockImplementation(respond);
    await user.click(screen.getByRole('button', {name: 'Refresh'}));

    expect(await screen.findByText('Editor')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it("reads the user's assignments again on retry when only they could not be loaded", async () => {
    const respond = mockHttpRequest.getMockImplementation()!;
    mockHttpRequest.mockImplementation((config: RecordedRequest) =>
      new URL(config.url).pathname.endsWith('/assignments')
        ? Promise.reject(new Error('unavailable'))
        : respond(config),
    );
    const user = userEvent.setup();
    render(<UserRolesSection userId="user-1" />);

    await screen.findByRole('alert');
    mockHttpRequest.mockImplementation(respond);
    await user.click(screen.getByRole('button', {name: 'Refresh'}));

    expect(await screen.findByText('Editor')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
