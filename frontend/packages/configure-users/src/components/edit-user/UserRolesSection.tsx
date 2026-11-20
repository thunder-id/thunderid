// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {Alert, Avatar, Box, Button, DataGrid, IconButton, Stack, Tooltip} from '@wso2/oxygen-ui';
import {Eye, Plus, ShieldCheck, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddUserRolesDialog from './AddUserRolesDialog';
import useAddUserToRoles from '../../api/useAddUserToRoles';
import useGetAssignableRoles from '../../api/useGetAssignableRoles';
import useGetUserRoles from '../../api/useGetUserRoles';
import useRemoveUserFromRole from '../../api/useRemoveUserFromRole';
import type {UserRole} from '../../models/users';
import getUserErrorMessage from '../../utils/getUserErrorMessage';

interface UserRolesSectionProps {
  userId: string;
}

/**
 * Lists the roles assigned to the user and lets them be added and removed, in the style of a role's
 * Assignments table. Inside a project only the project's roles are shown and offered.
 */
export default function UserRolesSection({userId}: UserRolesSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {projectId} = useProjectScope();
  const {can} = useCapabilitiesContext();
  const canEdit = can('role', 'update');
  const dataGridLocaleText = useDataGridLocaleText();
  // The roles the user could hold are read first: the user's own are found among them.
  const {
    data: assignable,
    isLoading: assignableLoading,
    error: assignableError,
    refetch: refetchAssignable,
  } = useGetAssignableRoles(projectId);
  const {
    data,
    isLoading: assignedLoading,
    error: assignedError,
    refetch: refetchAssigned,
  } = useGetUserRoles(userId, assignable?.roles);
  const addToRoles = useAddUserToRoles();
  const removeFromRole = useRemoveUserFromRole();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});

  const error = assignableError ?? assignedError;
  const assigned: UserRole[] = useMemo(() => data ?? [], [data]);
  const unassigned: UserRole[] = (assignable?.roles ?? []).filter(
    (role) => !assigned.some((held) => held.id === role.id),
  );

  // Resolves an error through the `users` catalog, forwarding explicit `ns:` prefixes unchanged.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `users:${key}`, options),
    [t],
  );

  const closeAddDialog = (): void => {
    setAddDialogOpen(false);
    addToRoles.reset();
  };

  const columns: DataGrid.GridColDef<UserRole>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <Avatar sx={{width: 30, height: 30, bgcolor: 'primary.main', color: 'primary.contrastText'}}>
              <ShieldCheck size={14} aria-hidden />
            </Avatar>
          </Box>
        ),
      },
      {
        field: 'name',
        headerName: t('users:manageUser.sections.roles.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
      },
      {
        field: 'id',
        headerName: t('users:manageUser.sections.roles.columns.id', 'ID'),
        flex: 1,
        minWidth: 250,
      },
      ...(canEdit
        ? [
            {
              field: 'actions',
              headerName: t('users:manageUser.sections.roles.columns.actions', 'Actions'),
              renderHeader: (): null => null,
              width: 60,
              sortable: false,
              filterable: false,
              renderCell: (params: DataGrid.GridRenderCellParams<UserRole>): JSX.Element =>
                params.row.isReadOnly ? (
                  <Tooltip title={t('common:status.readOnly', 'Read only')}>
                    <IconButton
                      size="small"
                      disableRipple
                      sx={{cursor: 'default'}}
                      aria-label={t('common:status.readOnly', 'Read only')}
                    >
                      <Eye size={14} />
                    </IconButton>
                  </Tooltip>
                ) : (
                  <IconButton
                    size="small"
                    color="error"
                    aria-label={t('users:manageUser.sections.roles.remove', 'Remove {{role}}', {
                      role: params.row.name,
                    })}
                    disabled={removeFromRole.isPending}
                    onClick={(event) => {
                      event.stopPropagation();
                      removeFromRole.mutate({userId, roleId: params.row.id, ouId: params.row.assignmentOuId});
                    }}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                ),
            } satisfies DataGrid.GridColDef<UserRole>,
          ]
        : []),
    ],
    [t, canEdit, removeFromRole, userId],
  );

  return (
    <Stack spacing={3}>
      {removeFromRole.isError && (
        <Alert severity="error" onClose={() => removeFromRole.reset()}>
          {getUserErrorMessage(
            removeFromRole.error,
            tForErrors,
            'manageUser.sections.roles.removeError',
            'Failed to remove the role. Please try again.',
          )}
        </Alert>
      )}

      <SettingsCard
        title={t('users:manageUser.sections.roles.title', 'Assigned roles')}
        description={t(
          'users:manageUser.sections.roles.description',
          'Manage the roles assigned to this user. Roles held through a group are not listed.',
        )}
        headerAction={
          canEdit && !error && !assignableLoading && !assignedLoading ? (
            <Button
              variant="contained"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => {
                addToRoles.reset();
                setAddDialogOpen(true);
              }}
            >
              {t('common:actions.add', 'Add')}
            </Button>
          ) : undefined
        }
        slotProps={{content: {sx: {p: error ? undefined : 0}}}}
      >
        {error ? (
          <QueryErrorNotice
            error={error}
            t={tForErrors}
            variant="inline"
            fallbackKey="users:manageUser.sections.roles.error"
            fallbackDefaultValue="Failed to load the roles of this user."
            onRetry={() => void (assignableError ? refetchAssignable() : refetchAssigned())}
          />
        ) : (
          <Box sx={{height: 400, width: '100%'}}>
            <DataGrid.DataGrid
              rows={assigned}
              columns={columns}
              loading={assignableLoading || assignedLoading}
              getRowId={(row): string => row.id}
              paginationModel={paginationModel}
              onPaginationModelChange={setPaginationModel}
              pageSizeOptions={[5, 10, 25]}
              disableRowSelectionOnClick
              localeText={{
                ...dataGridLocaleText,
                noRowsLabel: t('users:manageUser.sections.roles.empty', 'This user does not have any roles assigned.'),
              }}
              sx={{'--oxygen-shape-borderRadius': '0px', border: 'none'}}
            />
          </Box>
        )}
      </SettingsCard>

      {addDialogOpen && canEdit && (
        <AddUserRolesDialog
          open={addDialogOpen}
          roles={unassigned}
          onClose={closeAddDialog}
          onAdd={(roleIds) => addToRoles.mutate({userId, roleIds}, {onSuccess: closeAddDialog})}
          error={
            addToRoles.isError
              ? t('users:manageUser.sections.roles.assignError', 'Could not assign the roles {{roles}}. Try again.', {
                  roles: addToRoles.error.failedRoles.map((role) => role.name).join(', '),
                })
              : null
          }
          // Changing the selection clears a failed add. An add still in flight is left alone: resetting it
          // would unlock the dialog and drop the result of the request.
          onErrorDismiss={() => {
            if (addToRoles.isError) addToRoles.reset();
          }}
          isSubmitting={addToRoles.isPending}
        />
      )}
    </Stack>
  );
}
