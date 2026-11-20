// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDataGridLocaleText} from '@thunderid/hooks';
import {Alert, Box, Button, DataGrid, Dialog, DialogActions, DialogContent, DialogTitle} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {UserRole} from '../../models/users';

interface AddUserRolesDialogProps {
  open: boolean;
  /** The roles the user can still be given. */
  roles: UserRole[];
  onClose: () => void;
  onAdd: (roleIds: string[]) => void;
  /** Why the last add failed, shown beside the actions. */
  error?: string | null;
  onErrorDismiss?: () => void;
  isSubmitting?: boolean;
}

/**
 * Picks the roles to assign to a user, in the style of a role's Add assignment dialog.
 */
export default function AddUserRolesDialog({
  open,
  roles,
  onClose,
  onAdd,
  error = null,
  onErrorDismiss = undefined,
  isSubmitting = false,
}: AddUserRolesDialogProps): JSX.Element {
  const {t} = useTranslation();
  const dataGridLocaleText = useDataGridLocaleText();
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [selectionModel, setSelectionModel] = useState<DataGrid.GridRowSelectionModel>({
    type: 'include',
    ids: new Set(),
  });

  // Selecting every row is reported as an exclusion of none, so the chosen roles are worked out from both forms.
  const selectedIds: string[] = roles
    .filter((role) => selectionModel.ids.has(role.id) === (selectionModel.type === 'include'))
    .map((role) => role.id);

  const columns: DataGrid.GridColDef<UserRole>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('users:manageUser.sections.roles.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'description',
        headerName: t('users:manageUser.sections.roles.columns.description', 'Description'),
        flex: 2,
        minWidth: 250,
        valueGetter: (_value: unknown, row: UserRole) => row.description ?? '-',
      },
    ],
    [t],
  );

  return (
    <Dialog open={open} onClose={isSubmitting ? undefined : onClose} maxWidth="md" fullWidth>
      <DialogTitle>{t('users:manageUser.sections.roles.dialog.title', 'Add roles')}</DialogTitle>
      <DialogContent>
        <Box sx={{height: 400, width: '100%'}}>
          <DataGrid.DataGrid
            rows={roles}
            columns={columns}
            getRowId={(row): string => row.id}
            checkboxSelection
            rowSelectionModel={selectionModel}
            onRowSelectionModelChange={(next) => {
              setSelectionModel(next);
              onErrorDismiss?.();
            }}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            pageSizeOptions={[5, 10]}
            disableRowSelectionOnClick
            localeText={{
              ...dataGridLocaleText,
              noRowsLabel: t('users:manageUser.sections.roles.dialog.empty', 'There are no more roles to assign.'),
            }}
          />
        </Box>
      </DialogContent>
      {error && (
        <Box sx={{px: 3, pt: 2}}>
          <Alert severity="error">{error}</Alert>
        </Box>
      )}
      <DialogActions>
        <Button onClick={onClose} disabled={isSubmitting}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={() => onAdd(selectedIds)}
          disabled={selectedIds.length === 0 || isSubmitting}
        >
          {t('common:actions.add', 'Add')} {selectedIds.length > 0 ? `(${selectedIds.length})` : ''}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
