// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, getInitials} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import type {User} from '@thunderid/types';
import {Box, DataGrid, Avatar} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetOrganizationUnitUsers from '../../../api/useGetOrganizationUnitUsers';

/**
 * Props for the {@link ManageUsersSection} component.
 */
interface ManageUsersSectionProps {
  /**
   * The ID of the organization unit
   */
  organizationUnitId: string;
}

/**
 * Section component for managing users belonging to an organization unit.
 *
 * Displays a DataGrid of users with:
 * - Avatar with initials
 * - Display Name (falls back to User ID)
 * - User ID
 * - User Type
 *
 * @param props - Component props
 * @returns Manage users section within a SettingsCard
 */
export default function ManageUsersSection({organizationUnitId}: ManageUsersSectionProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: usersData, isLoading, error, refetch} = useGetOrganizationUnitUsers(organizationUnitId);

  const columns: DataGrid.GridColDef<User>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<User>): JSX.Element => {
          const displayVal = params.row.display ?? params.row.id;

          return (
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
                  color: 'primary.contrastText',
                  fontSize: '0.875rem',
                }}
              >
                {getInitials(displayVal)}
              </Avatar>
            </Box>
          );
        },
      },
      {
        field: 'display',
        headerName: t('organizationUnits:edit.users.sections.manage.listing.columns.name'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value: unknown, row: User) => row.display ?? row.id,
      },
      {
        field: 'id',
        headerName: t('organizationUnits:edit.users.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
      },
      {
        field: 'type',
        headerName: t('organizationUnits:edit.users.sections.manage.listing.columns.type'),
        flex: 0.6,
        minWidth: 120,
      },
    ],
    [t],
  );

  if (error) {
    return (
      <SettingsCard
        title={t('organizationUnits:edit.users.sections.manage.title', 'Users')}
        description={t(
          'organizationUnits:edit.users.sections.manage.description',
          'View users belonging to this organization unit',
        )}
      >
        <QueryErrorNotice
          error={error}
          t={(key, options) => t(key.includes(':') ? key : `organizationUnits:${key}`, options)}
          variant="inline"
          onRetry={() => void refetch()}
          fallbackKey="organizationUnits:edit.users.sections.manage.error"
          fallbackDefaultValue="Failed to load users"
        />
      </SettingsCard>
    );
  }

  return (
    <SettingsCard
      title={t('organizationUnits:edit.users.sections.manage.title', 'Users')}
      description={t(
        'organizationUnits:edit.users.sections.manage.description',
        'View users belonging to this organization unit',
      )}
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
          rows={usersData?.users ?? []}
          columns={columns}
          loading={isLoading}
          getRowId={(row): string => row.id}
          initialState={{
            pagination: {
              paginationModel: {pageSize: 10},
            },
          }}
          pageSizeOptions={[5, 10, 25]}
          disableRowSelectionOnClick
          localeText={dataGridLocaleText}
        />
      </Box>
    </SettingsCard>
  );
}
