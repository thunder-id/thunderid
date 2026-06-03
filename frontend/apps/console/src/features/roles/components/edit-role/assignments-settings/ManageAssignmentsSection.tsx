/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {SettingsCard} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {Box, Avatar, DataGrid, IconButton, Tabs, Tab} from '@wso2/oxygen-ui';
import {AppWindow, Bot, Trash2, User, Users} from '@wso2/oxygen-ui-icons-react';
import {useState, useMemo, type JSX, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import useGetRoleAssignments from '../../../api/useGetRoleAssignments';
import type {RoleAssignment} from '../../../models/role';

interface ManageAssignmentsSectionProps {
  roleId: string;
  onRemoveAssignment: (assignment: RoleAssignment) => void;
  headerAction?: ReactNode;
  activeAssignmentTab: number;
  onAssignmentTabChange: (tab: number) => void;
  isReadOnly?: boolean;
}

/**
 * Section component for displaying and managing role assignments
 * with separate Users and Groups sub-tabs.
 */
export default function ManageAssignmentsSection({
  roleId,
  onRemoveAssignment,
  headerAction = undefined,
  activeAssignmentTab,
  onAssignmentTabChange,
  isReadOnly = false,
}: ManageAssignmentsSectionProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();

  const [userPaginationModel, setUserPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [groupPaginationModel, setGroupPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });
  const [appPaginationModel, setAppPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });
  const [agentPaginationModel, setAgentPaginationModel] = useState<DataGrid.GridPaginationModel>({
    pageSize: 10,
    page: 0,
  });

  const userAssignmentsParams = useMemo(
    () => ({
      roleId,
      limit: userPaginationModel.pageSize,
      offset: userPaginationModel.page * userPaginationModel.pageSize,
      include: 'display' as const,
      type: 'user' as const,
    }),
    [roleId, userPaginationModel],
  );

  const groupAssignmentsParams = useMemo(
    () => ({
      roleId,
      limit: groupPaginationModel.pageSize,
      offset: groupPaginationModel.page * groupPaginationModel.pageSize,
      include: 'display' as const,
      type: 'group' as const,
    }),
    [roleId, groupPaginationModel],
  );
  const appAssignmentsParams = useMemo(
    () => ({
      roleId,
      limit: appPaginationModel.pageSize,
      offset: appPaginationModel.page * appPaginationModel.pageSize,
      include: 'display' as const,
      type: 'app' as const,
    }),
    [roleId, appPaginationModel],
  );
  const agentAssignmentsParams = useMemo(
    () => ({
      roleId,
      limit: agentPaginationModel.pageSize,
      offset: agentPaginationModel.page * agentPaginationModel.pageSize,
      include: 'display' as const,
      type: 'agent' as const,
    }),
    [roleId, agentPaginationModel],
  );

  const {data: userAssignmentsData, isLoading: isUsersLoading} = useGetRoleAssignments(userAssignmentsParams);
  const {data: groupAssignmentsData, isLoading: isGroupsLoading} = useGetRoleAssignments(groupAssignmentsParams);
  const {data: appAssignmentsData, isLoading: isAppsLoading} = useGetRoleAssignments(appAssignmentsParams);
  const {data: agentAssignmentsData, isLoading: isAgentsLoading} = useGetRoleAssignments(agentAssignmentsParams);

  const baseColumns: DataGrid.GridColDef<RoleAssignment>[] = useMemo(
    () => [
      {
        field: 'display',
        headerName: t('roles:edit.assignments.sections.manage.listing.columns.name'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value: unknown, row: RoleAssignment) => row.display ?? row.id,
      },
      {
        field: 'id',
        headerName: t('roles:edit.assignments.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
      },
      {
        field: 'actions',
        headerName: '',
        width: 60,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<RoleAssignment>): JSX.Element => (
          <IconButton
            size="small"
            color="error"
            aria-label={t('common:actions.remove')}
            onClick={(e) => {
              e.stopPropagation();
              onRemoveAssignment(params.row);
            }}
          >
            <Trash2 size={14} />
          </IconButton>
        ),
      },
    ],
    [t, onRemoveAssignment],
  );

  const effectiveBaseColumns = useMemo(
    () => (isReadOnly ? baseColumns.filter((col) => col.field !== 'actions') : baseColumns),
    [isReadOnly, baseColumns],
  );

  const userColumns: DataGrid.GridColDef<RoleAssignment>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<RoleAssignment>): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar sx={{width: 30, height: 30, bgcolor: 'primary.main', fontSize: '0.875rem'}}>
              {(params.row.display ?? params.row.id).charAt(0).toUpperCase()}
            </Avatar>
          </Box>
        ),
      },
      ...effectiveBaseColumns,
    ],
    [effectiveBaseColumns],
  );

  const groupColumns: DataGrid.GridColDef<RoleAssignment>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<RoleAssignment>): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar sx={{width: 30, height: 30, bgcolor: 'primary.main', fontSize: '0.875rem'}}>
              {(params.row.display ?? params.row.id).charAt(0).toUpperCase()}
            </Avatar>
          </Box>
        ),
      },
      ...effectiveBaseColumns,
    ],
    [effectiveBaseColumns],
  );
  const appColumns: DataGrid.GridColDef<RoleAssignment>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar sx={{width: 30, height: 30, bgcolor: 'primary.main', fontSize: '0.875rem'}}>
              <AppWindow size={14} />
            </Avatar>
          </Box>
        ),
      },
      ...effectiveBaseColumns,
    ],
    [effectiveBaseColumns],
  );
  const agentColumns: DataGrid.GridColDef<RoleAssignment>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar sx={{width: 30, height: 30, bgcolor: 'primary.main', fontSize: '0.875rem'}}>
              <Bot size={14} />
            </Avatar>
          </Box>
        ),
      },
      ...effectiveBaseColumns,
    ],
    [effectiveBaseColumns],
  );

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    onAssignmentTabChange(newValue);
  };

  return (
    <SettingsCard
      title={t('roles:edit.assignments.sections.manage.title')}
      description={t('roles:edit.assignments.sections.manage.description')}
      headerAction={headerAction}
      slotProps={{content: {sx: {p: 0}}}}
    >
      <Box sx={{borderBottom: 1, borderColor: 'divider', px: 2}}>
        <Tabs value={activeAssignmentTab} onChange={handleTabChange}>
          <Tab
            icon={<User size={16} />}
            iconPosition="start"
            label={t('roles:edit.assignments.sections.manage.tabs.users')}
            sx={{textTransform: 'none'}}
          />
          <Tab
            icon={<Users size={16} />}
            iconPosition="start"
            label={t('roles:edit.assignments.sections.manage.tabs.groups')}
            sx={{textTransform: 'none'}}
          />
          <Tab
            icon={<AppWindow size={16} />}
            iconPosition="start"
            label={t('roles:edit.assignments.sections.manage.tabs.apps')}
            sx={{textTransform: 'none'}}
          />
          <Tab
            icon={<Bot size={16} />}
            iconPosition="start"
            label={t('roles:edit.assignments.sections.manage.tabs.agents', 'Agents')}
            sx={{textTransform: 'none'}}
          />
        </Tabs>
      </Box>

      {activeAssignmentTab === 0 && (
        <Box sx={{height: 400, width: '100%'}}>
          <DataGrid.DataGrid
            rows={userAssignmentsData?.assignments ?? []}
            columns={userColumns}
            loading={isUsersLoading}
            getRowId={(row): string => `user:${row.id}`}
            paginationMode="server"
            rowCount={userAssignmentsData?.totalResults ?? 0}
            paginationModel={userPaginationModel}
            onPaginationModelChange={setUserPaginationModel}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            sx={{'--oxygen-shape-borderRadius': '0px', border: 'none'}}
          />
        </Box>
      )}

      {activeAssignmentTab === 1 && (
        <Box sx={{height: 400, width: '100%'}}>
          <DataGrid.DataGrid
            rows={groupAssignmentsData?.assignments ?? []}
            columns={groupColumns}
            loading={isGroupsLoading}
            getRowId={(row): string => `group:${row.id}`}
            paginationMode="server"
            rowCount={groupAssignmentsData?.totalResults ?? 0}
            paginationModel={groupPaginationModel}
            onPaginationModelChange={setGroupPaginationModel}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            sx={{'--oxygen-shape-borderRadius': '0px', border: 'none'}}
          />
        </Box>
      )}

      {activeAssignmentTab === 2 && (
        <Box sx={{height: 400, width: '100%'}}>
          <DataGrid.DataGrid
            rows={appAssignmentsData?.assignments ?? []}
            columns={appColumns}
            loading={isAppsLoading}
            getRowId={(row): string => `app:${row.id}`}
            paginationMode="server"
            rowCount={appAssignmentsData?.totalResults ?? 0}
            paginationModel={appPaginationModel}
            onPaginationModelChange={setAppPaginationModel}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            sx={{'--oxygen-shape-borderRadius': '0px', border: 'none'}}
          />
        </Box>
      )}

      {activeAssignmentTab === 3 && (
        <Box sx={{height: 400, width: '100%'}}>
          <DataGrid.DataGrid
            rows={agentAssignmentsData?.assignments ?? []}
            columns={agentColumns}
            loading={isAgentsLoading}
            getRowId={(row): string => `agent:${row.id}`}
            paginationMode="server"
            rowCount={agentAssignmentsData?.totalResults ?? 0}
            paginationModel={agentPaginationModel}
            onPaginationModelChange={setAgentPaginationModel}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            sx={{'--oxygen-shape-borderRadius': '0px', border: 'none'}}
          />
        </Box>
      )}
    </SettingsCard>
  );
}
