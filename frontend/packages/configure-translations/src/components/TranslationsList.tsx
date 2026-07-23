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

import {ResourceAvatar} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {getDisplayNameForCode, toFlagEmoji, useGetLanguages} from '@thunderid/i18n';
import {useLogger} from '@thunderid/logger/react';
import {Chip, DataGrid, IconButton, ListingTable, Tooltip, useTheme} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import TranslationConstants from '../constants/translation-constants';
import TranslationDeleteDialog from '@/components/TranslationDeleteDialog';
import useTranslationRoutes from '@/hooks/useTranslationRoutes';

export default function TranslationsList(): JSX.Element {
  const theme = useTheme();
  const {t} = useTranslation('translations');
  const navigate = useNavigate();
  const logger = useLogger('TranslationsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const routes = useTranslationRoutes();

  const [selectedLanguage, setSelectedLanguage] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const {data, isLoading} = useGetLanguages();

  const handleEditClick = useCallback(
    (language: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(language));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to translation editor', {error: _error, language});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteClick = useCallback((language: string): void => {
    setSelectedLanguage(language);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedLanguage(null);
  };

  const rows = useMemo(() => (data?.languages ?? []).map((code) => ({id: code, code})), [data?.languages]);

  const columns: DataGrid.GridColDef<{id: string; code: string}>[] = useMemo(
    () => [
      {
        field: 'code',
        headerName: t('listing.columns.language'),
        flex: 1,
        minWidth: 240,
        renderCell: (params: DataGrid.GridRenderCellParams<{id: string; code: string}>): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={
              <ResourceAvatar
                variant="rounded"
                value={toFlagEmoji(params.row.code)}
                size={30}
                fallback={TranslationConstants.DEFAULT_AVATAR}
              />
            }
            primary={getDisplayNameForCode(params.row.code)}
            secondary={
              <Chip
                label={params.row.code}
                size="small"
                variant="outlined"
                sx={{fontSize: '0.7rem', fontFamily: 'monospace', height: 18}}
              />
            }
          />
        ),
      },
      {
        field: 'actions',
        headerName: t('listing.columns.actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<{id: string; code: string}>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit')}>
              <IconButton
                size="small"
                aria-label={t('common:actions.edit')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleEditClick(params.row.code);
                }}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete')}>
              <IconButton
                size="small"
                color="error"
                aria-label={t('common:actions.delete')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleDeleteClick(params.row.code);
                }}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </ListingTable.RowActions>
        ),
      },
    ],
    [handleDeleteClick, handleEditClick, t, theme],
  );

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={rows}
            columns={columns}
            getRowId={(row): string => (row as {id: string; code: string}).id}
            onRowClick={(params) => {
              handleEditClick((params.row as {id: string; code: string}).code);
            }}
            initialState={{
              pagination: {
                paginationModel: {pageSize: 10},
              },
            }}
            pageSizeOptions={[5, 10, 25]}
            rowHeight={56}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            autoHeight
            sx={{
              '& .MuiDataGrid-row': {
                cursor: 'pointer',
              },
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <TranslationDeleteDialog open={deleteDialogOpen} language={selectedLanguage} onClose={handleDeleteDialogClose} />
    </>
  );
}
