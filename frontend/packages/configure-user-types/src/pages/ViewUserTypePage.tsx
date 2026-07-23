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

import {PageLoadingAnimation, UnsavedChangesBar} from '@thunderid/components';
import {useToast} from '@thunderid/contexts';
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
  PageContent,
  PageTitle,
} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode, SyntheticEvent, JSX} from 'react';
import {useState, useMemo, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetUserType from '../api/useGetUserType';
import useUpdateUserType from '../api/useUpdateUserType';
import EditGeneralSettings from '../components/edit-user-type/general-settings/EditGeneralSettings';
import EditSchemaSettings from '../components/edit-user-type/schema-settings/EditSchemaSettings';
import UserTypeDeleteDialog from '../components/edit-user-type/UserTypeDeleteDialog';
import useUserTypeRoutes from '../hooks/useUserTypeRoutes';
import type {PropertyDefinition, UserTypeDefinition, PropertyType, SchemaPropertyInput} from '../types/user-types';

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
      id={`usertype-tabpanel-${index}`}
      aria-labelledby={`usertype-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

/**
 * Convert API schema to editable property inputs.
 */
function convertSchemaToProperties(schema: UserTypeDefinition): SchemaPropertyInput[] {
  return Object.entries(schema).map(([key, value], index) => ({
    id: `${index}`,
    name: key,
    displayName: 'displayName' in value ? (value.displayName ?? '') : '',
    type:
      value.type === 'string' && 'enum' in value && Array.isArray(value.enum) && value.enum.length > 0
        ? 'enum'
        : value.type,
    required: value.required ?? false,
    unique: 'unique' in value ? (value.unique ?? false) : false,
    credential: 'credential' in value ? (value.credential ?? false) : false,
    enum: 'enum' in value ? (value.enum ?? []) : [],
    regex: 'regex' in value ? (value.regex ?? '') : '',
    ...('items' in value ? {items: value.items} : {}),
    ...('properties' in value ? {properties: value.properties} : {}),
  }));
}

/**
 * Convert editable property inputs back to API schema format.
 */
function convertPropertiesToSchema(properties: SchemaPropertyInput[]): UserTypeDefinition {
  const schema: UserTypeDefinition = {};

  properties
    .filter((prop) => prop.name.trim())
    .forEach((prop) => {
      const actualType: PropertyType = prop.type === 'enum' ? 'string' : prop.type;

      const propDef: Partial<PropertyDefinition> = {
        type: actualType,
        required: prop.required,
        ...(prop.displayName.trim() ? {displayName: prop.displayName.trim()} : {}),
      };

      if (prop.unique) {
        (propDef as {unique?: boolean}).unique = true;
      }

      if ((prop.type === 'string' || prop.type === 'number' || prop.type === 'enum') && prop.credential) {
        (propDef as {credential?: boolean}).credential = true;
      }

      if (prop.type === 'string' || prop.type === 'enum') {
        if (prop.enum.length > 0) {
          (propDef as {enum?: string[]}).enum = prop.enum;
        }
        if (prop.regex.trim()) {
          (propDef as {regex?: string}).regex = prop.regex;
        }
      }

      if (prop.type === 'array') {
        (propDef as {items?: {type: string}}).items = prop.items ?? {type: 'string'};
      } else if (prop.type === 'object') {
        (propDef as {properties?: Record<string, PropertyDefinition>}).properties = prop.properties ?? {};
      }

      schema[prop.name.trim()] = propDef as PropertyDefinition;
    });

  return schema;
}

export default function ViewUserTypePage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('ViewUserTypePage');
  const {showToast} = useToast();
  const {id} = useParams<{id: string}>();
  const routes = useUserTypeRoutes();
  const listUrl = routes.list();

  const {data: userType, isLoading, error: fetchError} = useGetUserType(id);
  const updateUserTypeMutation = useUpdateUserType();

  // Tab state
  const [activeTab, setActiveTab] = useState(0);

  // Inline name editing
  const [isEditingName, setIsEditingName] = useState(false);
  const [tempName, setTempName] = useState('');

  // Edited fields (partial — only fields the user has changed)
  const [editedUserType, setEditedUserType] = useState<
    Partial<{name: string; ouId: string; allowSelfRegistration: boolean; displayAttribute: string}>
  >({});

  // Edited schema properties (null = no changes, non-null = user edited)
  const [editedProperties, setEditedProperties] = useState<SchemaPropertyInput[] | null>(null);

  // Delete dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Base properties from server data (useMemo so they're available synchronously)
  const baseProperties = useMemo(() => (userType ? convertSchemaToProperties(userType.schema) : []), [userType]);

  // Effective properties (edited or from server)
  const effectiveProperties = editedProperties ?? baseProperties;

  // Effective name
  const effectiveName = editedUserType.name ?? userType?.name ?? '';

  // Eligible display properties (computed from effective properties)
  const eligibleDisplayProperties = useMemo(
    () =>
      effectiveProperties.filter(
        (p) =>
          (p.type === 'string' || p.type === 'number' || p.type === 'enum') &&
          !p.credential &&
          p.name.trim().length > 0,
      ),
    [effectiveProperties],
  );

  // Clear display attribute if selected property becomes ineligible
  const effectiveDisplayAttribute = editedUserType.displayAttribute ?? userType?.systemAttributes?.display ?? '';
  const [prevEligible, setPrevEligible] = useState(eligibleDisplayProperties);
  if (prevEligible !== eligibleDisplayProperties) {
    setPrevEligible(eligibleDisplayProperties);
    if (effectiveDisplayAttribute) {
      const eligibleNames = eligibleDisplayProperties.map((p) => p.name.trim());
      if (!eligibleNames.includes(effectiveDisplayAttribute)) {
        setEditedUserType((prev) => ({...prev, displayAttribute: ''}));
      }
    }
  }

  // Change detection
  const hasChanges = useMemo(
    () => Object.keys(editedUserType).length > 0 || editedProperties !== null,
    [editedUserType, editedProperties],
  );

  const handleBack = async (): Promise<void> => {
    await navigate(listUrl);
  };

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
  };

  const handleFieldChange = useCallback((field: string, value: unknown): void => {
    setEditedUserType((prev) => ({...prev, [field]: value}));
  }, []);

  const handlePropertiesChange = useCallback((newProperties: SchemaPropertyInput[]): void => {
    setEditedProperties(newProperties);
  }, []);

  const handleReset = useCallback((): void => {
    setEditedUserType({});
    setEditedProperties(null);
    updateUserTypeMutation.reset();
  }, [updateUserTypeMutation]);

  const handleSave = useCallback(async (): Promise<void> => {
    if (!id || !userType) return;

    const name = (editedUserType.name ?? userType.name).trim();
    const ouId = (editedUserType.ouId ?? userType.ouId).trim();
    const allowSelfRegistration = editedUserType.allowSelfRegistration ?? userType.allowSelfRegistration;
    const displayAttribute = editedUserType.displayAttribute ?? userType.systemAttributes?.display ?? '';

    if (!ouId) {
      showToast(t('userTypes:validationErrors.ouIdRequired'), 'error');
      return;
    }

    // Check for duplicate property names
    const trimmedNames = effectiveProperties.filter((p) => p.name.trim()).map((p) => p.name.trim());
    const duplicates = trimmedNames.filter((n, i) => trimmedNames.indexOf(n) !== i);
    if (duplicates.length > 0) {
      showToast(
        t('userTypes:validationErrors.duplicateProperties', {duplicates: [...new Set(duplicates)].join(', ')}),
        'error',
      );
      return;
    }

    const schema = convertPropertiesToSchema(effectiveProperties);

    try {
      await updateUserTypeMutation.mutateAsync({
        userTypeId: id,
        data: {
          name,
          ouId,
          allowSelfRegistration,
          ...(displayAttribute ? {systemAttributes: {display: displayAttribute}} : {}),
          schema,
        },
      });
      setEditedUserType({});
      setEditedProperties(null);
    } catch (err: unknown) {
      logger.error('Failed to update user type', {error: err});
      const message = err instanceof Error ? err.message : t('userTypes:edit.saveError', 'Failed to save user type');
      showToast(message, 'error');
    }
  }, [id, userType, editedUserType, effectiveProperties, updateUserTypeMutation, logger, showToast, t]);

  const handleDeleteSuccess = (): void => {
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate after deleting user type', {error});
    });
  };

  // Loading state
  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  // Error state
  if (fetchError) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {fetchError.message ?? t('userTypes:edit.loadError', 'Failed to load user type information')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('userTypes:edit.back', 'Back to User Types')}
        </Button>
      </PageContent>
    );
  }

  // Not found
  if (!userType) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('userTypes:edit.notFound', 'User type not found')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('userTypes:edit.back', 'Back to User Types')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {userType.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('userTypes:edit.back', 'Back to User Types')}
        </PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  const trimmedName = tempName.trim();
                  if (trimmedName && trimmedName !== effectiveName) {
                    handleFieldChange('name', trimmedName);
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    const trimmedName = tempName.trim();
                    if (trimmedName && trimmedName !== effectiveName) {
                      handleFieldChange('name', trimmedName);
                    }
                    setIsEditingName(false);
                  } else if (e.key === 'Escape') {
                    setTempName(effectiveName);
                    setIsEditingName(false);
                  }
                }}
                size="small"
                inputProps={{'aria-label': t('userTypes:edit.nameInputAriaLabel', 'User type name')}}
              />
            ) : (
              <>
                <Typography variant="h3">{effectiveName}</Typography>
                {!userType.isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('userTypes:edit.editName', 'Edit user type name')}
                    onClick={() => {
                      setTempName(effectiveName);
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
      </PageTitle>

      {/* Tabs */}
      <Tabs value={activeTab} onChange={handleTabChange} aria-label="user type settings tabs">
        <Tab
          label={t('userTypes:edit.tabs.general', 'General')}
          id="usertype-tab-0"
          aria-controls="usertype-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('userTypes:edit.tabs.schema', 'Schema')}
          id="usertype-tab-1"
          aria-controls="usertype-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        <TabPanel value={activeTab} index={0}>
          <EditGeneralSettings
            userType={userType}
            editedOuId={editedUserType.ouId}
            editedAllowSelfRegistration={editedUserType.allowSelfRegistration}
            editedDisplayAttribute={editedUserType.displayAttribute}
            onFieldChange={handleFieldChange}
            onDeleteClick={userType.isReadOnly ? undefined : () => setDeleteDialogOpen(true)}
            eligibleDisplayProperties={eligibleDisplayProperties}
          />
        </TabPanel>

        <TabPanel value={activeTab} index={1}>
          <EditSchemaSettings
            properties={effectiveProperties}
            onPropertiesChange={handlePropertiesChange}
            userTypeName={effectiveName}
            disabled={userType.isReadOnly}
          />
        </TabPanel>
      </>

      {/* Delete Dialog */}
      <UserTypeDeleteDialog
        open={deleteDialogOpen}
        userTypeId={id ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />

      {/* Unsaved Changes Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={t('userTypes:edit.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('common:actions.reset', 'Reset')}
          saveLabel={t('common:actions.save', 'Save')}
          savingLabel={t('common:status.saving', 'Saving...')}
          isSaving={updateUserTypeMutation.isPending}
          saveDisabled={userType.isReadOnly === true}
          onReset={handleReset}
          onSave={() => {
            handleSave().catch(() => null);
          }}
        />
      )}
    </PageContent>
  );
}
