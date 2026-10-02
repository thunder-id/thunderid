// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Button, Chip, DataGrid, IconButton, ListingTable, Tooltip, Typography} from '@wso2/oxygen-ui';
import {KeyRound, Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import GatewayValueDeleteDialog from './GatewayValueDeleteDialog';
import GatewayValueFormDialog, {type GatewayValueFormValues} from './GatewayValueFormDialog';
import useCreateGatewaySecret from '../api/useCreateGatewaySecret';
import useDeleteGatewaySecret from '../api/useDeleteGatewaySecret';
import useGetGatewaySecrets from '../api/useGetGatewaySecrets';
import useUpdateGatewaySecret from '../api/useUpdateGatewaySecret';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {GatewaySecret} from '../models/gateway';
import getGatewayValuesErrorMessage from '../utils/getGatewayValuesErrorMessage';

export interface GatewaySecretsCardProps {
  gatewayId: string;
}

type FormTarget = {mode: 'create'} | {mode: 'replace'; secret: GatewaySecret};

/**
 * The secrets a gateway holds, read from and written to the gateway's own store. A secret's value
 * is never returned, so it can only be replaced.
 */
export default function GatewaySecretsCard({gatewayId}: GatewaySecretsCardProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('GatewaySecretsCard');
  const dataGridLocaleText = useDataGridLocaleText();
  const [paginationModel, setPaginationModel] = useState({page: 0, pageSize: 10});
  const {data, isLoading, error, refetch} = useGetGatewaySecrets(gatewayId, {
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
  });
  const createSecret = useCreateGatewaySecret();
  const updateSecret = useUpdateGatewaySecret();
  const deleteSecret = useDeleteGatewaySecret();
  const [formTarget, setFormTarget] = useState<FormTarget | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<GatewaySecret | null>(null);

  const saveMutation = formTarget?.mode === 'replace' ? updateSecret : createSecret;

  const closeForm = (): void => {
    createSecret.reset();
    updateSecret.reset();
    setFormTarget(null);
  };

  const closeDelete = (): void => {
    deleteSecret.reset();
    setDeleteTarget(null);
  };

  const handleSubmit = (values: GatewayValueFormValues): void => {
    const description = values.description.trim() ? values.description.trim() : undefined;
    const onError = (err: Error): void => {
      logger.error('Failed to save gateway secret', {error: err});
    };

    if (formTarget?.mode === 'replace') {
      updateSecret.mutate(
        {gatewayId, name: formTarget.secret.name, data: {value: values.value, description}},
        {onSuccess: closeForm, onError},
      );
      return;
    }
    createSecret.mutate(
      {gatewayId, data: {name: values.name.trim(), value: values.value, description}},
      {onSuccess: closeForm, onError},
    );
  };

  const handleDelete = (): void => {
    if (!deleteTarget) return;
    deleteSecret.mutate(
      {gatewayId, name: deleteTarget.name},
      {
        onSuccess: closeDelete,
        onError: (err: Error) => {
          logger.error('Failed to delete gateway secret', {error: err});
        },
      },
    );
  };

  const columns: DataGrid.GridColDef<GatewaySecret>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('gateways:values.columns.name', 'Name'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewaySecret>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontWeight: 500}}>
            {params.row.name}
          </Typography>
        ),
      },
      {
        field: 'description',
        headerName: t('gateways:values.columns.description', 'Description'),
        flex: 1,
        minWidth: 180,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewaySecret>) =>
          params.row.description ? (
            <Typography variant="body2">{params.row.description}</Typography>
          ) : (
            <Typography variant="body2" color="text.disabled">
              -
            </Typography>
          ),
      },
      {
        field: 'exists',
        headerName: t('gateways:secrets.columns.value', 'Value'),
        width: 120,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewaySecret>) =>
          params.row.exists ? (
            <Chip size="small" color="success" variant="outlined" label={t('gateways:secrets.set', 'Set')} />
          ) : (
            <Chip size="small" variant="outlined" label={t('gateways:secrets.notSet', 'Not set')} />
          ),
      },
      {
        field: 'actions',
        headerName: t('gateways:values.columns.actions', 'Actions'),
        width: 120,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewaySecret>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('gateways:secrets.replace.action', 'Replace value')}>
              <IconButton
                size="small"
                aria-label={t('gateways:secrets.replaceAction', 'Replace the value of {{name}}', {
                  name: params.row.name,
                })}
                onClick={() => setFormTarget({mode: 'replace', secret: params.row})}
              >
                <KeyRound size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton
                size="small"
                color="error"
                aria-label={t('gateways:secrets.deleteAction', 'Delete {{name}}', {name: params.row.name})}
                onClick={() => setDeleteTarget(params.row)}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </ListingTable.RowActions>
        ),
      },
    ],
    [t],
  );

  return (
    <SettingsCard
      title={t('gateways:secrets.title', 'Secrets')}
      description={t(
        'gateways:secrets.description',
        'Credentials this gateway holds that configuration refers to by name. A secret value is never shown, only replaced.',
      )}
      headerAction={
        <Button
          variant="outlined"
          startIcon={<Plus size={16} />}
          onClick={() => setFormTarget({mode: 'create'})}
          disabled={Boolean(error)}
        >
          {t('gateways:secrets.add', 'Add secret')}
        </Button>
      }
    >
      {error ? (
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="inline"
          fallbackKey="secrets.error"
          fallbackDefaultValue="Failed to load the secrets of this gateway"
          resolveErrorMessage={getGatewayValuesErrorMessage}
          onRetry={() => void refetch()}
        />
      ) : (
        <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
          <ListingTable.Container disablePaper>
            <ListingTable.DataGrid
              rows={data?.secrets ?? []}
              columns={columns}
              getRowId={(row) => (row as GatewaySecret).name}
              disableRowSelectionOnClick
              disableColumnFilter
              localeText={{
                ...dataGridLocaleText,
                noRowsLabel: t('gateways:secrets.empty', 'This gateway holds no secrets yet.'),
              }}
              paginationMode="server"
              rowCount={data?.totalResults ?? 0}
              paginationModel={paginationModel}
              onPaginationModelChange={setPaginationModel}
              pageSizeOptions={[10, 25, 50, 100]}
              autoHeight
            />
          </ListingTable.Container>
        </ListingTable.Provider>
      )}

      {formTarget && (
        <GatewayValueFormDialog
          idPrefix="gateway-secret"
          title={
            formTarget.mode === 'replace'
              ? t('gateways:secrets.replace.title', 'Replace secret value')
              : t('gateways:secrets.create.title', 'Add a secret')
          }
          description={
            formTarget.mode === 'replace'
              ? t(
                  'gateways:secrets.replace.description',
                  'The current value is not shown. Enter a new value to replace it.',
                )
              : undefined
          }
          nameEditable={formTarget.mode === 'create'}
          initialValues={
            formTarget.mode === 'replace'
              ? {name: formTarget.secret.name, value: '', description: formTarget.secret.description ?? ''}
              : {name: '', value: '', description: ''}
          }
          valueLabel={
            formTarget.mode === 'replace'
              ? t('gateways:secrets.form.newValue', 'New value')
              : t('gateways:values.form.value.label', 'Value')
          }
          valueHint={t('gateways:secrets.form.valueHint', 'The value is stored on the gateway and never shown again.')}
          secretValue
          submitLabel={
            formTarget.mode === 'replace'
              ? t('gateways:secrets.replace.submit', 'Replace')
              : t('gateways:values.form.add', 'Add')
          }
          submittingLabel={t('common:status.saving', 'Saving...')}
          isPending={saveMutation.isPending}
          error={
            saveMutation.error
              ? getGatewayValuesErrorMessage(
                  saveMutation.error,
                  tForErrors,
                  'secrets.save.error',
                  'The secret could not be saved. Please try again.',
                )
              : undefined
          }
          onEdit={() => {
            if (saveMutation.isError) saveMutation.reset();
          }}
          onSubmit={handleSubmit}
          onClose={closeForm}
        />
      )}

      {deleteTarget && (
        <GatewayValueDeleteDialog
          title={t('gateways:secrets.delete.title', 'Delete secret')}
          message={t('gateways:secrets.delete.message', 'Delete the secret {{name}} from this gateway?', {
            name: deleteTarget.name,
          })}
          isPending={deleteSecret.isPending}
          error={
            deleteSecret.error
              ? getGatewayValuesErrorMessage(
                  deleteSecret.error,
                  tForErrors,
                  'secrets.delete.error',
                  'The secret could not be deleted. Please try again.',
                )
              : undefined
          }
          onConfirm={handleDelete}
          onClose={closeDelete}
        />
      )}
    </SettingsCard>
  );
}
