// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  DocumentTitle,
  PageLoadingAnimation,
  QueryErrorNotice,
  ResourceAvatar,
  UnsavedChangesBar,
} from '@thunderid/components';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
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
import {Navigate, useNavigate, useParams, useLocation, Link} from 'react-router';
import useGetOrganizationUnit from '../api/useGetOrganizationUnit';
import useUpdateOrganizationUnit from '../api/useUpdateOrganizationUnit';
import EditChildOrganizationUnitSettings from '../components/edit-organization-unit/child-organization-unit-settings/EditChildOrganizationUnitSettings';
import EditCustomization from '../components/edit-organization-unit/customization-settings/EditCustomizationSettings';
import DangerZoneSection from '../components/edit-organization-unit/general-settings/DangerZoneSection';
import EditGeneralSettings from '../components/edit-organization-unit/general-settings/EditGeneralSettings';
import EditGroups from '../components/edit-organization-unit/group-settings/EditGroupSettings';
import EditUsers from '../components/edit-organization-unit/user-settings/EditUserSettings';
import OrganizationUnitDeleteDialog from '../components/OrganizationUnitDeleteDialog';
import OrganizationUnitConstraints from '../constants/organization-unit-constraints';
import OrganizationUnitTreeConstants from '../constants/organization-unit-tree-constants';
import useOrganizationUnit from '../contexts/useOrganizationUnit';
import useOrganizationUnitRoutes from '../hooks/useOrganizationUnitRoutes';
import type {OUNavigationState} from '../models/navigation';
import type {OrganizationUnit} from '../models/organization-unit';

interface TabPanelProps {
  children?: ReactNode;
  index: string;
  value: string;
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

/**
 * Props passed to a {@link OrganizationUnitEditPageProps.renderDefaultFlowsSettings} renderer.
 */
export interface DefaultFlowsSettingsRenderProps {
  organizationUnit: OrganizationUnit;
  editedOU: Partial<OrganizationUnit>;
  onFieldChange: (field: keyof OrganizationUnit, value: unknown) => void;
}

export interface OrganizationUnitEditPageProps {
  /**
   * Renders the "Default Flows" tab content. This package has no access to the console app's
   * flow-fetching API, so the consuming app supplies the tab via this render prop instead. The
   * tab is omitted entirely when this is not provided.
   */
  renderDefaultFlowsSettings?: (renderProps: DefaultFlowsSettingsRenderProps) => ReactNode;
}

export default function OrganizationUnitEditPage({
  renderDefaultFlowsSettings = undefined,
}: OrganizationUnitEditPageProps): JSX.Element {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const location = useLocation();
  const routes = useOrganizationUnitRoutes();
  const {t} = useTranslation();
  const logger = useLogger('OrganizationUnitEditPage');
  const {can, confinedOuId} = useCapabilitiesContext();

  // Resolves an error through the `organizationUnits` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `organizationUnits:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `organizationUnits:${key}`, options),
    [t],
  );

  // Check if we came from another OU (via parent or child OU link)
  const navigationState = location.state as OUNavigationState | null;
  const fromOU = navigationState?.fromOU;

  const {data: organizationUnit, isLoading, error: fetchError, refetch} = useGetOrganizationUnit(id);
  const updateOrganizationUnit = useUpdateOrganizationUnit();
  const {resetTreeState} = useOrganizationUnit();
  const {projectId} = useProjectScope();

  const [tabState, setTabState] = useState<{organizationUnitId: string | undefined; value: string}>({
    organizationUnitId: id,
    value: 'general',
  });

  if (tabState.organizationUnitId !== id) {
    setTabState({organizationUnitId: id, value: 'general'});
  }
  const [editedOU, setEditedOU] = useState<Partial<OrganizationUnit>>({});
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
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
    : t('organizationUnits:edit.page.back', 'Back to organization units');

  const handleTabChange = (_event: SyntheticEvent, newValue: string): void => {
    setTabState({organizationUnitId: id, value: newValue});
  };

  const handleFieldChange = useCallback(
    (field: keyof OrganizationUnit, value: unknown): void => {
      updateOrganizationUnit.reset(); // a save error is stale once the form changes
      setEditedOU((prev) => ({...prev, [field]: value}));
    },
    [updateOrganizationUnit],
  );

  const commitName = useCallback(
    (value: string): void => {
      const trimmedName = value.trim();
      // The API rejects names outside these bounds, so an out of range rename is discarded here.
      if (
        trimmedName.length >= OrganizationUnitConstraints.NAME_MIN_LENGTH &&
        trimmedName.length <= OrganizationUnitConstraints.NAME_MAX_LENGTH
      ) {
        handleFieldChange('name', trimmedName);
      }
    },
    [handleFieldChange],
  );

