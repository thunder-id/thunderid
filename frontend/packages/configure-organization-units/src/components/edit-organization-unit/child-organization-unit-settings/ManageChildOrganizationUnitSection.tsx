/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import {useLogger} from '@thunderid/logger/react';
import {Box, DataGrid, Avatar, useTheme} from '@wso2/oxygen-ui';
import {Building} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useGetChildOrganizationUnits from '../../../api/useGetChildOrganizationUnits';
import useOrganizationUnitRoutes from '../../../hooks/useOrganizationUnitRoutes';
import type {OUNavigationState} from '../../../models/navigation';
import type {OrganizationUnit} from '../../../models/organization-unit';

/**
 * Props for the {@link ManageChildOrganizationUnitSection} component.
 */
interface ManageChildOrganizationUnitSectionProps {
  /**
   * The ID of the parent organization unit
   */
  organizationUnitId: string;
  /**
   * The name of the parent organization unit (for back navigation)
   */
  organizationUnitName: string;
}

/**
 * Section component for managing child organization units.
 *
 * Displays a DataGrid of child organization units with:
 * - Avatar icon
 * - Name
 * - Handle
 * - Description
 *
 * Clicking a row navigates to that child OU's detail page.
 *
 * @param props - Component props
 * @returns Manage child OUs section within a SettingsCard
 */
export default function ManageChildOrganizationUnitSection({
  organizationUnitId,
  organizationUnitName,
}: ManageChildOrganizationUnitSectionProps): JSX.Element {
  const navigate = useNavigate();
  const routes = useOrganizationUnitRoutes();
  const {t} = useTranslation();
  const theme = useTheme();
  const logger = useLogger('ManageChildOrganizationUnitSection');
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: childOUsData, isLoading} = useGetChildOrganizationUnits(organizationUnitId);

  const columns: DataGrid.GridColDef<OrganizationUnit>[] = useMemo(
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
                backgroundColor: theme.vars?.palette.grey[500],
                width: 30,
                height: 30,
                fontSize: '0.875rem',
                ...theme.applyStyles('dark', {
                  backgroundColor: theme.vars?.palette.grey[900],
                }),
              }}
            >
              <Building size={14} />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('organizationUnits:listing.columns.name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'handle',
        headerName: t('organizationUnits:listing.columns.handle'),
        flex: 1,
        minWidth: 150,
      },
      {
        field: 'description',
        headerName: t('organizationUnits:listing.columns.description'),
        flex: 2,
        minWidth: 250,
        valueGetter: (_value, row): string => row.description ?? '-',
      },
    ],
    [t, theme],
  );

  return (
    <SettingsCard
      title={t('organizationUnits:edit.childOUs.sections.manage.title')}
      description={t('organizationUnits:edit.childOUs.sections.manage.description')}
      slotProps={{
        content: {
          sx: {
            p: 0,
          },
        },
      }}
    >
      <Box sx={{maxHeight: 400, width: '100%'}}>
        <DataGrid.DataGrid
          rows={childOUsData?.organizationUnits ?? []}
          columns={columns}
          loading={isLoading}
          getRowId={(row): string => row.id}
          onRowClick={(params) => {
            const ou = params.row as OrganizationUnit;
            const navigationState: OUNavigationState = {
              fromOU: {
                id: organizationUnitId,
                name: organizationUnitName,
              },
            };
            (async (): Promise<void> => {
              await navigate(routes.detail(ou.id), {state: navigationState});
            })().catch((_error: unknown) => {
              logger.error('Failed to navigate to child organization unit', {error: _error, ouId: ou.id});
            });
          }}
          initialState={{
            pagination: {
              paginationModel: {pageSize: 10},
            },
          }}
          pageSizeOptions={[5, 10, 25]}
          disableRowSelectionOnClick
          localeText={dataGridLocaleText}
          sx={{
            '& .MuiDataGrid-row': {
              cursor: 'pointer',
            },
          }}
        />
      </Box>
    </SettingsCard>
  );
}
