// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useHiddenResources} from '@thunderid/contexts';
import {useDataGridLocaleText, useDebouncedValue} from '@thunderid/hooks';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  DataGrid,
  IconButton,
  InputAdornment,
  ListingTable,
  Stack,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import {Search, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {SetValueButton, ValueName, ValueText} from './ValueCells';
import ValueEditDialog from './ValueEditDialog';
import type {ValueRow} from './ValuesTable';
import {
  isValueName,
  pendingWriteOf,
  useDeleteStoredValue,
  usePendingValueWrites,
  useStoredValuesPage,
} from '../api/useStoredValues';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useGatewayValueAccess from '../hooks/useGatewayValueAccess';
import type {ValueKind, ValueReference} from '../models/deploy';
import {isHeld, maskHiddenResources, pageRowsOf} from '../utils/valueRows';

export interface StoredValuesListProps {
  gatewayId: string;
  kind: ValueKind;
  /** The places the version the gateway runs refers to values, to say what each row is for. */
  references: ValueReference[];
  /** The places the configuration as it stands refers to values, for a name the running version does not. */
  current?: ValueReference[];
}

/** How long typing in the search waits before the server is asked. */
const SEARCH_DELAY = 300;

/** The page sizes offered; a gateway's store answers with at most 100 at a time. */
const PAGE_SIZES = [10, 25, 50, 100];

/**
 * The variables or the secrets a gateway holds, a page at a time as the server answers them, searched by
 * the start of their name on the server. Each is set or removed in place. A secret's value is never shown;
 * setting one replaces it.
 */
export default function StoredValuesList({
  gatewayId,
  kind,
  references,
  current = [],
}: StoredValuesListProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const dataGridLocaleText = useDataGridLocaleText();
  const {canValue} = useGatewayValueAccess();
  const {isHiddenResource} = useHiddenResources();
  const canSet = canValue(kind, 'update');
  const canDelete = canValue(kind, 'delete');
  const secret = kind === 'secret';

  const [search, setSearch] = useState('');
  const prefix = useDebouncedValue(search.trim(), SEARCH_DELAY);
  const invalidSearch = prefix !== '' && !isValueName(prefix);
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({page: 0, pageSize: 10});
  // Another search starts from its first page.
  const [searched, setSearched] = useState(prefix);
  if (searched !== prefix) {
    setSearched(prefix);
    setPaginationModel({...paginationModel, page: 0});
  }

  const {data, isLoading, isPlaceholderData, error, refetch} = useStoredValuesPage(
    gatewayId,
    kind,
    {
      limit: paginationModel.pageSize,
      offset: paginationModel.page * paginationModel.pageSize,
      prefix: invalidSearch ? '' : prefix,
    },
    !invalidSearch,
  );
  // A page left empty, as by removing its last value, gives way to the one before it. Every value the
  // server counted is shown, a hidden resource's too, so the rows, the total and this check agree.
  if (!isPlaceholderData && data?.values.length === 0 && paginationModel.page > 0) {
    setPaginationModel({...paginationModel, page: paginationModel.page - 1});
  }

  const rows: ValueRow[] = useMemo(
    () => maskHiddenResources(pageRowsOf(kind, data?.values ?? [], references, current), isHiddenResource),
    [kind, data, references, current, isHiddenResource],
  );

  const deleteValue = useDeleteStoredValue(gatewayId, kind);
  // Writes started in any view of the gateway, so a value is not set while it is being removed, nor
  // removed while it is being saved: the one that lands last would undo the other.
  const pending = usePendingValueWrites(gatewayId);
  const [editing, setEditing] = useState<ValueRow | null>(null);

  const columns: DataGrid.GridColDef<ValueRow>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('values.name', 'Name'),
        flex: 1,
        minWidth: 240,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<ValueRow>) => (
          <Box sx={{py: 1}}>
            <ValueName row={params.row} withResource />
          </Box>
        ),
      },
      {
        field: 'value',
        headerName: t('values.value', 'Value'),
        flex: 1,
        minWidth: 240,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<ValueRow>) => (
          <Box sx={{py: 1}}>
            <ValueText row={params.row} kind={kind} />
          </Box>
        ),
      },
      {
        field: 'actions',
        headerName: '',
        width: 170,
        align: 'right',
        sortable: false,
        filterable: false,
        hideable: false,
        disableColumnMenu: true,
        renderCell: (params: DataGrid.GridRenderCellParams<ValueRow>) => {
          const row = params.row;
          if (row.hiddenUse) return null;
          const held = isHeld(row, secret);
          return (
            <ListingTable.RowActions>
              {canSet && (
                <SetValueButton
                  row={row}
                  kind={kind}
                  onClick={() => {
                    deleteValue.reset();
                    setEditing(row);
                  }}
                  disabled={pending.removing.has(pendingWriteOf(kind, row.name))}
                />
              )}
              {held && canDelete && (
                <Tooltip title={t('values.delete', 'Remove')}>
                  <IconButton
                    size="small"
                    color="error"
                    aria-label={t('values.deleteLabel', 'Remove {{name}}', {name: row.name})}
                    onClick={() => deleteValue.mutate(row.name)}
                    disabled={deleteValue.isPending || pending.setting.has(pendingWriteOf(kind, row.name))}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Tooltip>
              )}
            </ListingTable.RowActions>
          );
        },
      },
    ],
    [t, kind, secret, canSet, canDelete, deleteValue, pending],
  );

  let noRowsLabel: string;
  if (prefix) {
    noRowsLabel = secret
      ? t('values.noSecretsMatch', 'No secrets have a name that starts with {{prefix}}.', {prefix})
      : t('values.noVariablesMatch', 'No variables have a name that starts with {{prefix}}.', {prefix});
  } else {
    noRowsLabel = secret
      ? t('values.noSecrets', 'This gateway holds no secrets.')
      : t('values.noVariables', 'This gateway holds no variables.');
  }

  return (
    <Stack spacing={2} data-testid={`values-list-${kind}`}>
      <TextField
        size="small"
        fullWidth
        value={search}
        onChange={(event) => setSearch(event.target.value)}
        placeholder={
          secret
            ? t('values.searchSecrets', 'Search secrets by the start of their name')
            : t('values.searchVariables', 'Search variables by the start of their name')
        }
        error={invalidSearch}
        helperText={
          invalidSearch
            ? t(
                'values.searchInvalid',
                'A name starts with a letter or an underscore and holds only letters, digits and underscores.',
              )
            : undefined
        }
        slotProps={{
          htmlInput: {
            'aria-label': secret
              ? t('values.searchSecretsLabel', 'Search secrets')
              : t('values.searchVariablesLabel', 'Search variables'),
          },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={16} />
              </InputAdornment>
            ),
          },
        }}
      />
      {deleteValue.error && (
        <Alert severity="error" onClose={() => deleteValue.reset()}>
          {getErrorMessage(
            deleteValue.error,
            tForErrors,
            'deploy:values.deleteFailed',
            'The value could not be removed.',
          )}
        </Alert>
      )}
      {error && !data ? (
        <QueryErrorNotice error={error} t={tForErrors} variant="inline" onRetry={() => void refetch()} />
      ) : (
        <>
          {error && <QueryErrorNotice error={error} t={tForErrors} variant="inline" onRetry={() => void refetch()} />}
          <ListingTable.Provider variant="data-grid-card" loading={isLoading || isPlaceholderData}>
            <ListingTable.Container disablePaper>
              <ListingTable.DataGrid
                rows={invalidSearch ? [] : rows}
                columns={columns}
                getRowId={(row) => (row as ValueRow).name}
                getRowHeight={() => 'auto'}
                paginationMode="server"
                rowCount={invalidSearch ? 0 : (data?.totalResults ?? 0)}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={PAGE_SIZES}
                disableRowSelectionOnClick
                disableColumnFilter
                localeText={{...dataGridLocaleText, noRowsLabel}}
                autoHeight
              />
            </ListingTable.Container>
          </ListingTable.Provider>
        </>
      )}
      <ValueEditDialog gatewayId={gatewayId} kind={kind} row={editing} onClose={() => setEditing(null)} />
    </Stack>
  );
}
