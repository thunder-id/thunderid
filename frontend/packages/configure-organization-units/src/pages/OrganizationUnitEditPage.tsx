/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {PageLoadingAnimation, ResourceAvatar, UnsavedChangesBar} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
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
  Snackbar,
  PageContent,
  PageTitle,
} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useMemo} from 'react';
import type {ReactNode, SyntheticEvent, JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams, useLocation, Link} from 'react-router';
import useGetOrganizationUnit from '../api/useGetOrganizationUnit';
import useUpdateOrganizationUnit from '../api/useUpdateOrganizationUnit';
import EditChildOrganizationUnitSettings from '../components/edit-organization-unit/child-organization-unit-settings/EditChildOrganizationUnitSettings';
import EditCustomization from '../components/edit-organization-unit/customization-settings/EditCustomizationSettings';
import EditGeneralSettings from '../components/edit-organization-unit/general-settings/EditGeneralSettings';
import EditGroups from '../components/edit-organization-unit/group-settings/EditGroupSettings';
import EditUsers from '../components/edit-organization-unit/user-settings/EditUserSettings';
import OrganizationUnitDeleteDialog from '../components/OrganizationUnitDeleteDialog';
import OrganizationUnitTreeConstants from '../constants/organization-unit-tree-constants';
import useOrganizationUnit from '../contexts/useOrganizationUnit';
import useOrganizationUnitRoutes from '../hooks/useOrganizationUnitRoutes';
import type {OUNavigationState} from '../models/navigation';
import type {OrganizationUnit} from '../models/organization-unit';

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
      id={`ou-tabpanel-${index}`}
      aria-labelledby={`ou-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

export default function OrganizationUnitEditPage(): JSX.Element {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const location = useLocation();
  const routes = useOrganizationUnitRoutes();
  const {t} = useTranslation();
  const logger = useLogger('OrganizationUnitEditPage');

  // Check if we came from another OU (via parent or child OU link)
  const navigationState = location.state as OUNavigationState | null;
  const fromOU = navigationState?.fromOU;

  const {data: organizationUnit, isLoading, error: fetchError, refetch} = useGetOrganizationUnit(id);
  const updateOrganizationUnit = useUpdateOrganizationUnit();
  const {resetTreeState} = useOrganizationUnit();

  const [activeTab, setActiveTab] = useState(0);
  const [editedOU, setEditedOU] = useState<Partial<OrganizationUnit>>({});
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [snackbar, setSnackbar] = useState<{open: boolean; message: string}>({open: false, message: ''});
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempName, setTempName] = useState('');
  const [tempDescription, setTempDescription] = useState('');
  const listUrl = routes.list();

  const handleBack = async (): Promise<void> => {
    if (fromOU) {
      await navigate(routes.detail(fromOU.id));
    } else {
      await navigate(listUrl);
    }
  };

  const backButtonText = fromOU
    ? t('organizationUnits:edit.page.backToOU', {name: fromOU.name})
    : t('organizationUnits:edit.page.back');

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
  };

  const handleFieldChange = useCallback((field: keyof OrganizationUnit, value: unknown): void => {
    setEditedOU((prev) => ({...prev, [field]: value}));
  }, []);

  const handleSave = useCallback(async (): Promise<void> => {
    if (!organizationUnit || !id) return;

    const updatedData = {
      handle: editedOU.handle ?? organizationUnit.handle,
      name: editedOU.name ?? organizationUnit.name,
      description: editedOU.description !== undefined ? editedOU.description : organizationUnit.description,
      parent: organizationUnit.parent ?? null,
      themeId: editedOU.themeId !== undefined ? editedOU.themeId : organizationUnit.themeId,
      logoUrl: editedOU.logoUrl ?? organizationUnit.logoUrl,
    };

    try {
      await updateOrganizationUnit.mutateAsync({
        id,
        data: updatedData,
      });
      resetTreeState();
      setEditedOU({});
      await refetch();
    } catch {
      logger.error('Failed to update organization unit');
    }
  }, [organizationUnit, id, editedOU, updateOrganizationUnit, resetTreeState, refetch, logger]);

  const hasChanges = useMemo(() => Object.keys(editedOU).length > 0, [editedOU]);

  const handleDeleteSuccess = (): void => {
    resetTreeState();
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate after deleting organization unit', {error: _error});
    });
  };

  const handleDeleteError = (message: string): void => {
    setSnackbar({open: true, message});
  };

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (fetchError) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {fetchError.message ?? t('organizationUnits:edit.page.error')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('organizationUnits:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  if (!organizationUnit) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('organizationUnits:edit.page.notFound')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('organizationUnits:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {organizationUnit.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={fromOU ? routes.detail(fromOU.id) : listUrl} />}>
          {backButtonText}
        </PageTitle.BackButton>
        <PageTitle.Avatar variant="rounded" sx={{overflow: 'visible'}}>
          <ResourceAvatar
            size={55}
            variant="rounded"
            supportedShapes={['rounded']}
            editable={!organizationUnit.isReadOnly}
            value={editedOU.logoUrl ?? organizationUnit.logoUrl ?? undefined}
            fallback={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
            editAriaLabel={t('organizationUnits:edit.page.logoUpdate.label', 'Update Logo')}
            onSelect={(newLogoUrl: string) =>
              setEditedOU((prev) => {
                if (newLogoUrl === organizationUnit.logoUrl) {
                  const {logoUrl, ...rest} = prev;
                  void logoUrl;
                  return rest;
                }
                return {...prev, logoUrl: newLogoUrl};
              })
            }
            onSave={handleSave}
          />
        </PageTitle.Avatar>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  if (tempName.trim()) {
                    handleFieldChange('name', tempName.trim());
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    if (tempName.trim()) {
                      handleFieldChange('name', tempName.trim());
                    }
                    setIsEditingName(false);
                  } else if (e.key === 'Escape') {
                    setTempName(editedOU.name ?? organizationUnit.name);
                    setIsEditingName(false);
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{editedOU.name ?? organizationUnit.name}</Typography>
                {!organizationUnit.isReadOnly && (
                  <IconButton
                    size="small"
                    onClick={() => {
                      setTempName(editedOU.name ?? organizationUnit.name);
                      setIsEditingName(true);
                    }}
                    sx={{
                      opacity: 0.6,
                      '&:hover': {opacity: 1},
                    }}
                  >
                    <Edit size={16} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <Stack direction="row" alignItems="flex-start" spacing={1}>
            {isEditingDescription ? (
              <TextField
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={() => {
                  const trimmedDescription = tempDescription.trim();
                  if (trimmedDescription !== (organizationUnit.description ?? '')) {
                    handleFieldChange('description', trimmedDescription || null);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmedDescription = tempDescription.trim();
                    if (trimmedDescription !== (organizationUnit.description ?? '')) {
                      handleFieldChange('description', trimmedDescription || null);
                    }
                    setIsEditingDescription(false);
                  } else if (e.key === 'Escape') {
                    setTempDescription(
                      (editedOU.description !== undefined ? editedOU.description : organizationUnit.description) ?? '',
                    );
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('organizationUnits:edit.page.description.placeholder')}
                sx={{
                  maxWidth: '600px',
                  '& .MuiInputBase-root': {
                    fontSize: '0.875rem',
                  },
                }}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {(editedOU.description !== undefined ? editedOU.description : organizationUnit.description) ??
                    t('organizationUnits:edit.page.description.empty')}
                </Typography>
                {!organizationUnit.isReadOnly && (
                  <IconButton
                    size="small"
                    onClick={() => {
                      setTempDescription(
                        (editedOU.description !== undefined ? editedOU.description : organizationUnit.description) ??
                          '',
                      );
                      setIsEditingDescription(true);
                    }}
                    sx={{
                      opacity: 0.6,
                      '&:hover': {opacity: 1},
                      mt: -0.5,
                    }}
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
      <Tabs value={activeTab} onChange={handleTabChange} aria-label="organization unit settings tabs">
        <Tab
          label={t('organizationUnits:edit.page.tabs.general')}
          id="ou-tab-0"
          aria-controls="ou-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('organizationUnits:edit.page.tabs.childOUs')}
          id="ou-tab-1"
          aria-controls="ou-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('organizationUnits:edit.page.tabs.users')}
          id="ou-tab-2"
          aria-controls="ou-tabpanel-2"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('organizationUnits:edit.page.tabs.groups')}
          id="ou-tab-3"
          aria-controls="ou-tabpanel-3"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('organizationUnits:edit.page.tabs.customization')}
          id="ou-tab-4"
          aria-controls="ou-tabpanel-4"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        {/* General Settings Tab */}
        <TabPanel value={activeTab} index={0}>
          <EditGeneralSettings
            organizationUnit={organizationUnit}
            onDeleteClick={organizationUnit.isReadOnly ? undefined : () => setDeleteDialogOpen(true)}
          />
        </TabPanel>

        {/* Child OUs Tab */}
        <TabPanel value={activeTab} index={1}>
          <EditChildOrganizationUnitSettings organizationUnitId={id!} organizationUnitName={organizationUnit.name} />
        </TabPanel>

        {/* Users Tab */}
        <TabPanel value={activeTab} index={2}>
          <EditUsers organizationUnitId={id!} />
        </TabPanel>

        {/* Groups Tab */}
        <TabPanel value={activeTab} index={3}>
          <EditGroups organizationUnitId={id!} />
        </TabPanel>

        {/* Customization Tab */}
        <TabPanel value={activeTab} index={4}>
          <EditCustomization
            organizationUnit={organizationUnit}
            editedOU={editedOU}
            onFieldChange={handleFieldChange}
          />
        </TabPanel>
      </>

      {/* Delete Dialog */}
      <OrganizationUnitDeleteDialog
        open={deleteDialogOpen}
        organizationUnitId={id ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
        onError={handleDeleteError}
      />

      <Snackbar
        open={snackbar.open}
        autoHideDuration={6000}
        onClose={() => setSnackbar((prev) => ({...prev, open: false}))}
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
      >
        <Alert onClose={() => setSnackbar((prev) => ({...prev, open: false}))} severity="error" sx={{width: '100%'}}>
          {snackbar.message}
        </Alert>
      </Snackbar>

      {/* Floating Action Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={t('organizationUnits:edit.actions.unsavedChanges.label')}
          resetLabel={t('organizationUnits:edit.actions.reset.label')}
          saveLabel={t('organizationUnits:edit.actions.save.label')}
          savingLabel={t('organizationUnits:edit.actions.saving.label')}
          isSaving={updateOrganizationUnit.isPending}
          saveDisabled={organizationUnit.isReadOnly === true}
          onReset={() => setEditedOU({})}
          onSave={() => {
            // Errors are handled in handleSave
            handleSave().catch(() => null);
          }}
        />
      )}
    </PageContent>
  );
}
