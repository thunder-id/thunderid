// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import type {BasicApplication} from '@thunderid/configure-applications';
import {useDataGridLocaleText} from '@thunderid/hooks';
import type {User} from '@thunderid/types';
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
import useGetAllCandidates from '../../../api/useGetAllCandidates';
import useGetAllGroupMembers from '../../../api/useGetAllGroupMembers';
import type {BasicAgent} from '../../../internal/agent';
import type {GroupBasic, Member} from '../../../models/group';

interface AddMemberDialogProps {
  open: boolean;
  onClose: () => void;
  onAdd: (members: Member[]) => void;
  /** Group being edited, excluded from the groups tab so it cannot be made a member of itself. */
  excludeGroupId?: string;
  /** Inline error shown in the dialog when the last add attempt failed. */
  error?: string | null;
  /** Called when the tab or a selection changes, so the parent can clear a stale error. */
  onErrorDismiss?: () => void;
  /** Whether the add mutation is in flight, so the confirm button can show progress. */
  isSubmitting?: boolean;
  /** Group ID used to hide members that are already assigned. */
  groupId?: string;
}

/**
 * Dialog for searching and adding user, app, agent, or group members to a group.
 */
export default function AddMemberDialog({
  open,
  onClose,
  onAdd,
  excludeGroupId = undefined,
  error = null,
  onErrorDismiss = undefined,
  isSubmitting = false,
  groupId = undefined,
}: AddMemberDialogProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();

  const [activeTab, setActiveTab] = useState(0);
  const [userSelectionModel, setUserSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
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
  const [groupSelectionModel, setGroupSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
    type: 'include',
    ids: new Set(),
  });
  const [userPaginationModel, setUserPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [appPaginationModel, setAppPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [agentPaginationModel, setAgentPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });
  const [groupPaginationModel, setGroupPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });

  const {
    data: users = [],
    isLoading: usersLoading,
    error: usersError,
    refetch: refetchUsers,
  } = useGetAllCandidates<User>({path: '/users', itemsKey: 'users', enabled: activeTab === 0});
  const {
    data: applications = [],
    isLoading: appsLoading,
    error: appsError,
    refetch: refetchApplications,
  } = useGetAllCandidates<BasicApplication>({
    path: '/applications',
    itemsKey: 'applications',
    enabled: activeTab === 1,
  });
  const {
    data: agents = [],
    isLoading: agentsLoading,
    error: agentsError,
    refetch: refetchAgents,
  } = useGetAllCandidates<BasicAgent>({path: '/agents', itemsKey: 'agents', enabled: activeTab === 2});
  const {
    data: allGroups = [],
    isLoading: groupsLoading,
    error: groupsError,
    refetch: refetchGroups,
  } = useGetAllCandidates<GroupBasic>({path: '/groups', itemsKey: 'groups', enabled: activeTab === 3});
  const {
    data: membersData,
    isLoading: membersLoading,
    error: membersError,
    refetch: refetchMembers,
  } = useGetAllGroupMembers(groupId);
  const membershipUnavailable = Boolean(groupId) && (membersLoading || Boolean(membersError));

  const existingMemberKeys = useMemo(
    () => new Set((membersData?.members ?? []).map((member) => `${member.type}:${member.id}`)),
    [membersData],
  );
  const filteredUsers = useMemo(
    () => users.filter((user) => !existingMemberKeys.has(`user:${user.id}`)),
    [users, existingMemberKeys],
  );
  const filteredApplications = useMemo(
    () => applications.filter((application) => !existingMemberKeys.has(`app:${application.id}`)),
    [applications, existingMemberKeys],
  );
  const filteredAgents = useMemo(
    () => agents.filter((agent) => !existingMemberKeys.has(`agent:${agent.id}`)),
    [agents, existingMemberKeys],
  );
  const groups: GroupBasic[] = useMemo(
    () => allGroups.filter((group) => group.id !== excludeGroupId && !existingMemberKeys.has(`group:${group.id}`)),
    [allGroups, excludeGroupId, existingMemberKeys],
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
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
            }}
          >
            <Avatar
              sx={{
                p: 0.5,
                width: 30,
                height: 30,
                fontSize: '0.875rem',
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
              }}
            >
              <UserRound size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'display',
        headerName: t('groups:addMember.columns.displayName'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<User>): JSX.Element => (
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
              {params.row.display ?? params.row.id}
            </Typography>
            <Typography
              variant="caption"
              color="text.secondary"
              noWrap
              sx={{fontFamily: 'monospace', fontSize: '0.7rem'}}
            >
              {params.row.id}
            </Typography>
          </Box>
        ),
      },
      {
        field: 'type',
        headerName: t('groups:addMember.columns.userType'),
        width: 150,
        renderCell: (params: DataGrid.GridRenderCellParams<User>): JSX.Element => (
          <Chip label={params.row.type} size="small" variant="outlined" sx={{textTransform: 'capitalize'}} />
        ),
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
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
            }}
          >
            <Avatar
              sx={{
                p: 0.5,
                width: 30,
                height: 30,
                fontSize: '0.875rem',
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
              }}
            >
              <Bot size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('groups:addMember.columns.displayName'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'id',
        headerName: t('groups:edit.members.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
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
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
            }}
          >
            <Avatar
              sx={{
                p: 0.5,
                width: 30,
                height: 30,
                fontSize: '0.875rem',
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
              }}
            >
              <UsersRound size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('groups:addMember.columns.displayName'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'id',
        headerName: t('groups:edit.members.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
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
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
            }}
          >
            <Avatar
              sx={{
                p: 0.5,
                width: 30,
                height: 30,
                fontSize: '0.875rem',
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
              }}
            >
              <AppWindow size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('groups:addMember.columns.displayName'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'id',
        headerName: t('groups:edit.members.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
      },
    ],
    [t],
  );

  const handleAdd = useCallback(() => {
    const newMembers: Member[] = [
      ...[...userSelectionModel.ids].map((id) => ({id: String(id), type: 'user' as const})),
      ...[...appSelectionModel.ids].map((id) => ({id: String(id), type: 'app' as const})),
      ...[...agentSelectionModel.ids].map((id) => ({id: String(id), type: 'agent' as const})),
      ...[...groupSelectionModel.ids].map((id) => ({id: String(id), type: 'group' as const})),
    ];
    // Selections are deliberately left as-is here: the dialog unmounts on a successful add (the
    // parent closes it), and on failure the user needs their selection intact to retry.
    onAdd(newMembers);
  }, [userSelectionModel, appSelectionModel, agentSelectionModel, groupSelectionModel, onAdd]);

  const handleClose = (): void => {
    // Also reached via Escape and backdrop clicks, so guard the in-flight case here rather than
    // only disabling Cancel: closing mid-request would discard the selection needed to retry.
    if (isSubmitting) return;

    setUserSelectionModel({type: 'include', ids: new Set()});
    setAppSelectionModel({type: 'include', ids: new Set()});
    setAgentSelectionModel({type: 'include', ids: new Set()});
    setGroupSelectionModel({type: 'include', ids: new Set()});
    onClose();
  };

  const totalSelected =
    userSelectionModel.ids.size +
    appSelectionModel.ids.size +
    agentSelectionModel.ids.size +
    groupSelectionModel.ids.size;

  const handleTabChange = (_event: SyntheticEvent, tab: number): void => {
    setActiveTab(tab);
    onErrorDismiss?.();
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="md" fullWidth>
      <DialogTitle>{t('groups:addMember.title')}</DialogTitle>
      <DialogContent>
        <Tabs value={activeTab} onChange={handleTabChange} sx={{mb: 2}}>
          <Tab icon={<UserRound size={16} />} iconPosition="start" label={t('groups:addMember.tabs.users')} />
          <Tab icon={<AppWindow size={16} />} iconPosition="start" label={t('groups:addMember.tabs.apps')} />
          <Tab icon={<Bot size={16} />} iconPosition="start" label={t('groups:addMember.tabs.agents', 'Agents')} />
          <Tab
            icon={<UsersRound size={16} />}
            iconPosition="start"
            label={t('groups:addMember.tabs.groups', 'Groups')}
          />
        </Tabs>

        {membersError && (
          <QueryErrorNotice
            error={membersError}
            t={t}
            variant="inline"
            fallbackKey="groups:addMember.membersFetchError"
            fallbackDefaultValue="Failed to load current group members. Please try again."
            onRetry={() => void refetchMembers()}
          />
        )}

        {activeTab === 0 && !membersLoading && !membersError && (
          <>
            {usersError && (
              <QueryErrorNotice
                error={usersError}
                t={t}
                variant="inline"
                fallbackKey="groups:addMember.fetchError"
                fallbackDefaultValue="Failed to load users. Please try again."
                onRetry={() => void refetchUsers()}
              />
            )}
            {!usersError && filteredUsers.length === 0 && !usersLoading && !membersLoading && (
              <Alert severity="info" sx={{mb: 2}}>
                {t('groups:addMember.noResults', 'No users found')}
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
                paginationModel={userPaginationModel}
                onPaginationModelChange={setUserPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}

        {activeTab === 1 && !membersLoading && !membersError && (
          <>
            {appsError && (
              <QueryErrorNotice
                error={appsError}
                t={t}
                variant="inline"
                fallbackKey="groups:addMember.fetchAppsError"
                fallbackDefaultValue="Failed to load apps. Please try again."
                onRetry={() => void refetchApplications()}
              />
            )}
            {!appsError && filteredApplications.length === 0 && !appsLoading && !membersLoading && (
              <Alert severity="info" sx={{mb: 2}}>
                {t('groups:addMember.noResultsApps', 'No apps found')}
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
                paginationModel={appPaginationModel}
                onPaginationModelChange={setAppPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}

        {activeTab === 2 && !membersLoading && !membersError && (
          <>
            {agentsError && (
              <QueryErrorNotice
                error={agentsError}
                t={t}
                variant="inline"
                fallbackKey="groups:addMember.fetchAgentsError"
                fallbackDefaultValue="Failed to load agents. Please try again."
                onRetry={() => void refetchAgents()}
              />
            )}
            {!agentsError && filteredAgents.length === 0 && !agentsLoading && !membersLoading && (
              <Alert severity="info" sx={{mb: 2}}>
                {t('groups:addMember.noResultsAgents', 'No agents found')}
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
                paginationModel={agentPaginationModel}
                onPaginationModelChange={setAgentPaginationModel}
                pageSizeOptions={[5, 10]}
                disableRowSelectionOnClick
                localeText={dataGridLocaleText}
              />
            </Box>
          </>
        )}

        {activeTab === 3 && !membersLoading && !membersError && (
          <>
            {groupsError && (
              <QueryErrorNotice
                error={groupsError}
                t={t}
                variant="inline"
                fallbackKey="groups:addMember.fetchGroupsError"
                fallbackDefaultValue="Failed to load groups. Please try again."
                onRetry={() => void refetchGroups()}
              />
            )}
            {!groupsError && groups.length === 0 && !groupsLoading && !membersLoading && (
              <Alert severity="info" sx={{mb: 2}}>
                {t('groups:addMember.noResultsGroups', 'No groups found')}
              </Alert>
            )}

            <Box sx={{height: 400, width: '100%'}}>
              <DataGrid.DataGrid
                rows={groups}
                columns={groupColumns}
                loading={groupsLoading}
                getRowId={(row): string => row.id}
                checkboxSelection
                rowSelectionModel={groupSelectionModel}
                onRowSelectionModelChange={(newSelection) => {
                  setGroupSelectionModel(newSelection);
                  onErrorDismiss?.();
                }}
                paginationModel={groupPaginationModel}
                onPaginationModelChange={setGroupPaginationModel}
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
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={handleAdd}
          disabled={totalSelected === 0 || isSubmitting || membershipUnavailable}
        >
          {t('groups:addMember.add', 'Add Selected')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
