// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import UserAddFormPage from '../UserAddFormPage';

const mockNavigate = vi.fn();
const mockHttpRequest = vi.fn<(config: RecordedRequest) => Promise<{data: unknown}>>();
const mockUseGetOrganizationUnits = vi.fn<(params: {placement?: boolean}, enabled: boolean) => unknown>();

// Translations are shared across the suite, so keys are asserted rather than English copy.
vi.mock('react-i18next', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useTranslation: () => ({
      t: (key: string, _fallback?: string, options?: {roles?: string}) =>
        options?.roles ? `${key} ${options.roles}` : key,
    }),
  };
});

vi.mock('react-router', async () => ({
  ...(await vi.importActual<typeof import('react-router')>('react-router')),
  useNavigate: () => mockNavigate,
}));

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useThunderID: () => ({http: {request: mockHttpRequest}}),
  };
});

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetOrganizationUnits: (params: {placement?: boolean}, enabled: boolean) =>
    mockUseGetOrganizationUnits(params, enabled),
  OrganizationUnitTreePicker: ({
    value,
    onChange,
    rootOuId = undefined,
    projectId = undefined,
    withinOuId = undefined,
  }: {
    value: string;
    onChange: (ouId: string) => void;
    rootOuId?: string;
    projectId?: string;
    withinOuId?: string;
  }) => (
    <>
      <button
        type="button"
        data-testid="ou-picker"
        data-root={rootOuId}
        data-project-id={projectId}
        data-within={withinOuId}
        data-value={value}
        onClick={() => onChange('ou-child')}
      >
        pick ou
      </button>
      {projectId && (
        <button type="button" data-testid="ou-picker-system" onClick={() => onChange('system-ou')}>
          pick system ou
        </button>
      )}
      {projectId && withinOuId && (
        <button type="button" data-testid="ou-picker-within" onClick={() => onChange(withinOuId)}>
          pick the type organization unit
        </button>
      )}
    </>
  ),
}));

interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

const userTypes = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  types: [{id: 'type-1', handle: 'customer', displayName: 'Customer', ouId: 'ou-root'}],
};

const customerType = {
  id: 'type-1',
  handle: 'customer',
  displayName: 'Customer',
  schema: {
    username: {type: 'string', required: true},
    given_name: {type: 'string'},
  },
};

function requests(): RecordedRequest[] {
  return mockHttpRequest.mock.calls.map(([config]) => config);
}

