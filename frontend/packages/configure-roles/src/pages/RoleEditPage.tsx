// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useIsMutating} from '@tanstack/react-query';
import {ManagedResourceNotice, PageLoadingAnimation, QueryErrorNotice, UnsavedChangesBar} from '@thunderid/components';
import {arePermissionsEqual, type ResourcePermissions} from '@thunderid/configure-resource-servers';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage, isEqualIgnoringEmpty} from '@thunderid/utils';
import {
  Box,
  Stack,
  Typography,
  Button,
  TextField,
  Alert,
  IconButton,
  Tabs,
  Tab,
  PageContent,
  PageTitle,
} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useMemo} from 'react';
import type {ReactNode, SyntheticEvent, JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import {useIsManagedResource} from '@thunderid/contexts';
import useGetRole from '../api/useGetRole';
import useUpdateRole, {ROLE_MUTATION_KEY} from '../api/useUpdateRole';
import EditAdvancedSettings from '../components/edit-role/advanced-settings/EditAdvancedSettings';
import EditAssignmentsSettings from '../components/edit-role/assignments-settings/EditAssignmentsSettings';
import EditGeneralSettings from '../components/edit-role/general-settings/EditGeneralSettings';
import EditPermissionsSettings from '../components/edit-role/permissions-settings/EditPermissionsSettings';
import RoleDeleteDialog from '../components/RoleDeleteDialog';
import RoleConstraints from '../constants/role-constraints';
import type {Role} from '../models/role';

interface TabPanelProps {
  children?: ReactNode;
  index: number;
  value: number;
}

function TabPanel({children = null, value, index, ...other}: TabPanelProps): JSX.Element {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`role-tabpanel-${index}`}
      aria-labelledby={`role-tab-${index}`}
      style={value !== index ? {display: 'none'} : undefined}
      {...other}
    >
      <Box sx={{py: 3}}>{children}</Box>
    </div>
  );
}

