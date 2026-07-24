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

import {useIsMutating} from '@tanstack/react-query';
import {PageLoadingAnimation} from '@thunderid/components';
import {arePermissionsEqual, type ResourcePermissions} from '@thunderid/configure-resource-servers';
import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {isEqualIgnoringEmpty} from '@thunderid/utils';
import {
  Box,
  Stack,
  Typography,
  Button,
  TextField,
  Paper,
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
import useGetRole from '../api/useGetRole';
import useUpdateRole, {ROLE_MUTATION_KEY} from '../api/useUpdateRole';
import EditAssignmentsSettings from '../components/edit-role/assignments-settings/EditAssignmentsSettings';
import EditGeneralSettings from '../components/edit-role/general-settings/EditGeneralSettings';
import EditPermissionsSettings from '../components/edit-role/permissions-settings/EditPermissionsSettings';
import RoleDeleteDialog from '../components/RoleDeleteDialog';
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
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('RoleEditPage');
  const {showToast} = useToast();

  const {data: role, isLoading, error: fetchError, refetch} = useGetRole(roleId ?? '');
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

  const handleFieldChange = useCallback((field: keyof Role, value: unknown): void => {
    setEditedRole((prev) => ({...prev, [field]: value}));
  }, []);

  const serverPermissions = useMemo(() => role?.permissions ?? [], [role]);

  const handlePermissionsChange = useCallback(
    (next: ResourcePermissions[]): void => {
      setEditedRole((prev) => {
        if (arePermissionsEqual(next, serverPermissions)) {
          const {permissions: _permissions, ...rest} = prev;
          void _permissions;
          return rest;
        }
        return {...prev, permissions: next};
      });
    },
    [serverPermissions],
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
      const message = err instanceof Error ? err.message : t('roles:edit.page.saveError');
      showToast(message, 'error');
    }
  }, [role, roleId, editedRole, updateRole, refetch, logger, showToast, t]);

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
        <Alert severity="error" sx={{mb: 2}}>
          {fetchError.message ?? t('roles:edit.page.error')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('roles:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  if (!role) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('roles:edit.page.notFound')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('roles:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {role.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>{t('roles:edit.page.back')}</PageTitle.BackButton>
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
                  if (trimmed && trimmed !== current) {
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
                {!role.isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('roles:edit.page.editName')}
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
                placeholder={t('roles:edit.page.description.placeholder')}
                sx={{maxWidth: '600px', '& .MuiInputBase-root': {fontSize: '0.875rem'}}}
              />
            ) : (
              <>
                <Typography component="span" variant="body2" color="text.secondary">
                  {effectiveDescription || t('roles:edit.page.description.empty')}
                </Typography>
                {!role.isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('roles:edit.page.editDescription')}
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
      <Tabs value={activeTab} onChange={handleTabChange} aria-label={t('roles:edit.page.settingsTabs')}>
        <Tab
          label={t('roles:edit.page.tabs.general')}
          id="role-tab-0"
          aria-controls="role-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('roles:edit.page.tabs.permissions')}
          id="role-tab-1"
          aria-controls="role-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('roles:edit.page.tabs.assignments')}
          id="role-tab-2"
          aria-controls="role-tabpanel-2"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        <TabPanel value={activeTab} index={0}>
          <EditGeneralSettings
            role={role}
            onDeleteClick={role.isReadOnly ? undefined : () => setDeleteDialogOpen(true)}
          />
        </TabPanel>

        <TabPanel value={activeTab} index={1}>
          <EditPermissionsSettings
            permissions={editedRole.permissions ?? serverPermissions}
            onPermissionsChange={handlePermissionsChange}
            isReadOnly={role.isReadOnly}
          />
        </TabPanel>

        <TabPanel value={activeTab} index={2}>
          <EditAssignmentsSettings roleId={role.id} isReadOnly={role.isReadOnly} />
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
        <Paper
          sx={{
            position: 'fixed',
            bottom: 0,
            left: 0,
            right: 0,
            p: 2,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 2,
            borderRadius: '12px 12px 0 0',
            boxShadow: '0 -4px 20px rgba(0, 0, 0, 0.1)',
            zIndex: 1000,
            bgcolor: 'background.paper',
          }}
        >
          <Stack direction="row" spacing={2} alignItems="center">
            <Typography variant="body2" sx={{display: 'flex', alignItems: 'center', gap: 1}}>
              <Box
                component="span"
                sx={{
                  width: 20,
                  height: 20,
                  borderRadius: '50%',
                  border: '2px solid',
                  borderColor: 'warning.main',
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '12px',
                  fontWeight: 'bold',
                }}
              >
                !
              </Box>
              {t('roles:edit.page.unsavedChanges')}
            </Typography>
            <Button
              variant="outlined"
              color="error"
              onClick={() => {
                setEditedRole({});
              }}
            >
              {t('roles:edit.page.reset')}
            </Button>
            <Button
              variant="contained"
              onClick={() => {
                handleSave().catch(() => {
                  /* noop */
                });
              }}
              disabled={updateRole.isPending || isRoleUpdating || role.isReadOnly === true}
            >
              {updateRole.isPending ? t('roles:edit.page.saving') : t('roles:edit.page.save')}
            </Button>
          </Stack>
        </Paper>
      )}
    </PageContent>
  );
}