  const commitDescription = useCallback(
    (value: string): void => {
      const trimmedDescription = value.trim();
      if (trimmedDescription === (organizationUnit?.description ?? '')) {
        setEditedOU((prev) => {
          if (!('description' in prev)) return prev;
          const next = {...prev};
          delete next.description;
          return next;
        });
      } else {
        handleFieldChange('description', trimmedDescription || null);
      }
    },
    [organizationUnit, handleFieldChange],
  );

  const handleSave = useCallback(async (): Promise<void> => {
    if (!organizationUnit || !id) return;

    const updatedData = {
      handle: editedOU.handle ?? organizationUnit.handle,
      name: editedOU.name ?? organizationUnit.name,
      description: editedOU.description !== undefined ? editedOU.description : organizationUnit.description,
      parent: organizationUnit.parent ?? null,
      themeId: editedOU.themeId !== undefined ? editedOU.themeId : organizationUnit.themeId,
      layoutId: editedOU.layoutId !== undefined ? editedOU.layoutId : organizationUnit.layoutId,
      authFlowId: editedOU.authFlowId !== undefined ? editedOU.authFlowId : organizationUnit.authFlowId,
      registrationFlowId:
        editedOU.registrationFlowId !== undefined ? editedOU.registrationFlowId : organizationUnit.registrationFlowId,
      isRegistrationFlowEnabled: editedOU.isRegistrationFlowEnabled ?? organizationUnit.isRegistrationFlowEnabled,
      recoveryFlowId: editedOU.recoveryFlowId !== undefined ? editedOU.recoveryFlowId : organizationUnit.recoveryFlowId,
      isRecoveryFlowEnabled: editedOU.isRecoveryFlowEnabled ?? organizationUnit.isRecoveryFlowEnabled,
      signOutFlowId: editedOU.signOutFlowId !== undefined ? editedOU.signOutFlowId : organizationUnit.signOutFlowId,
      isSignOutFlowEnabled: editedOU.isSignOutFlowEnabled ?? organizationUnit.isSignOutFlowEnabled,
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

  const hasChanges = useMemo(
    () =>
      Object.entries(editedOU).some(
        ([key, value]) => !isEqualIgnoringEmpty(value, organizationUnit?.[key as keyof OrganizationUnit]),
      ),
    [editedOU, organizationUnit],
  );

  const handleDeleteSuccess = (): void => {
    resetTreeState();
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate after deleting organization unit', {error: _error});
    });
  };

  const resourceTypeTitle = t('navigation:pages.organizationUnits', 'Organization units');
  // An admin confined to an organization unit cannot change or delete that unit; its parent manages it.
  const isConfinedUnit = Boolean(confinedOuId) && organizationUnit?.id === confinedOuId;
  const isReadOnly = Boolean(organizationUnit?.isReadOnly) || isConfinedUnit || !can('ou', 'update');

  if (isLoading) {
    return (
      <>
        <DocumentTitle title={resourceTypeTitle} />
        <PageLoadingAnimation />
      </>
    );
  }

  if (fetchError) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
        <QueryErrorNotice
          error={fetchError}
          t={tForErrors}
          variant="block"
          title={t('organizationUnits:edit.page.errorTitle', 'Failed to load organization unit')}
          fallbackKey="organizationUnits:edit.page.error"
          fallbackDefaultValue="Failed to load organization unit information"
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
              {t('organizationUnits:edit.page.back', 'Back to organization units')}
            </Button>
          }
        />
      </PageContent>
    );
  }

  if (!organizationUnit) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
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
          {t('organizationUnits:edit.page.back', 'Back to organization units')}
        </Button>
      </PageContent>
    );
  }

  // Another project's organization unit is not shown in the selected one: a deep link, a stale tab or a switch
  // of project goes to the selected project's organization units instead.
  if (projectId && organizationUnit.projectId !== projectId) {
    return <Navigate to={listUrl} replace />;
  }

  // An admin confined to an organization unit reads the units directly beneath it, but not what is inside them:
  // listing their children, users and groups is refused, so those tabs are left out.
  // TODO: This repeats the server's rule. Read the reach from the server instead, for example a capabilities tier
  // for direct child units or GET /capabilities?ouId=<unit>.
  const canSeeContents = !confinedOuId || organizationUnit.id === confinedOuId;
  const tabs: {key: string; label: string; panel: ReactNode; visible: boolean}[] = [
    {
      key: 'general',
      label: t('organizationUnits:edit.page.tabs.general'),
      panel: <EditGeneralSettings organizationUnit={organizationUnit} />,
      visible: true,
    },
    {
      key: 'childOUs',
      label: t('organizationUnits:edit.page.tabs.childOUs'),
      panel: (
        <EditChildOrganizationUnitSettings organizationUnitId={id!} organizationUnitName={organizationUnit.name} />
      ),
      visible: canSeeContents,
    },
    {
      key: 'users',
      label: t('organizationUnits:edit.page.tabs.users'),
      panel: <EditUsers organizationUnitId={id!} />,
      visible: canSeeContents,
    },
    {
      key: 'groups',
      label: t('organizationUnits:edit.page.tabs.groups'),
      panel: <EditGroups organizationUnitId={id!} />,
      visible: canSeeContents,
    },
    {
      key: 'defaultFlows',
      label: t('organizationUnits:edit.page.tabs.defaultFlows'),
      panel: renderDefaultFlowsSettings?.({
        organizationUnit: isConfinedUnit ? {...organizationUnit, isReadOnly: true} : organizationUnit,
        editedOU,
        onFieldChange: handleFieldChange,
      }),
      visible: Boolean(renderDefaultFlowsSettings),
    },
    {
      key: 'customization',
      label: t('organizationUnits:edit.page.tabs.customization'),
      panel: (
        <EditCustomization organizationUnit={organizationUnit} editedOU={editedOU} onFieldChange={handleFieldChange} />
      ),
      visible: true,
    },
    {
      key: 'advanced',
      label: t('organizationUnits:edit.page.tabs.advanced'),
      panel: !organizationUnit.isReadOnly && !isConfinedUnit && can('ou', 'delete') && (
        <DangerZoneSection onDeleteClick={() => setDeleteDialogOpen(true)} />
      ),
      visible: true,
    },
  ].filter((tab) => tab.visible);
  const activeTab = tabs.some((tab) => tab.key === tabState.value) ? tabState.value : 'general';

  return (
    <PageContent>
      <DocumentTitle title={organizationUnit.name} />
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
            editable={!isReadOnly}
            value={editedOU.logoUrl ?? organizationUnit.logoUrl ?? undefined}
            fallback={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
            editAriaLabel={t('organizationUnits:edit.page.logoUpdate.label', 'Update logo')}
            onSelect={(newLogoUrl: string) => {
              updateOrganizationUnit.reset(); // a save error is stale once the form changes
              setEditedOU((prev) => {
                if (newLogoUrl === organizationUnit.logoUrl) {
                  const {logoUrl, ...rest} = prev;
                  void logoUrl;
                  return rest;
                }
                return {...prev, logoUrl: newLogoUrl};
              });
            }}
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
                  commitName(tempName);
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    commitName(tempName);
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
                {!isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('organizationUnits:edit.page.editName', 'Edit organization unit name')}
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
                  commitDescription(tempDescription);
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    commitDescription(tempDescription);
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
                {!isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('organizationUnits:edit.page.editDescription', 'Edit description')}
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
        {tabs.map((tab) => (
          <Tab
            key={tab.key}
            value={tab.key}
            label={tab.label}
            id={`ou-tab-${tab.key}`}
            aria-controls={`ou-tabpanel-${tab.key}`}
            sx={{textTransform: 'none'}}
          />
        ))}
      </Tabs>

      {/* Tab Panels */}
      {tabs.map((tab) => (
        <TabPanel key={tab.key} value={activeTab} index={tab.key}>
          {tab.panel}
        </TabPanel>
      ))}

      {/* Delete Dialog */}
      <OrganizationUnitDeleteDialog
        open={deleteDialogOpen}
        organizationUnitId={id ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />

      {/* Floating Action Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={t('organizationUnits:edit.actions.unsavedChanges.label')}
          resetLabel={t('organizationUnits:edit.actions.reset.label')}
          saveLabel={t('organizationUnits:edit.actions.save.label')}
          savingLabel={t('organizationUnits:edit.actions.saving.label')}
          isSaving={updateOrganizationUnit.isPending}
          saveDisabled={isReadOnly}
          error={
            updateOrganizationUnit.error
              ? getErrorMessage(
                  updateOrganizationUnit.error,
                  tForErrors,
                  'update.error',
                  'Failed to update organization unit. Please try again.',
                )
              : undefined
          }
          onReset={() => {
            setEditedOU({});
            updateOrganizationUnit.reset();
          }}
          onSave={() => {
            // Errors are handled in handleSave
            handleSave().catch(() => null);
          }}
        />
      )}
    </PageContent>
  );
}