describe('UserAddFormPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetOrganizationUnits.mockReturnValue({data: undefined, isLoading: false});
    mockHttpRequest.mockImplementation((config: RecordedRequest) => {
      if (config.method === 'POST' && config.url.endsWith('/users')) {
        return Promise.resolve({data: {id: 'user-1'}});
      }
      if (config.url.endsWith('/user-types/type-1')) {
        return Promise.resolve({data: customerType});
      }
      if (config.url.includes('/user-types')) {
        return Promise.resolve({data: userTypes});
      }
      return Promise.reject(new Error(`unexpected request: ${config.url}`));
    });
  });

  it('posts the chosen type, organization unit and schema attributes to /users', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));

    const picker = await screen.findByTestId('ou-picker');
    expect(picker).toHaveAttribute('data-root', 'ou-root');
    await user.click(picker);

    await user.type(await screen.findByRole('textbox', {name: /username/i}), 'alice');
    await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));

    await waitFor(() => {
      expect(requests().some((request) => request.method === 'POST')).toBe(true);
    });

    const post = requests().find((request) => request.method === 'POST');
    expect(post?.url).toMatch(/\/users$/);
    // The optional field left empty is omitted.
    expect(post?.data).toEqual({ouId: 'ou-child', type: 'customer', attributes: {username: 'alice'}});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });

    // No flow is involved anywhere along the way.
    expect(requests().some((request) => request.url.includes('/flow/execute'))).toBe(false);
    expect(requests().some((request) => request.url.includes('/server-config/flow'))).toBe(false);
  });

  it('asks for a full page of types and posts nothing a previously chosen type was given', async () => {
    const twoTypes = {
      ...userTypes,
      totalResults: 2,
      count: 2,
      types: [...userTypes.types, {id: 'type-2', handle: 'employee', displayName: 'Employee', ouId: 'ou-root'}],
    };
    const employeeType = {...customerType, id: 'type-2', handle: 'employee', displayName: 'Employee'};
    mockHttpRequest.mockImplementation((config: RecordedRequest) => {
      if (config.method === 'POST') return Promise.resolve({data: {id: 'user-2'}});
      if (config.url.endsWith('/user-types/type-1')) return Promise.resolve({data: customerType});
      if (config.url.endsWith('/user-types/type-2')) return Promise.resolve({data: employeeType});
      return Promise.resolve({data: twoTypes});
    });
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    const choose = async (type: string): Promise<void> => {
      await user.click(screen.getByRole('combobox'));
      await user.click(within(await screen.findByRole('listbox')).getByText(type));
    };
    await screen.findByRole('combobox');
    await choose('Customer');
    await screen.findByRole('textbox', {name: /given_name/i});
    await choose('Employee');
    await user.type(await screen.findByRole('textbox', {name: /given_name/i}), 'ally');
    // Back to a type whose schema is already loaded, so its fields stay mounted across the change.
    await choose('Customer');
    await user.click(await screen.findByTestId('ou-picker'));
    await waitFor(() => expect(screen.getByRole('textbox', {name: /given_name/i})).toHaveValue(''));
    await user.type(screen.getByRole('textbox', {name: /username/i}), 'bob');
    await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toEqual({
        ouId: 'ou-child',
        type: 'customer',
        attributes: {username: 'bob'},
      });
    });
    expect(requests().some((request) => /\/user-types\?.*limit=100/.test(request.url))).toBe(true);
  });

  it('keeps submit disabled until a type is chosen', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    const submit = await screen.findByRole('button', {name: 'users:addForm.submit'});
    expect(submit).toBeDisabled();

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
    await screen.findByRole('textbox', {name: /username/i});
    expect(submit).toBeEnabled();
  });

  it('does not look up project organization units outside a project', () => {
    render(<UserAddFormPage />);

    expect(mockUseGetOrganizationUnits).toHaveBeenCalledWith({limit: 1, offset: 0, projectId: undefined}, false);
    expect(mockUseGetOrganizationUnits).toHaveBeenCalledWith(expect.objectContaining({placement: true}), false);
    expect(mockUseGetOrganizationUnits).not.toHaveBeenCalledWith(expect.anything(), true);
  });

  describe('an administrator, added outside any project', () => {
    // The roles come from the console's config, as a packaged Control Plane's names them.
    function giveAdministrators(roleIds: string[]): void {
      window.__THUNDERID_RUNTIME_CONFIG__ = {
        ...window.__THUNDERID_RUNTIME_CONFIG__!,
        administrators: {role_ids: roleIds},
      };
    }

    afterEach(() => {
      const config = {...window.__THUNDERID_RUNTIME_CONFIG__!};
      delete config.administrators;
      window.__THUNDERID_RUNTIME_CONFIG__ = config;
    });

    async function create(user: ReturnType<typeof userEvent.setup>, page: JSX.Element): Promise<void> {
      render(page);
      await user.click(await screen.findByRole('combobox'));
      await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
      await user.type(await screen.findByRole('textbox', {name: /username/i}), 'erin');
      await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));
    }

    function assigned(): RecordedRequest[] {
      return requests().filter((request) => request.url.includes('/assignments/add'));
    }

    it('gives the administrators roles once created, then opens the list', async () => {
      const fallback = mockHttpRequest.getMockImplementation();
      const finishAssignments: (() => void)[] = [];
      mockHttpRequest.mockImplementation((config: RecordedRequest) =>
        config.url.includes('/assignments/add')
          ? new Promise((resolve) => {
              finishAssignments.push(() => resolve({data: undefined}));
            })
          : fallback!(config),
      );
      const user = userEvent.setup();
      giveAdministrators(['admins', 'auditors']);
      await create(user, <UserAddFormPage />);

      // The list opens only once every role is assigned, and the user cannot be submitted again meanwhile.
      for (const count of [1, 2]) {
        await waitFor(() => expect(assigned()).toHaveLength(count));
        expect(mockNavigate).not.toHaveBeenCalled();
        expect(screen.getByRole('button', {name: 'users:addForm.creating'})).toBeDisabled();
        finishAssignments[count - 1]();
      }

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/users'));
      expect(assigned().map((request) => [request.url.replace(/^.*\/roles\//, ''), request.data])).toEqual([
        ['admins/assignments/add', {assignments: [{id: 'user-1', type: 'user'}]}],
        ['auditors/assignments/add', {assignments: [{id: 'user-1', type: 'user'}]}],
      ]);
    });

    it('tries every role, names those it could not assign, stays, and cannot create the user again', async () => {
      const fallback = mockHttpRequest.getMockImplementation();
      let refuseAdmins = true;
      mockHttpRequest.mockImplementation((config: RecordedRequest) => {
        if (config.url.includes('/admins/assignments/add') && refuseAdmins) {
          return Promise.reject(new Error('forbidden'));
        }
        if (config.method === 'GET' && config.url.endsWith('/roles/admins')) {
          return Promise.resolve({data: {name: 'Administrators'}});
        }
        return config.url.includes('/assignments/add') ? Promise.resolve({data: undefined}) : fallback!(config);
      });
      const user = userEvent.setup();
      giveAdministrators(['admins', 'auditors']);
      await create(user, <UserAddFormPage />);

      expect(await screen.findByTestId('administrator-roles-error')).toHaveTextContent(
        'users:addForm.administratorRolesError Administrators',
      );
      expect(screen.getByTestId('administrator-roles-error')).not.toHaveTextContent('auditors');
      expect(assigned().map((request) => request.url.replace(/^.*\/roles\//, ''))).toEqual([
        'admins/assignments/add',
        'auditors/assignments/add',
      ]);
      expect(mockNavigate).not.toHaveBeenCalled();
      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();

      // Trying again assigns only the role that failed, then opens the list.
      refuseAdmins = false;
      await user.click(screen.getByRole('button', {name: 'users:addForm.retryRoles'}));

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/users'));
      expect(assigned().map((request) => request.url.replace(/^.*\/roles\//, ''))).toEqual([
        'admins/assignments/add',
        'auditors/assignments/add',
        'admins/assignments/add',
      ]);
      expect(assigned().at(-1)?.data).toEqual({assignments: [{id: 'user-1', type: 'user'}]});
    });

    it('still gives the roles when the page is closed while the user is being created', async () => {
      const fallback = mockHttpRequest.getMockImplementation();
      let finishCreate: (response: {data: {id: string}}) => void = () => undefined;
      mockHttpRequest.mockImplementation((config: RecordedRequest) => {
        if (config.method === 'POST' && config.url.endsWith('/users')) {
          return new Promise((resolve) => {
            finishCreate = resolve;
          });
        }
        return config.url.includes('/assignments/add') ? Promise.resolve({data: undefined}) : fallback!(config);
      });
      const user = userEvent.setup();
      giveAdministrators(['admins']);
      const {unmount} = render(<UserAddFormPage />);
      await user.click(await screen.findByRole('combobox'));
      await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
      await user.type(await screen.findByRole('textbox', {name: /username/i}), 'erin');
      await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));
      await waitFor(() => expect(requests().some((request) => request.url.endsWith('/users'))).toBe(true));

      unmount();
      finishCreate({data: {id: 'user-1'}});

      await waitFor(() =>
        expect(assigned().map((request) => request.url.replace(/^.*\/roles\//, ''))).toEqual([
          'admins/assignments/add',
        ]),
      );
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('gives no role in a project', async () => {
      mockUseGetOrganizationUnits.mockImplementation((params: {placement?: boolean}) =>
        params.placement
          ? {data: {totalResults: 1}, isLoading: false}
          : {data: {totalResults: 0, organizationUnits: []}, isLoading: false},
      );
      giveAdministrators(['admins']);
      const user = userEvent.setup();
      await create(
        user,
        <ProjectScopeProvider project={{id: 'project-1'}}>
          <UserAddFormPage />
        </ProjectScopeProvider>,
      );

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/users'));
      expect(assigned()).toEqual([]);
    });
  });

  describe('inside a project', () => {
    afterEach(() => {
      userTypes.types[0].ouId = 'ou-root';
    });

    async function fillInProject(user: ReturnType<typeof userEvent.setup>): Promise<void> {
      render(
        <ProjectScopeProvider project={{id: 'project-1'}}>
          <UserAddFormPage />
        </ProjectScopeProvider>,
      );

      await user.click(await screen.findByRole('combobox'));
      await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
      await user.type(await screen.findByRole('textbox', {name: /username/i}), 'dave');
    }

    // The project's own organization units, and the placements the selected type allows in the project.
    function mockLookups(projectOus: number, placements: number, placementsLoading = false): void {
      mockUseGetOrganizationUnits.mockImplementation((params: {placement?: boolean}) =>
        params.placement
          ? {data: placementsLoading ? undefined : {totalResults: placements}, isLoading: placementsLoading}
          : {data: {totalResults: projectOus, organizationUnits: []}, isLoading: false},
      );
    }

    async function posted(user: ReturnType<typeof userEvent.setup>): Promise<unknown> {
      await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));
      await waitFor(() => {
        expect(requests().some((request) => request.method === 'POST')).toBe(true);
      });
      return requests().find((request) => request.method === 'POST')?.data;
    }

    it('creates the user in the selected project, without an organization unit to pick when the project has none', async () => {
      mockLookups(0, 1);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(mockUseGetOrganizationUnits).toHaveBeenCalledWith({limit: 1, offset: 0, projectId: 'project-1'}, true);
      expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
      expect(await posted(user)).toEqual({projectId: 'project-1', type: 'customer', attributes: {username: 'dave'}});
    });

    it('offers the placements of a type in the system organization unit and creates the user where the picker selects', async () => {
      mockLookups(1, 2);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(mockUseGetOrganizationUnits).toHaveBeenCalledWith(
        {limit: 1, offset: 0, projectId: 'project-1', placement: true, within: 'ou-root'},
        true,
      );
      const picker = screen.getByTestId('ou-picker');
      expect(picker).toHaveAttribute('data-project-id', 'project-1');
      expect(picker).toHaveAttribute('data-within', 'ou-root');
      expect(picker).toHaveAttribute('data-value', '');
      expect(picker).not.toHaveAttribute('data-root');
      await user.click(screen.getByTestId('ou-picker-system'));

      expect(await posted(user)).toEqual({
        projectId: 'project-1',
        ouId: 'system-ou',
        type: 'customer',
        attributes: {username: 'dave'},
      });
    });

    it('offers only the subtree of a type in a project organization unit and creates the user in it', async () => {
      userTypes.types[0].ouId = 'ou-shop';
      mockLookups(1, 1);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(screen.getByTestId('ou-picker')).toHaveAttribute('data-within', 'ou-shop');
      await user.click(screen.getByTestId('ou-picker-within'));

      expect(await posted(user)).toEqual({
        projectId: 'project-1',
        ouId: 'ou-shop',
        type: 'customer',
        attributes: {username: 'dave'},
      });
    });

    it('creates the user in the picked project organization unit', async () => {
      mockLookups(1, 2);
      const user = userEvent.setup();
      await fillInProject(user);

      await user.click(screen.getByTestId('ou-picker'));

      expect(await posted(user)).toEqual({
        projectId: 'project-1',
        ouId: 'ou-child',
        type: 'customer',
        attributes: {username: 'dave'},
      });
    });

    it('says the type cannot be used in the project when it allows no placement there, and keeps submit disabled', async () => {
      mockLookups(1, 0);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(screen.getByText('users:addForm.typeOutsideProject')).toBeInTheDocument();
      expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();
    });

    it('says the type cannot be used in a project without organization units when it is outside the system organization unit', async () => {
      userTypes.types[0].ouId = 'ou-elsewhere';
      mockLookups(0, 0);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(mockUseGetOrganizationUnits).toHaveBeenCalledWith(
        {limit: 1, offset: 0, projectId: 'project-1', placement: true, within: 'ou-elsewhere'},
        true,
      );
      expect(screen.getByText('users:addForm.typeOutsideProject')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();
    });

    it('keeps submit disabled while the project organization units are loading', async () => {
      mockUseGetOrganizationUnits.mockImplementation((params: {placement?: boolean}) =>
        params.placement ? {data: {totalResults: 1}, isLoading: false} : {data: undefined, isLoading: true},
      );
      const user = userEvent.setup();
      await fillInProject(user);

      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();
    });

    it("keeps submit disabled while the type's placements are loading", async () => {
      mockLookups(0, 0, true);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(screen.queryByText('users:addForm.typeOutsideProject')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();
    });

    it('keeps submit disabled until the picker selects an organization unit', async () => {
      mockLookups(1, 2);
      const user = userEvent.setup();
      await fillInProject(user);

      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();
      await user.click(screen.getByTestId('ou-picker'));
      expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeEnabled();
    });

    it.each([true, false])(
      'says so when a placement read fails (the placements: %s), keeps submit disabled, and retries the failed read',
      async (placementsFail) => {
        const refetchProjectOus = vi.fn();
        const refetchPlacements = vi.fn();
        const failure = new Error('network down');
        mockUseGetOrganizationUnits.mockImplementation((params: {placement?: boolean}) =>
          params.placement
            ? placementsFail
              ? {data: undefined, isLoading: false, error: failure, refetch: refetchPlacements}
              : {data: {totalResults: 1}, isLoading: false, error: null, refetch: refetchPlacements}
            : placementsFail
              ? {data: {totalResults: 1}, isLoading: false, error: null, refetch: refetchProjectOus}
              : {data: undefined, isLoading: false, error: failure, refetch: refetchProjectOus},
        );
        const user = userEvent.setup();
        await fillInProject(user);

        expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
        expect(screen.queryByText('users:addForm.typeOutsideProject')).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();

        await user.click(screen.getByRole('button', {name: 'common:actions.refresh'}));
        expect(refetchPlacements).toHaveBeenCalledTimes(placementsFail ? 1 : 0);
        expect(refetchProjectOus).toHaveBeenCalledTimes(placementsFail ? 0 : 1);
      },
    );
  });

  it("preselects the chosen type's organization unit and posts it when no other is picked", async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));

    expect(await screen.findByTestId('ou-picker')).toHaveAttribute('data-value', 'ou-root');

    await user.type(await screen.findByRole('textbox', {name: /username/i}), 'carol');
    await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toEqual({
        ouId: 'ou-root',
        type: 'customer',
        attributes: {username: 'carol'},
      });
    });
  });

  it('clears the organization unit when the type is cleared again', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
    expect(await screen.findByTestId('ou-picker')).toHaveAttribute('data-value', 'ou-root');

    await user.click(screen.getByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('users:addForm.selectUserType'));

    await waitFor(() => {
      expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
    });
    expect(screen.getByRole('button', {name: 'users:addForm.submit'})).toBeDisabled();
  });

  it('does not post when a required attribute is missing', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
    await user.click(await screen.findByTestId('ou-picker'));
    await screen.findByRole('textbox', {name: /username/i});
    await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));

    await screen.findByText(/username is required/i);
    expect(requests().some((request) => request.method === 'POST')).toBe(false);
  });

  it('says so when no user types exist', async () => {
    mockHttpRequest.mockResolvedValue({data: {totalResults: 0, startIndex: 1, count: 0, types: []}});
    render(<UserAddFormPage />);

    expect(await screen.findByText('users:addForm.noUserTypes')).toBeInTheDocument();
  });

  it('returns to the user list on cancel', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('button', {name: 'common:actions.cancel'}));

    expect(mockNavigate).toHaveBeenCalledWith('/users');
  });
});
