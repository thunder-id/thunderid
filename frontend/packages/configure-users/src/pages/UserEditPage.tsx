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

import {
  PageLoadingAnimation,
  ResourceAvatar,
  SettingsCard,
  UnsavedChangesBar,
  getInitials,
} from '@thunderid/components';
import {useResolveDisplayName} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import type {User} from '@thunderid/types';
import {
  Box,
  Stack,
  Typography,
  Button,
  Alert,
  Chip,
  Tabs,
  Tab,
  TextField,
  InputAdornment,
  Tooltip,
  IconButton,
  PageContent,
  PageTitle,
  FormControl,
  FormLabel,
} from '@wso2/oxygen-ui';
import {ArrowLeft, Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useState, useEffect, useMemo, useCallback, useRef, type SyntheticEvent, type ReactNode, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetUser from '../api/useGetUser';
import useGetUserType from '../api/useGetUserType';
import useGetUserTypes from '../api/useGetUserTypes';
import useUpdateUser from '../api/useUpdateUser';
import AttributesSummarySection from '../components/edit-user/AttributesSummarySection';
import CredentialsTabPanel, {type CredentialFieldInfo} from '../components/edit-user/CredentialsTabPanel';
import EditUserAttributes from '../components/edit-user/EditUserAttributes';
import QuickCopySection from '../components/edit-user/QuickCopySection';
import UserDeleteDialog from '../components/UserDeleteDialog';
import UserConstants from '../constants/user-constants';
import useUserRoutes from '../hooks/useUserRoutes';

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
      id={`user-tabpanel-${index}`}
      aria-labelledby={`user-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

interface TabConfig {
  key: string;
  label: string;
  render: () => ReactNode;
}

export default function UserEditPage() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UserEditPage');
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const {userId} = useParams<{userId: string}>();
  const routes = useUserRoutes();

  const [activeTab, setActiveTab] = useState(0);
  const [editedUser, setEditedUser] = useState<Partial<User>>({});
  // Bumped on Save/Reset to force EditUserAttributes to remount with a clean form — it keeps
  // its own react-hook-form state locally, which a `setEditedUser({})` alone wouldn't reset.
  const [attributesResetKey, setAttributesResetKey] = useState(0);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const {data: user, isLoading: isUserLoading, error: userError, refetch} = useGetUser(userId);
  const updateUserMutation = useUpdateUser();

  // Get all schemas to find the schema ID from the schema name
  const {data: userTypeList} = useGetUserTypes();

  // Find the schema ID based on the user's type (which is the schema name)
  const matchedSchema = userTypeList?.types?.find((s) => s.name === user?.type);

  const schemaId = matchedSchema?.id;
  const trimmedOuId = matchedSchema?.ouId?.trim();
  const schemaOuId = trimmedOuId === '' ? undefined : trimmedOuId;

  const {data: userTypeDetails, isLoading: isSchemaLoading, error: schemaError} = useGetUserType(schemaId);

  const credentialFields: CredentialFieldInfo[] = useMemo(() => {
    if (!userTypeDetails?.schema) return [];
    return Object.entries(userTypeDetails.schema)
      .filter(([, fieldDef]) => (fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential)
      .map(([fieldName, fieldDef]) => {
        let label = fieldName;
        if (fieldDef.displayName) {
          const resolved = resolveDisplayName(fieldDef.displayName);
          if (resolved) label = resolved;
        }
        return {fieldName, label};
      });
  }, [userTypeDetails, resolveDisplayName]);

  const displayName = user?.display ?? user?.id ?? '';

  useEffect(
    () => () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    },
    [],
  );

  const handleCopyToClipboard = useCallback(async (text: string, fieldName: string): Promise<void> => {
    await navigator.clipboard.writeText(text);
    setCopiedField(fieldName);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => {
      setCopiedField(null);
    }, 2000);
  }, []);

  const handleTabChange = (_event: SyntheticEvent, newValue: number) => {
    setActiveTab(newValue);
  };

  const handleFieldChange = useCallback((field: keyof User, value: unknown) => {
    setEditedUser((prev) => ({...prev, [field]: value}));
  }, []);

  const handleSave = useCallback(async () => {
    const organizationUnitId = schemaOuId ?? user?.ouId;
    if (!userId || !organizationUnitId || !user?.type) return;

    try {
      await updateUserMutation.mutateAsync({
        userId,
        data: {
          ouId: organizationUnitId,
          type: user.type,
          attributes: editedUser.attributes ?? user.attributes,
        },
      });
      setEditedUser({});
      setAttributesResetKey((key) => key + 1);
      await refetch();
    } catch (err) {
      logger.error('Failed to update user', {error: err});
    }
  }, [schemaOuId, user, userId, editedUser, updateUserMutation, refetch, logger]);

  const hasChanges = Object.keys(editedUser).length > 0;

  const handleBack = async () => {
    await navigate(routes.list());
  };

  const handleDeleteSuccess = () => {
    (async () => {
      await navigate(routes.list());
    })().catch((error: unknown) => {
      logger.error('Failed to navigate after deleting user', {error});
    });
  };

  // Loading state
  if (isUserLoading || isSchemaLoading) {
    return <PageLoadingAnimation />;
  }

  // Error state
  if (userError ?? schemaError) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {userError?.message ?? schemaError?.message ?? 'Failed to load user information'}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('users:manageUser.back')}
        </Button>
      </PageContent>
    );
  }

  // No user found
  if (!user) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('users:manageUser.notFound', 'User not found')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('users:manageUser.back')}
        </Button>
      </PageContent>
    );
  }

  const picture = user.attributes?.['picture'] as string | undefined;

  const tabs: TabConfig[] = [
    {
      key: 'general',
      label: t('users:manageUser.tabs.general', 'General'),
      render: () => (
        <Stack spacing={3}>
          <QuickCopySection user={user} copiedField={copiedField} onCopyToClipboard={handleCopyToClipboard} />

          <AttributesSummarySection user={user} />

          {/* Organization Unit */}
          <SettingsCard
            title={t('users:manageUser.sections.organizationUnit.title', 'Organization Unit')}
            description={t(
              'users:manageUser.sections.organizationUnit.description',
              'The organization unit this user belongs to.',
            )}
          >
            <Stack spacing={2}>
              <FormControl fullWidth>
                <FormLabel htmlFor="ou-handle-input">
                  {t('users:manageUser.sections.organizationUnit.handleLabel', 'Handle')}
                </FormLabel>
                <TextField
                  id="ou-handle-input"
                  value={user.ouHandle ?? '-'}
                  fullWidth
                  size="small"
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: user.ouHandle ? (
                        <InputAdornment position="end">
                          <Tooltip
                            title={
                              copiedField === 'ouHandle'
                                ? t('common:actions.copied')
                                : t(
                                    'users:manageUser.sections.organizationUnit.copyHandle',
                                    'Copy Organization Unit Handle',
                                  )
                            }
                          >
                            <IconButton
                              aria-label={t(
                                'users:manageUser.sections.organizationUnit.copyHandle',
                                'Copy Organization Unit Handle',
                              )}
                              onClick={() => {
                                handleCopyToClipboard(user.ouHandle!, 'ouHandle').catch(() => null);
                              }}
                              edge="end"
                            >
                              {copiedField === 'ouHandle' ? <Check size={16} /> : <Copy size={16} />}
                            </IconButton>
                          </Tooltip>
                        </InputAdornment>
                      ) : undefined,
                    },
                  }}
                  sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
              </FormControl>
              <FormControl fullWidth>
                <FormLabel htmlFor="ou-id-input">
                  {t('users:manageUser.sections.organizationUnit.idLabel', 'ID')}
                </FormLabel>
                <TextField
                  id="ou-id-input"
                  value={user.ouId}
                  fullWidth
                  size="small"
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <Tooltip
                            title={
                              copiedField === 'ouId'
                                ? t('common:actions.copied')
                                : t('users:manageUser.sections.organizationUnit.copyId', 'Copy Organization Unit ID')
                            }
                          >
                            <IconButton
                              aria-label={t(
                                'users:manageUser.sections.organizationUnit.copyId',
                                'Copy Organization Unit ID',
                              )}
                              onClick={() => {
                                handleCopyToClipboard(user.ouId, 'ouId').catch(() => null);
                              }}
                              edge="end"
                            >
                              {copiedField === 'ouId' ? <Check size={16} /> : <Copy size={16} />}
                            </IconButton>
                          </Tooltip>
                        </InputAdornment>
                      ),
                    },
                  }}
                  sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
              </FormControl>
            </Stack>
          </SettingsCard>

          {/* Danger Zone */}
          {!user.isReadOnly && (
            <SettingsCard
              title={t('users:manageUser.sections.dangerZone.title', 'Danger Zone')}
              description={t(
                'users:manageUser.sections.dangerZone.description',
                'Irreversible and destructive actions.',
              )}
            >
              <Typography variant="h6" gutterBottom color="error">
                {t('users:manageUser.sections.dangerZone.deleteUser', 'Delete User')}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
                {t(
                  'users:manageUser.sections.dangerZone.deleteUserDescription',
                  'Once deleted, this user cannot be recovered. All associated data will be permanently removed.',
                )}
              </Typography>
              <Button variant="contained" color="error" onClick={() => setDeleteDialogOpen(true)}>
                {t('common:actions.delete', 'Delete')}
              </Button>
            </SettingsCard>
          )}
        </Stack>
      ),
    },
    {
      key: 'attributes',
      label: t('users:manageUser.tabs.attributes', 'Attributes'),
      render: () => (
        <EditUserAttributes
          key={attributesResetKey}
          user={user}
          editedUser={editedUser}
          onFieldChange={handleFieldChange}
        />
      ),
    },
  ];

  if (!user.isReadOnly && credentialFields.length > 0) {
    tabs.push({
      key: 'credentials',
      label: t('users:manageUser.tabs.credentials', 'Credentials'),
      render: () => <CredentialsTabPanel userId={userId!} credentialFields={credentialFields} />,
    });
  }

  const safeActiveTab = activeTab >= tabs.length ? 0 : activeTab;

  return (
    <PageContent>
      {user.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={routes.list()} />}>
          {t('users:manageUser.back', 'Back to Users')}
        </PageTitle.BackButton>
        <PageTitle.Avatar>
          <ResourceAvatar
            value={picture}
            fallback={`${UserConstants.DEFAULT_AVATAR_PREFIX}${getInitials(displayName)}`}
            size={55}
          />
        </PageTitle.Avatar>
        <PageTitle.Header>
          <Typography variant="h3">{displayName}</Typography>
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Chip label={user.type} size="small" sx={{px: 0.5}} />
          </Stack>
        </PageTitle.SubHeader>
      </PageTitle>

      {/* Tabs */}
      <Tabs value={safeActiveTab} onChange={handleTabChange} aria-label="user settings tabs">
        {tabs.map((tab, idx) => (
          <Tab
            key={tab.key}
            label={tab.label}
            id={`user-tab-${idx}`}
            aria-controls={`user-tabpanel-${idx}`}
            sx={{textTransform: 'none'}}
          />
        ))}
      </Tabs>

      {tabs.map((tab, idx) => (
        <TabPanel key={tab.key} value={safeActiveTab} index={idx}>
          {tab.render()}
        </TabPanel>
      ))}

      {/* Delete Dialog */}
      <UserDeleteDialog
        open={deleteDialogOpen}
        userId={userId ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />

      {hasChanges && (
        <UnsavedChangesBar
          message={t('users:manageUser.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('users:manageUser.reset', 'Reset')}
          saveLabel={t('users:manageUser.save', 'Save')}
          savingLabel={t('users:manageUser.saving', 'Saving…')}
          isSaving={updateUserMutation.isPending}
          saveDisabled={user.isReadOnly === true}
          onReset={() => {
            setEditedUser({});
            setAttributesResetKey((key) => key + 1);
          }}
          onSave={() => {
            void handleSave();
          }}
        />
      )}
    </PageContent>
  );
}
