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

import {SettingsCard, getInitials} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {Box, Avatar, DataGrid, IconButton} from '@wso2/oxygen-ui';
import {AppWindow, Bot, Trash2, User, Users} from '@wso2/oxygen-ui-icons-react';
import {useState, useMemo, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import useGetGroupMembers from '../../../api/useGetGroupMembers';
import type {Member} from '../../../models/group';

interface ManageMembersSectionProps {
  groupId: string;
  onRemoveMember: (member: Member) => void;
  headerAction?: ReactNode;
  isReadOnly?: boolean;
}

/**
 * Section component for displaying and managing group members.
 */
export default function ManageMembersSection({
  groupId,
  onRemoveMember,
  headerAction = undefined,
  isReadOnly = false,
}: ManageMembersSectionProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});

  const membersParams = useMemo(
    () => ({
      limit: paginationModel.pageSize,
      offset: paginationModel.page * paginationModel.pageSize,
    }),
    [paginationModel],
  );
  const {data: membersData, isLoading} = useGetGroupMembers(groupId, membersParams);

  const baseColumns: DataGrid.GridColDef<Member>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<Member>): JSX.Element => (
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
                width: 30,
                height: 30,
                bgcolor: 'primary.main',
                fontSize: '0.875rem',
              }}
            >
              {params.row.type === 'user' && <User size={14} />}
              {params.row.type === 'group' && <Users size={14} />}
              {params.row.type === 'app' && <AppWindow size={14} />}
              {params.row.type === 'agent' && <Bot size={14} />}
              {!['user', 'group', 'app', 'agent'].includes(params.row.type) &&
                getInitials(params.row.display ?? params.row.id)}
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'display',
        headerName: t('groups:edit.members.sections.manage.listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value: unknown, row: Member) => row.display ?? row.id,
      },
      {
        field: 'id',
        headerName: t('groups:edit.members.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
      },
      {
        field: 'type',
        headerName: t('groups:edit.members.sections.manage.listing.columns.type'),
        flex: 0.6,
        minWidth: 120,
      },
      {
        field: 'actions',
        headerName: '',
        width: 60,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<Member>): JSX.Element => (
          <IconButton
            size="small"
            color="error"
            aria-label={t('common:actions.remove')}
            onClick={(e) => {
              e.stopPropagation();
              onRemoveMember(params.row);
            }}
          >
            <Trash2 size={14} />
          </IconButton>
        ),
      },
    ],
    [t, onRemoveMember],
  );

  const columns = useMemo(
    () => (isReadOnly ? baseColumns.filter((col) => col.field !== 'actions') : baseColumns),
    [isReadOnly, baseColumns],
  );

  return (
    <SettingsCard
      title={t('groups:edit.members.sections.manage.title')}
      description={t('groups:edit.members.sections.manage.description')}
      headerAction={headerAction}
      slotProps={{
        content: {
          sx: {
            p: 0,
          },
        },
      }}
    >
      <Box sx={{height: 400, width: '100%'}}>
        <DataGrid.DataGrid
          rows={membersData?.members ?? []}
          columns={columns}
          loading={isLoading}
          getRowId={(row): string => row.id}
          paginationMode="server"
          rowCount={membersData?.totalResults ?? 0}
          paginationModel={paginationModel}
          onPaginationModelChange={setPaginationModel}
          pageSizeOptions={[5, 10, 25]}
          disableRowSelectionOnClick
          localeText={dataGridLocaleText}
        />
      </Box>
    </SettingsCard>
  );
}
