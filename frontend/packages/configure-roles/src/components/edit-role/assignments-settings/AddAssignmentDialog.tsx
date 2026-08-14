// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetApplications} from '@thunderid/configure-applications';
import type {BasicApplication} from '@thunderid/configure-applications';
import {useGetGroups} from '@thunderid/configure-groups';
import type {GroupBasic} from '@thunderid/configure-groups';
import {useGetUsers} from '@thunderid/configure-users';
import {useDataGridLocaleText} from '@thunderid/hooks';
import type {User} from '@thunderid/types';
import {getErrorMessage} from '@thunderid/utils';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Box,
  Alert,
  DataGrid,
  Avatar,
  Chip,
  Typography,
  Tabs,
  Tab,
} from '@wso2/oxygen-ui';
import {AppWindow, Bot, UserRound, UsersRound} from '@wso2/oxygen-ui-icons-react';
import {useState, useMemo, useCallback, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import useGetRoleAssignments from '../../../api/useGetRoleAssignments';
import type {BasicAgent} from '../../../internal/agent';
import useGetAgents from '../../../internal/useGetAgents';
import type {RoleAssignment} from '../../../models/role';

interface AddAssignmentDialogProps {
  open: boolean;
  roleId: string;
  onClose: () => void;
  onAdd: (assignments: RoleAssignment[]) => void;
  /** Inline error shown in the dialog when the last add attempt failed. */
  error?: string | null;
  /** Called when the tab or a selection changes, so the parent can clear a stale error. */
  onErrorDismiss?: () => void;
  /** Whether the add mutation is in flight, so the confirm button can show progress. */
  isSubmitting?: boolean;
  initialTab?: number;
}

/**
 * Dialog for searching and adding user, group, or app assignments to a role.
 */
export default function AddAssignmentDialog({
  open,
  roleId,
  onClose,
  onAdd,
  error = null,
  onErrorDismiss = undefined,
  isSubmitting = false,
  initialTab = 0,
}: AddAssignmentDialogProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();

  const [activeTab, setActiveTab] = useState(initialTab);
  const [userSelectionModel, setUserSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
    type: 'include',
    ids: new Set(),
  });
  const [groupSelectionModel, setGroupSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
    type: 'include',
    ids: new Set(),
  });
  const [appSelectionModel, setAppSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
    type: 'include',
    ids: new Set(),
  });
  const [agentSelectionModel, setAgentSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
    type: 'include',
    ids: new Set(),
  });
  const [userPaginationModel, setUserPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [groupPaginationModel, setGroupPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });
  const [appPaginationModel, setAppPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [agentPaginationModel, setAgentPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });

  const usersParams = useMemo(
    () => ({limit: userPaginationModel.pageSize, offset: userPaginationModel.page * userPaginationModel.pageSize}),
    [userPaginationModel],
  );
  const groupsParams = useMemo(
    () => ({limit: groupPaginationModel.pageSize, offset: groupPaginationModel.page * groupPaginationModel.pageSize}),
    [groupPaginationModel],
  );
  const applicationsParams = useMemo(
    () => ({limit: appPaginationModel.pageSize, offset: appPaginationModel.page * appPaginationModel.pageSize}),
    [appPaginationModel],
  );
  const agentsParams = useMemo(
    () => ({limit: agentPaginationModel.pageSize, offset: agentPaginationModel.page * agentPaginationModel.pageSize}),
    [agentPaginationModel],
  );

  const {data: usersData, isLoading: usersLoading, error: usersError} = useGetUsers(usersParams);
  const {data: groupsData, isLoading: groupsLoading, error: groupsError} = useGetGroups(groupsParams);
  const {data: applicationsData, isLoading: appsLoading, error: appsError} = useGetApplications(applicationsParams);
  const {data: agentsData, isLoading: agentsLoading, error: agentsError} = useGetAgents(agentsParams);

  const {data: initialUserAssignments} = useGetRoleAssignments({roleId, type: 'user', limit: 1, offset: 0});
  const {data: existingUserAssignments} = useGetRoleAssignments({
    roleId,
    type: 'user',
    limit: initialUserAssignments?.totalResults ?? 0,
    offset: 0,
    enabled: (initialUserAssignments?.totalResults ?? 0) > 0,
  });
  const {data: initialGroupAssignments} = useGetRoleAssignments({roleId, type: 'group', limit: 1, offset: 0});
  const {data: existingGroupAssignments} = useGetRoleAssignments({
    roleId,
    type: 'group',
    limit: initialGroupAssignments?.totalResults ?? 0,
    offset: 0,
    enabled: (initialGroupAssignments?.totalResults ?? 0) > 0,
  });
  const {data: initialAppAssignments} = useGetRoleAssignments({roleId, type: 'app', limit: 1, offset: 0});
  const {data: existingAppAssignments} = useGetRoleAssignments({
    roleId,
    type: 'app',
    limit: initialAppAssignments?.totalResults ?? 0,
    offset: 0,
    enabled: (initialAppAssignments?.totalResults ?? 0) > 0,
  });
  const {data: initialAgentAssignments} = useGetRoleAssignments({roleId, type: 'agent', limit: 1, offset: 0});
  const {data: existingAgentAssignments} = useGetRoleAssignments({
    roleId,
    type: 'agent',
    limit: initialAgentAssignments?.totalResults ?? 0,
    offset: 0,
    enabled: (initialAgentAssignments?.totalResults ?? 0) > 0,
  });

  const assignedUserIds = useMemo(
    () => new Set(existingUserAssignments?.assignments.map((a) => a.id) ?? []),
    [existingUserAssignments],
  );
  const assignedGroupIds = useMemo(
    () => new Set(existingGroupAssignments?.assignments.map((a) => a.id) ?? []),
    [existingGroupAssignments],
  );
  const assignedAppIds = useMemo(
    () => new Set(existingAppAssignments?.assignments.map((a) => a.id) ?? []),
    [existingAppAssignments],
  );
  const assignedAgentIds = useMemo(
    () => new Set(existingAgentAssignments?.assignments.map((a) => a.id) ?? []),
    [existingAgentAssignments],
  );

  const filteredUsers = useMemo(
    () => (usersData?.users ?? []).filter((u) => !assignedUserIds.has(u.id)),
    [usersData, assignedUserIds],
  );
  const filteredGroups = useMemo(
    () => (groupsData?.groups ?? []).filter((g) => !assignedGroupIds.has(g.id)),
    [groupsData, assignedGroupIds],
  );
  const filteredApplications = useMemo(
    () => (applicationsData?.applications ?? []).filter((app) => !assignedAppIds.has(app.id)),
    [applicationsData, assignedAppIds],
  );
  const filteredAgents = useMemo(
    () => (agentsData?.agents ?? []).filter((a) => !assignedAgentIds.has(a.id)),
    [agentsData, assignedAgentIds],
  );

  const userColumns: DataGrid.GridColDef<User>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar
              sx={{
                p: 0.5,
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                width: 30,
                height: 30,
                fontSize: '0.875rem',
              }}
            >
              <UserRound size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'display',
        headerName: t('roles:assignments.dialog.columns.displayName'),
        flex: 1,
        minWidth: 200,
        renderCell: (params): JSX.Element => {
          const row = params.row;
          return (
            <Box
              sx={{
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'center',
                height: '100%',
                overflow: 'hidden',
              }}
            >
              <Typography variant="body2" noWrap>
                {row.display ?? row.id}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                noWrap
                sx={{fontFamily: 'monospace', fontSize: '0.7rem'}}
              >
                {row.id}
              </Typography>
            </Box>
          );
        },
      },
      {
        field: 'type',
        headerName: t('roles:assignments.dialog.columns.userType'),
        width: 150,
        renderCell: (params): JSX.Element => (
          <Chip label={params.row.type} size="small" variant="outlined" sx={{textTransform: 'capitalize'}} />
        ),
      },
    ],
    [t],
  );

  const groupColumns: DataGrid.GridColDef<GroupBasic>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar
              sx={{
                p: 0.5,
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                width: 30,
                height: 30,
                fontSize: '0.875rem',
              }}
            >
              <UsersRound size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('roles:assignments.dialog.columns.name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'description',
        headerName: t('roles:assignments.dialog.columns.description'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value, row): string => row.description ?? '-',
      },
    ],
    [t],
  );
  const agentColumns: DataGrid.GridColDef<BasicAgent>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar
              sx={{
                p: 0.5,
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                width: 30,
                height: 30,
                fontSize: '0.875rem',
              }}
            >
              <Bot size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('roles:assignments.dialog.columns.name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'description',
        headerName: t('roles:assignments.dialog.columns.description'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value, row): string => row.description ?? '-',
      },
    ],
    [t],
  );

  const appColumns: DataGrid.GridColDef<BasicApplication>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar
              sx={{
                p: 0.5,
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                width: 30,
                height: 30,
                fontSize: '0.875rem',
              }}
            >
              <AppWindow size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('roles:assignments.dialog.columns.name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'description',
        headerName: t('roles:assignments.dialog.columns.description'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value, row): string => row.description ?? '-',
      },
    ],
    [t],
  );

  const handleAdd = useCallback(() => {
    const userAssignments: RoleAssignment[] = [...userSelectionModel.ids].map((id) => ({
      id: String(id),
      type: 'user' as const,
    }));
    const groupAssignments: RoleAssignment[] = [...groupSelectionModel.ids].map((id) => ({
      id: String(id),
      type: 'group' as const,
    }));
    const appAssignments: RoleAssignment[] = [...appSelectionModel.ids].map((id) => ({
      id: String(id),
      type: 'app' as const,
    }));
    const agentAssignments: RoleAssignment[] = [...agentSelectionModel.ids].map((id) => ({
      id: String(id),
      type: 'agent' as const,
    }));
    // Selections are deliberately left as-is here: the dialog unmounts on a successful add (the
    // parent closes it), and on failure the user needs their selection intact to retry.
    onAdd([...userAssignments, ...groupAssignments, ...appAssignments, ...agentAssignments]);
  }, [userSelectionModel, groupSelectionModel, appSelectionModel, agentSelectionModel, onAdd]);

  const handleClose = (): void => {
    // Also reached via Escape and backdrop clicks, so guard the in-flight case here rather than
    // only disabling Cancel: closing mid-request would discard the selection needed to retry.
    if (isSubmitting) return;

    setUserSelectionModel({type: 'include', ids: new Set()});
    setGroupSelectionModel({type: 'include', ids: new Set()});
    setAppSelectionModel({type: 'include', ids: new Set()});
    setAgentSelectionModel({type: 'include', ids: new Set()});
    onClose();
  };

  const totalSelected =
    userSelectionModel.ids.size +
    groupSelectionModel.ids.size +
    appSelectionModel.ids.size +
    agentSelectionModel.ids.size;

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="md" fullWidth>
      <DialogTitle>{t('roles:assignments.dialog.title')}</DialogTitle>
      <DialogContent>
        <Tabs
          value={activeTab}
          onChange={(_e: SyntheticEvent, v: number) => {
            setActiveTab(v);
            onErrorDismiss?.();
          }}
          sx={{mb: 2}}
        >
          <Tab label={t('roles:assignments.dialog.tabs.users')} />
          <Tab label={t('roles:assignments.dialog.tabs.groups')} />
          <Tab label={t('roles:assignments.dialog.tabs.apps')} />
          <Tab label={t('roles:assignments.dialog.tabs.agents', 'Agents')} />
        </Tabs>

        {activeTab === 0 && (
          <>
            {usersError && !usersLoading && (
              <Alert severity="error" sx={{mb: 2}}>
                {getErrorMessage(
                  usersError,
                  t,
                  'roles:assignments.dialog.fetchError',
                  'Failed to load data. Please try again.',
                )}
              </Alert>
            )}
            <Box sx={{height: 400, width: '100%'}}>
              <DataGrid.DataGrid
                rows={filteredUsers}
                columns={userColumns}
                loading={usersLoading}
                getRowId={(row): string => row.id}
                checkboxSelection
                rowSelectionModel={userSelectionModel}
                onRowSelectionModelChange={(newSelection) => {
                  setUserSelectionModel(newSelection);
                  onErrorDismiss?.();
                }}
                paginationMode="server"
                rowCount={usersData?.totalResults ?? 0}
                paginationModel={userPaginationModel}
                onPaginationModelChange={setUserPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}

        {activeTab === 1 && (
          <>
            {groupsError && !groupsLoading && (
              <Alert severity="error" sx={{mb: 2}}>
                {getErrorMessage(
                  groupsError,
                  t,
                  'roles:assignments.dialog.fetchError',
                  'Failed to load data. Please try again.',
                )}
              </Alert>
            )}
            <Box sx={{height: 400, width: '100%'}}>
              <DataGrid.DataGrid
                rows={filteredGroups}
                columns={groupColumns}
                loading={groupsLoading}
                getRowId={(row): string => row.id}
                checkboxSelection
                rowSelectionModel={groupSelectionModel}
                onRowSelectionModelChange={(newSelection) => {
                  setGroupSelectionModel(newSelection);
                  onErrorDismiss?.();
                }}
                paginationMode="server"
                rowCount={groupsData?.totalResults ?? 0}
                paginationModel={groupPaginationModel}
                onPaginationModelChange={setGroupPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}

        {activeTab === 2 && (
          <>
            {appsError && !appsLoading && (
              <Alert severity="error" sx={{mb: 2}}>
                {getErrorMessage(
                  appsError,
                  t,
                  'roles:assignments.dialog.fetchError',
                  'Failed to load data. Please try again.',
                )}
              </Alert>
            )}
            <Box sx={{height: 400, width: '100%'}}>
              <DataGrid.DataGrid
                rows={filteredApplications}
                columns={appColumns}
                loading={appsLoading}
                getRowId={(row): string => row.id}
                checkboxSelection
                rowSelectionModel={appSelectionModel}
                onRowSelectionModelChange={(newSelection) => {
                  setAppSelectionModel(newSelection);
                  onErrorDismiss?.();
                }}
                paginationMode="server"
                rowCount={applicationsData?.totalResults ?? 0}
                paginationModel={appPaginationModel}
                onPaginationModelChange={setAppPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}

        {activeTab === 3 && (
          <>
            {agentsError && !agentsLoading && (
              <Alert severity="error" sx={{mb: 2}}>
                {getErrorMessage(
                  agentsError,
                  t,
                  'roles:assignments.dialog.fetchError',
                  'Failed to load data. Please try again.',
                )}
              </Alert>
            )}
            <Box sx={{height: 400, width: '100%'}}>
              <DataGrid.DataGrid
                rows={filteredAgents}
                columns={agentColumns}
                loading={agentsLoading}
                getRowId={(row): string => row.id}
                checkboxSelection
                rowSelectionModel={agentSelectionModel}
                onRowSelectionModelChange={(newSelection) => {
                  setAgentSelectionModel(newSelection);
                  onErrorDismiss?.();
                }}
                paginationMode="server"
                rowCount={agentsData?.totalResults ?? 0}
                paginationModel={agentPaginationModel}
                onPaginationModelChange={setAgentPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}
      </DialogContent>
      {error && (
        <Box sx={{px: 3, pt: 2}}>
          <Alert severity="error">{error}</Alert>
        </Box>
      )}
      <DialogActions>
        <Button onClick={handleClose} disabled={isSubmitting}>
          {t('common:actions.cancel')}
        </Button>
        <Button variant="contained" onClick={handleAdd} disabled={totalSelected === 0 || isSubmitting}>
          {t('roles:assignments.dialog.add')} {totalSelected > 0 ? `(${totalSelected})` : ''}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