export default function RoleEditPage(): JSX.Element {
  const {roleId} = useParams<{roleId: string}>();
  // A role applied from the control plane can only be changed there, so this view is read
  // only for it in the same way a declarative resource is.
  const isManagedRole = useIsManagedResource('role');
  const isManaged: boolean = isManagedRole(roleId ?? '');
  const navigate = useNavigate();
  const {t} = useTranslation('roles');
  const logger = useLogger('RoleEditPage');

  const {data: fetchedRole, isLoading, error: fetchError, refetch} = useGetRole(roleId ?? '');
  // A resource the control plane owns is read only here, and saying so on the object
  // itself is what makes every section of this page and its children treat it that way,
  // rather than each one having to learn about ownership separately.
  const role = useMemo(
    () => (isManaged && fetchedRole ? {...fetchedRole, isReadOnly: true} : fetchedRole),
    [fetchedRole, isManaged],
  );
  const updateRole = useUpdateRole();
  const isRoleUpdating = useIsMutating({mutationKey: ROLE_MUTATION_KEY}) > 0;

  const [activeTab, setActiveTab] = useState(0);
  const [editedRole, setEditedRole] = useState<Partial<Role>>({});
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempName, setTempName] = useState('');
  const [tempDescription, setTempDescription] = useState('');
  const listUrl = '/roles';

  const handleBack = async (): Promise<void> => {
    await navigate(listUrl);
  };

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
  };

  const handleFieldChange = useCallback(
    (field: keyof Role, value: unknown): void => {
      updateRole.reset(); // a save error is stale once the form changes
      setEditedRole((prev) => ({...prev, [field]: value}));
    },
    [updateRole],
  );

  const serverPermissions = useMemo(() => role?.permissions ?? [], [role]);

  const handlePermissionsChange = useCallback(
    (next: ResourcePermissions[]): void => {
      updateRole.reset(); // a save error is stale once the form changes
      setEditedRole((prev) => {
        if (arePermissionsEqual(next, serverPermissions)) {
          const {permissions: _permissions, ...rest} = prev;
          void _permissions;
          return rest;
        }
        return {...prev, permissions: next};
      });
    },
    [serverPermissions, updateRole],
  );

  const handleSave = useCallback(async (): Promise<void> => {
    if (!role || !roleId) return;

    const updatedData = {
      name: editedRole.name ?? role.name,
      description: 'description' in editedRole ? editedRole.description : role.description,
      ouId: role.ouId,
      permissions: editedRole.permissions ?? role.permissions ?? [],
    };

    try {
      await updateRole.mutateAsync({roleId, data: updatedData});
      setEditedRole({});
      await refetch();
    } catch (err: unknown) {
      logger.error('Failed to update role', {error: err});
    }
  }, [role, roleId, editedRole, updateRole, refetch, logger]);

  const hasChanges = useMemo(
    () => Object.entries(editedRole).some(([key, value]) => !isEqualIgnoringEmpty(value, role?.[key as keyof Role])),
    [editedRole, role],
  );

  const effectiveDescription = 'description' in editedRole ? (editedRole.description ?? '') : (role?.description ?? '');

  const handleDeleteSuccess = (): void => {
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate after deleting role', {error: _error});
    });
  };

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (fetchError) {
    return (
      <PageContent>
        <QueryErrorNotice
          error={fetchError}
          t={t}
          variant="block"
          title={t('edit.page.error', 'Failed to load role')}
          onRetry={() => void refetch()}
          action={
            <Button
              onClick={() => {
                handleBack().catch((error: unknown) => {
                  logger.error('Failed to navigate back', {error});
                });
              }}
              startIcon={<ArrowLeft size={16} />}
            >
              {t('edit.page.back', 'Back to Roles')}
            </Button>
          }
        />
      </PageContent>
    );
  }

  if (!role) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('edit.page.notFound', 'Role not found')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('edit.page.back', 'Back to Roles')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {/* A managed resource says where it can be changed; a declarative one has no such place. */}
      {isManaged && <ManagedResourceNotice />}
      {role.isReadOnly && !isManaged && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('edit.page.back', 'Back to Roles')}
        </PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  const trimmed = tempName.trim();
                  const current = (editedRole.name ?? role.name).trim();
                  // The API rejects names outside these bounds, so an out of range rename is discarded here.
                  if (
                    trimmed !== current &&
                    trimmed.length >= RoleConstraints.NAME_MIN_LENGTH &&
                    trimmed.length <= RoleConstraints.NAME_MAX_LENGTH
                  ) {
                    handleFieldChange('name', trimmed);
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    (e.target as HTMLInputElement).blur();
                  } else if (e.key === 'Escape') {
                    setTempName(editedRole.name ?? role.name);
                    (e.target as HTMLInputElement).blur();
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{editedRole.name ?? role.name}</Typography>
                {!(role.isReadOnly === true || isManaged || isManaged) && (
                  <IconButton
                    size="small"
                    aria-label={t('edit.page.editName', 'Edit role name')}
                    onClick={() => {
                      setTempName(editedRole.name ?? role.name);
                      setIsEditingName(true);
                    }}
                    sx={{opacity: 0.6, '&:hover': {opacity: 1}}}
                  >
                    <Edit size={16} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader component="div">
          <Stack direction="row" alignItems="flex-start" spacing={1}>
            {isEditingDescription ? (
              <TextField
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={() => {
                  const trimmed = tempDescription.trim();
                  if (trimmed !== effectiveDescription) {
                    handleFieldChange('description', trimmed || undefined);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    (e.target as HTMLInputElement).blur();
                  } else if (e.key === 'Escape') {
                    setTempDescription(effectiveDescription);
                    (e.target as HTMLInputElement).blur();
                  }
                }}
                size="small"
                placeholder={t('edit.page.description.placeholder', 'Add a description...')}
                sx={{maxWidth: '600px', '& .MuiInputBase-root': {fontSize: '0.875rem'}}}
              />
            ) : (
              <>
                <Typography component="span" variant="body2" color="text.secondary">
                  {effectiveDescription || t('edit.page.description.empty', 'No description')}
                </Typography>
                {!(role.isReadOnly === true || isManaged || isManaged) && (
                  <IconButton
                    size="small"
                    aria-label={t('edit.page.editDescription', 'Edit role description')}
                    onClick={() => {
                      setTempDescription(effectiveDescription);
                      setIsEditingDescription(true);
                    }}
                    sx={{opacity: 0.6, '&:hover': {opacity: 1}, mt: -0.5}}
                  >
                    <Edit size={14} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
        </PageTitle.SubHeader>
      </PageTitle>

      {/* Tabs */}
      <Tabs value={activeTab} onChange={handleTabChange} aria-label={t('edit.page.settingsTabs', 'Role settings tabs')}>
        <Tab
          label={t('edit.page.tabs.general', 'General')}
          id="role-tab-0"
          aria-controls="role-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('edit.page.tabs.permissions', 'Permissions')}
          id="role-tab-1"
          aria-controls="role-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('edit.page.tabs.assignments', 'Assignments')}
          id="role-tab-2"
          aria-controls="role-tabpanel-2"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('edit.page.tabs.advanced', 'Advanced')}
          id="role-tab-3"
          aria-controls="role-tabpanel-3"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        <TabPanel value={activeTab} index={0}>
          <EditGeneralSettings role={role} />
        </TabPanel>

        <TabPanel value={activeTab} index={1}>
          <EditPermissionsSettings
            permissions={editedRole.permissions ?? serverPermissions}
            onPermissionsChange={handlePermissionsChange}
            isReadOnly={role.isReadOnly === true || isManaged}
          />
        </TabPanel>

        <TabPanel value={activeTab} index={2}>
          <EditAssignmentsSettings roleId={role.id} isReadOnly={role.isReadOnly === true || isManaged} />
        </TabPanel>

        <TabPanel value={activeTab} index={3}>
          <EditAdvancedSettings
            onDeleteClick={role.isReadOnly === true || isManaged ? undefined : () => setDeleteDialogOpen(true)}
          />
        </TabPanel>
      </>

      {/* Delete Dialog */}
      <RoleDeleteDialog
        open={deleteDialogOpen}
        roleId={roleId ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />

      {/* Floating Action Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={t('edit.page.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('edit.page.reset', 'Reset')}
          saveLabel={t('edit.page.save', 'Save Changes')}
          savingLabel={t('edit.page.saving', 'Saving...')}
          isSaving={updateRole.isPending}
          saveDisabled={isRoleUpdating || role.isReadOnly === true}
          error={
            updateRole.error
              ? getErrorMessage(updateRole.error, t, 'update.error', 'Failed to update role. Please try again.')
              : undefined
          }
          onReset={() => {
            updateRole.reset();
            setEditedRole({});
          }}
          onSave={() => {
            handleSave().catch(() => {
              /* noop */
            });
          }}
        />
      )}
    </PageContent>
  );
}
