// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, getInitials} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {Box, DataGrid, Avatar} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetOrganizationUnitGroups from '../../../api/useGetOrganizationUnitGroups';
import type {Group} from '../../../models/group';

/**
 * Props for the {@link ManageGroupsSection} component.
 */
interface ManageGroupsSectionProps {
  /**
   * The ID of the organization unit
   */
  organizationUnitId: string;
}

/**
 * Section component for managing groups belonging to an organization unit.
 *
 * Displays a DataGrid of groups with:
 * - Avatar icon
 * - Group Name
 * - Group ID
 *
 * @param props - Component props
 * @returns Manage groups section within a SettingsCard
 */
export default function ManageGroupsSection({organizationUnitId}: ManageGroupsSectionProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: groupsData, isLoading, error, refetch} = useGetOrganizationUnitGroups(organizationUnitId);

  const columns: DataGrid.GridColDef<Group>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<Group>): JSX.Element => {
          const displayVal = params.row.name ?? params.row.id;

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
        field: 'name',
        headerName: t('organizationUnits:edit.groups.sections.manage.listing.columns.name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'id',
        headerName: t('organizationUnits:edit.groups.sections.manage.listing.columns.id'),
        flex: 1,
        minWidth: 250,
      },
    ],
    [t],
  );

  if (error) {
    return (
      <SettingsCard
        title={t('organizationUnits:edit.groups.sections.manage.title', 'Groups')}
        description={t(
          'organizationUnits:edit.groups.sections.manage.description',
          'View groups belonging to this organization unit',
        )}
      >
        <QueryErrorNotice
          error={error}
          t={(key, options) => t(key.includes(':') ? key : `organizationUnits:${key}`, options)}
          variant="inline"
          onRetry={() => void refetch()}
          fallbackKey="organizationUnits:edit.groups.sections.manage.error"
          fallbackDefaultValue="Failed to load groups"
        />
      </SettingsCard>
    );
  }

  return (
    <SettingsCard
      title={t('organizationUnits:edit.groups.sections.manage.title', 'Groups')}
      description={t(
        'organizationUnits:edit.groups.sections.manage.description',
        'View groups belonging to this organization unit',
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
          rows={groupsData?.groups ?? []}
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
