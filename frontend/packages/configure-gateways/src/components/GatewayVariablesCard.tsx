// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Button, DataGrid, IconButton, ListingTable, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Pencil, Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import GatewayValueDeleteDialog from './GatewayValueDeleteDialog';
import GatewayValueFormDialog, {type GatewayValueFormValues} from './GatewayValueFormDialog';
import useCreateGatewayVariable from '../api/useCreateGatewayVariable';
import useDeleteGatewayVariable from '../api/useDeleteGatewayVariable';
import useGetGatewayVariables from '../api/useGetGatewayVariables';
import useUpdateGatewayVariable from '../api/useUpdateGatewayVariable';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {GatewayVariable} from '../models/gateway';
import getGatewayValuesErrorMessage from '../utils/getGatewayValuesErrorMessage';

export interface GatewayVariablesCardProps {
  gatewayId: string;
}

type FormTarget = {mode: 'create'} | {mode: 'edit'; variable: GatewayVariable};

/**
 * The variables a gateway holds, read from and written to the gateway's own store.
 */
export default function GatewayVariablesCard({gatewayId}: GatewayVariablesCardProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('GatewayVariablesCard');
  const dataGridLocaleText = useDataGridLocaleText();
  const [paginationModel, setPaginationModel] = useState({page: 0, pageSize: 10});
  const {data, isLoading, error, refetch} = useGetGatewayVariables(gatewayId, {
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
  });
  const createVariable = useCreateGatewayVariable();
  const updateVariable = useUpdateGatewayVariable();
  const deleteVariable = useDeleteGatewayVariable();
  const [formTarget, setFormTarget] = useState<FormTarget | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<GatewayVariable | null>(null);

  const saveMutation = formTarget?.mode === 'edit' ? updateVariable : createVariable;

  const closeForm = (): void => {
    createVariable.reset();
    updateVariable.reset();
    setFormTarget(null);
  };

  const closeDelete = (): void => {
    deleteVariable.reset();
    setDeleteTarget(null);
  };

  const handleSubmit = (values: GatewayValueFormValues): void => {
    const description = values.description.trim() ? values.description.trim() : undefined;
    const onError = (err: Error): void => {
      logger.error('Failed to save gateway variable', {error: err});
    };

    if (formTarget?.mode === 'edit') {
      updateVariable.mutate(
        {gatewayId, name: formTarget.variable.name, data: {value: values.value, description}},
        {onSuccess: closeForm, onError},
      );
      return;
    }
    createVariable.mutate(
      {gatewayId, data: {name: values.name.trim(), value: values.value, description}},
      {onSuccess: closeForm, onError},
    );
  };

  const handleDelete = (): void => {
    if (!deleteTarget) return;
    deleteVariable.mutate(
      {gatewayId, name: deleteTarget.name},
      {
        onSuccess: closeDelete,
        onError: (err: Error) => {
          logger.error('Failed to delete gateway variable', {error: err});
        },
      },
    );
  };

  const columns: DataGrid.GridColDef<GatewayVariable>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('gateways:values.columns.name', 'Name'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewayVariable>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontWeight: 500}}>
            {params.row.name}
          </Typography>
        ),
      },
      {
        field: 'value',
        headerName: t('gateways:values.columns.value', 'Value'),
        flex: 1,
        minWidth: 180,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewayVariable>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace'}} noWrap>
            {params.row.value}
          </Typography>
        ),
      },
      {
        field: 'description',
        headerName: t('gateways:values.columns.description', 'Description'),
        flex: 1,
        minWidth: 180,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<GatewayVariable>) =>
          params.row.description ? (
            <Typography variant="body2">{params.row.description}</Typography>
          ) : (
            <Typography variant="body2" color="text.disabled">
              -
            </Typography>
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
        renderCell: (params: DataGrid.GridRenderCellParams<GatewayVariable>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit', 'Edit')}>
              <IconButton
                size="small"
                aria-label={t('gateways:variables.editAction', 'Edit {{name}}', {name: params.row.name})}
                onClick={() => setFormTarget({mode: 'edit', variable: params.row})}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton
                size="small"
                color="error"
                aria-label={t('gateways:variables.deleteAction', 'Delete {{name}}', {name: params.row.name})}
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
      title={t('gateways:variables.title', 'Variables')}
      description={t(
        'gateways:variables.description',
        'Values this gateway holds that configuration refers to by name. Variable values are visible.',
      )}
      headerAction={
        <Button
          variant="outlined"
          startIcon={<Plus size={16} />}
          onClick={() => setFormTarget({mode: 'create'})}
          disabled={Boolean(error)}
        >
          {t('gateways:variables.add', 'Add variable')}
        </Button>
      }
    >
      {error ? (
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="inline"
          fallbackKey="variables.error"
          fallbackDefaultValue="Failed to load the variables of this gateway"
          resolveErrorMessage={getGatewayValuesErrorMessage}
          onRetry={() => void refetch()}
        />
      ) : (
        <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
          <ListingTable.Container disablePaper>
            <ListingTable.DataGrid
              rows={data?.variables ?? []}
              columns={columns}
              getRowId={(row) => (row as GatewayVariable).name}
              disableRowSelectionOnClick
              disableColumnFilter
              localeText={{
                ...dataGridLocaleText,
                noRowsLabel: t('gateways:variables.empty', 'This gateway holds no variables yet.'),
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
          idPrefix="gateway-variable"
          title={
            formTarget.mode === 'edit'
              ? t('gateways:variables.edit.title', 'Edit variable')
              : t('gateways:variables.create.title', 'Add a variable')
          }
          nameEditable={formTarget.mode === 'create'}
          initialValues={
            formTarget.mode === 'edit'
              ? {
                  name: formTarget.variable.name,
                  value: formTarget.variable.value,
                  description: formTarget.variable.description ?? '',
                }
              : {name: '', value: '', description: ''}
          }
          valueLabel={t('gateways:values.form.value.label', 'Value')}
          submitLabel={
            formTarget.mode === 'edit' ? t('common:actions.save', 'Save') : t('gateways:values.form.add', 'Add')
          }
          submittingLabel={t('common:status.saving', 'Saving...')}
          isPending={saveMutation.isPending}
          error={
            saveMutation.error
              ? getGatewayValuesErrorMessage(
                  saveMutation.error,
                  tForErrors,
                  'variables.save.error',
                  'The variable could not be saved. Please try again.',
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
          title={t('gateways:variables.delete.title', 'Delete variable')}
          message={t('gateways:variables.delete.message', 'Delete the variable {{name}} from this gateway?', {
            name: deleteTarget.name,
          })}
          isPending={deleteVariable.isPending}
          error={
            deleteVariable.error
              ? getGatewayValuesErrorMessage(
                  deleteVariable.error,
                  tForErrors,
                  'variables.delete.error',
                  'The variable could not be deleted. Please try again.',
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
