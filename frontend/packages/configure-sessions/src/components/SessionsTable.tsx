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

import {useDataGridLocaleText} from '@thunderid/hooks';
import {Alert, Chip, Stack, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import useGetSessions from '../api/useGetSessions';
import type {Session, SessionListFilter} from '../models/sessions';

export interface SessionsTableProps {
  /** Exactly one of userId or appId. */
  filter: SessionListFilter;
  /** Show the user column (application view). */
  showUser?: boolean;
  /** Hide the participants column (application view — the app itself is the context). */
  hideParticipants?: boolean;
}

function formatDateTime(iso: string): string {
  if (!iso) {
    return '-';
  }
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/** Earliest of the two expiry deadlines; sessions without deadlines never expire. */
function effectiveExpiry(session: Session): string {
  const deadlines = [session.idleExpiresAt, session.absoluteExpiresAt].filter(Boolean) as string[];
  if (deadlines.length === 0) {
    return '';
  }
  return deadlines.sort()[0];
}

/**
 * Read-only table of live sessions for a user or an application.
 * "Last active" reflects activity the server observes (sign-ins and session reuse),
 * not every request an application serves.
 */
export default function SessionsTable({filter, showUser = false, hideParticipants = false}: SessionsTableProps) {
  const {t} = useTranslation();
  const [paginationModel, setPaginationModel] = useState({page: 0, pageSize: 10});
  const {data, isLoading, isError} = useGetSessions(filter, {
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
  });
  const dataGridLocaleText = useDataGridLocaleText();

  // keepPreviousData on the query keeps `data` populated across page changes, so the total stays
  // stable and server-side pagination does not reset mid-fetch.
  const rowCount = data?.totalResults ?? 0;

  const columns: DataGrid.GridColDef<Session>[] = useMemo(() => {
    const cols: DataGrid.GridColDef<Session>[] = [];

    if (showUser) {
      cols.push({
        field: 'userId',
        headerName: t('sessions:table.columns.user', 'User'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<Session>) =>
          params.row.userName ? (
            <Typography variant="body2">{params.row.userName}</Typography>
          ) : (
            <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
              {params.row.userId}
            </Typography>
          ),
      });
    }

    cols.push(
      {
        field: 'authenticatedAt',
        headerName: t('sessions:table.columns.signedIn', 'Signed in'),
        width: 180,
        valueGetter: (_value, row): string => formatDateTime(row.authenticatedAt),
      },
      {
        field: 'lastActiveAt',
        headerName: t('sessions:table.columns.lastActive', 'Last active'),
        width: 180,
        valueGetter: (_value, row): string => formatDateTime(row.lastActiveAt),
      },
      {
        field: 'expiresAt',
        headerName: t('sessions:table.columns.expires', 'Expires'),
        width: 180,
        valueGetter: (_value, row): string => {
          const expiry = effectiveExpiry(row);
          return expiry ? formatDateTime(expiry) : t('sessions:table.values.never', 'Never');
        },
      },
    );

    if (!hideParticipants) {
      cols.push({
        field: 'participants',
        headerName: t('sessions:table.columns.applications', 'Applications'),
        flex: 1,
        minWidth: 220,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<Session>) => (
          <Stack direction="row" spacing={1} sx={{flexWrap: 'wrap', alignItems: 'center', height: '100%'}}>
            {params.row.participants.map((participant) => (
              <Chip key={participant.appId} label={participant.appName ?? participant.appId} size="small" />
            ))}
          </Stack>
        ),
      });
    }

    return cols;
  }, [hideParticipants, showUser, t]);

  if (isError) {
    return (
      <Alert severity="error">
        {t('sessions:table.errors.loadFailed', 'Unable to load sessions. You may not have permission to view them.')}
      </Alert>
    );
  }

  return (
    <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
      <ListingTable.Container disablePaper>
        <ListingTable.DataGrid
          rows={data?.sessions ?? []}
          columns={columns}
          getRowId={(row) => (row as Session).id}
          paginationMode="server"
          rowCount={rowCount}
          paginationModel={paginationModel}
          onPaginationModelChange={setPaginationModel}
          pageSizeOptions={[5, 10, 25, 50]}
          disableRowSelectionOnClick
          localeText={dataGridLocaleText}
          // The default no-rows overlay only shows a skeleton (no rows are known yet on the
          // very first load), so force a progress bar here to give a visible loading signal
          // whenever `loading` is true, regardless of row count.
          slotProps={{
            loadingOverlay: {
              variant: 'linear-progress',
              noRowsVariant: 'linear-progress',
            },
          }}
          autoHeight
        />
      </ListingTable.Container>
    </ListingTable.Provider>
  );
}
