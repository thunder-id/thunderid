// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {DataGrid, ListingTable, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetConfigurationVersions from '../api/useGetConfigurationVersions';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {ConfigurationVersion} from '../models/gateway';
import formatTimestamp from '../utils/formatTimestamp';

/**
 * Lists the captured configuration versions, newest first.
 */
export default function ConfigurationVersionsList(): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error, refetch} = useGetConfigurationVersions();

  const columns: DataGrid.GridColDef<ConfigurationVersion>[] = useMemo(
    () => [
      {
        field: 'version',
        headerName: t('gateways:versions.columns.version', 'Version'),
        width: 120,
        renderCell: (params: DataGrid.GridRenderCellParams<ConfigurationVersion>) => (
          <Typography variant="body2" fontWeight={500}>
            {t('gateways:versions.versionNumber', 'Version {{version}}', {version: params.row.version})}
          </Typography>
        ),
      },
      {
        field: 'note',
        headerName: t('gateways:versions.columns.note', 'Note'),
        flex: 1,
        minWidth: 200,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<ConfigurationVersion>) =>
          params.row.note ? (
            <Typography variant="body2">{params.row.note}</Typography>
          ) : (
            <Typography variant="body2" color="text.disabled">
              -
            </Typography>
          ),
      },
      {
        field: 'createdAt',
        headerName: t('gateways:versions.columns.createdAt', 'Captured'),
        width: 220,
        renderCell: (params: DataGrid.GridRenderCellParams<ConfigurationVersion>) => (
          <Typography variant="body2" color="text.secondary">
            {formatTimestamp(params.row.createdAt)}
          </Typography>
        ),
      },
    ],
    [t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="block"
        title={t('gateways:versions.error', 'Failed to load configuration versions')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
      <ListingTable.Container disablePaper>
        <ListingTable.DataGrid
          rows={data ?? []}
          columns={columns}
          getRowId={(row) => (row as ConfigurationVersion).version}
          disableRowSelectionOnClick
          disableColumnFilter
          localeText={{
            ...dataGridLocaleText,
            noRowsLabel: t('gateways:versions.empty', 'No configuration has been captured yet.'),
          }}
          initialState={{pagination: {paginationModel: {pageSize: 10}}}}
          pageSizeOptions={[5, 10, 25]}
          autoHeight
        />
      </ListingTable.Container>
    </ListingTable.Provider>
  );
}
