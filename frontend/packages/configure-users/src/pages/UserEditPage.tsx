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

import {ResourceAvatar, SettingsCard, getInitials} from '@thunderid/components';
import {useResolveDisplayName} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {
  Box,
  Stack,
  Typography,
  Button,
  CircularProgress,
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
import {ArrowLeft, Save, X, Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useState, useEffect, useMemo, useCallback, useRef, type SyntheticEvent, type ReactNode, type JSX} from 'react';
import {useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetUser from '../api/useGetUser';
import useGetUserType from '../api/useGetUserType';
import useGetUserTypes from '../api/useGetUserTypes';
import useUpdateUser from '../api/useUpdateUser';
import QuickCopySection from '../components/edit-user/QuickCopySection';
import UserDeleteDialog from '../components/UserDeleteDialog';
import renderSchemaField from '../utils/renderSchemaField';

type UpdateUserFormData = Record<string, string | number | boolean>;

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

export default function UserEditPage() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UserEditPage');
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const {userId} = useParams<{userId: string}>();

  const [activeTab, setActiveTab] = useState(0);
  const [isEditMode, setIsEditMode] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const {data: user, isLoading: isUserLoading, error: userError} = useGetUser(userId);
  const updateUserMutation = useUpdateUser();

  // Get all schemas to find the schema ID from the schema name
  const {data: userTypeList} = useGetUserTypes();

  // Find the schema ID based on the user's type (which is the schema name)
  const matchedSchema = useMemo(() => {
    if (!user?.type || !userTypeList?.types) {
      return undefined;
    }

    return userTypeList.types.find((s) => s.name === user.type);
  }, [user?.type, userTypeList?.types]);

  const schemaId = matchedSchema?.id;
  const trimmedOuId = matchedSchema?.ouId?.trim();
  const schemaOuId = trimmedOuId === '' ? undefined : trimmedOuId;

  const {data: userTypeDetails, isLoading: isSchemaLoading, error: schemaError} = useGetUserType(schemaId);

  const hasEditableFields = useMemo(() => {
    if (!userTypeDetails?.schema) return false;
    return Object.entries(userTypeDetails.schema).some(
      ([, fieldDef]) => !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential),
    );
  }, [userTypeDetails]);

  const displayName = user?.display ?? user?.id ?? '';

  const {
    control,
    handleSubmit,
    setValue,
    formState: {errors},
  } = useForm<UpdateUserFormData>({
    defaultValues: {},
  });

  // Populate form with user data when user data is loaded
  useEffect(() => {
    if (user?.attributes && userTypeDetails?.schema) {
      Object.entries(user.attributes).forEach(([key, value]) => {
        setValue(key, value as string | number | boolean);
      });
    }
  }, [user, userTypeDetails, setValue]);

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

  const onSubmit = async (data: UpdateUserFormData) => {
    const organizationUnitId = schemaOuId ?? user?.ouId;

    if (!userId || !organizationUnitId || !user?.type) return;

    try {
      setIsSubmitting(true);

      const requestBody = {
        ouId: organizationUnitId,
        type: user.type,
        attributes: data,
      };

      await updateUserMutation.mutateAsync({userId, data: requestBody});

      setIsEditMode(false);
    } catch (err) {
      logger.error('Failed to update user', {error: err});
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    setIsEditMode(false);
    updateUserMutation.reset();
    if (user?.attributes && userTypeDetails?.schema) {
      Object.entries(user.attributes).forEach(([key, value]) => {
        setValue(key, value as string | number | boolean);
      });
    }
  };

  const handleBack = async () => {
    await navigate('/users');
  };

  const handleDeleteSuccess = () => {
    (async () => {
      await navigate('/users');
    })().catch((error: unknown) => {
      logger.error('Failed to navigate after deleting user', {error});
    });
  };

  // Loading state
  if (isUserLoading || isSchemaLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px'}}>
        <CircularProgress />
      </Box>
    );
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

  return (
    <PageContent>
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to="/users" />}>
          {t('users:manageUser.back', 'Back to Users')}
        </PageTitle.BackButton>
        <PageTitle.Avatar>
          <ResourceAvatar value={picture} fallback={getInitials(displayName)} size={55} />
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
      <Tabs value={activeTab} onChange={handleTabChange} aria-label="user settings tabs">
        <Tab
          label={t('users:manageUser.tabs.general', 'General')}
          id="user-tab-0"
          aria-controls="user-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        {/* General Tab */}
        <TabPanel value={activeTab} index={0}>
          <Stack spacing={3}>
            <QuickCopySection user={user} copiedField={copiedField} onCopyToClipboard={handleCopyToClipboard} />

            {/* User Attributes */}
            <SettingsCard
              title={t('users:manageUser.sections.attributes.title', 'User Attributes')}
              description={t(
                'users:manageUser.sections.attributes.description',
                'View and manage user attribute values.',
              )}
              headerAction={
                !isEditMode && hasEditableFields ? (
                  <Button variant="outlined" size="small" onClick={() => setIsEditMode(true)}>
                    {t('common:actions.edit', 'Edit')}
                  </Button>
                ) : undefined
              }
            >
              {!isEditMode ? (
                <Stack spacing={2}>
                  {user.attributes && Object.keys(user.attributes).length > 0 ? (
                    Object.entries(user.attributes).map(([key, value]) => {
                      let displayValue: string;
                      if (value === null || value === undefined) {
                        displayValue = '-';
                      } else if (typeof value === 'boolean') {
                        displayValue = value ? t('common:actions.yes') : t('common:actions.no');
                      } else if (Array.isArray(value)) {
                        displayValue = value.join(', ');
                      } else if (typeof value === 'object') {
                        displayValue = JSON.stringify(value);
                      } else if (typeof value === 'string' || typeof value === 'number') {
                        displayValue = String(value);
                      } else {
                        displayValue = '-';
                      }

                      const fieldDef = userTypeDetails?.schema?.[key];
                      let attributeLabel = key;
                      if (fieldDef?.displayName) {
                        const resolved = resolveDisplayName(fieldDef.displayName);
                        attributeLabel = resolved || key;
                      }

                      return (
                        <Box key={key}>
                          <Typography variant="caption" color="text.secondary">
                            {attributeLabel}
                          </Typography>
                          <Typography variant="body1">{displayValue}</Typography>
                        </Box>
                      );
                    })
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      {t('users:manageUser.sections.attributes.empty', 'No attributes available')}
                    </Typography>
                  )}
                </Stack>
              ) : (
                <Box
                  component="form"
                  onSubmit={(event) => {
                    handleSubmit(onSubmit)(event).catch(() => null);
                  }}
                  noValidate
                  sx={{display: 'flex', flexDirection: 'column', gap: 2}}
                >
                  {userTypeDetails?.schema ? (
                    Object.entries(userTypeDetails.schema)
                      .filter(
                        ([, fieldDef]) =>
                          !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential),
                      )
                      .map(([fieldName, fieldDef]) =>
                        renderSchemaField(fieldName, fieldDef, control, errors, resolveDisplayName),
                      )
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      {t('users:manageUser.sections.attributes.noSchema', 'No schema available for editing')}
                    </Typography>
                  )}

                  {updateUserMutation.error && (
                    <Alert severity="error" sx={{mt: 2}}>
                      <Typography variant="body2" sx={{fontWeight: 'bold', mb: 0.5}}>
                        {updateUserMutation.error.message}
                      </Typography>
                    </Alert>
                  )}

                  <Stack direction="row" spacing={2} justifyContent="flex-end" sx={{mt: 2}}>
                    <Button
                      variant="outlined"
                      onClick={handleCancel}
                      disabled={isSubmitting}
                      startIcon={<X size={16} />}
                    >
                      {t('common:actions.cancel', 'Cancel')}
                    </Button>
                    <Button
                      type="submit"
                      variant="contained"
                      startIcon={isSubmitting ? null : <Save size={16} />}
                      disabled={isSubmitting}
                    >
                      {isSubmitting ? t('common:status.saving', 'Saving...') : t('common:actions.save', 'Save Changes')}
                    </Button>
                  </Stack>
                </Box>
              )}
            </SettingsCard>

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
          </Stack>
        </TabPanel>
      </>

      {/* Delete Dialog */}
      <UserDeleteDialog
        open={deleteDialogOpen}
        userId={userId ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />
    </PageContent>
  );
}
