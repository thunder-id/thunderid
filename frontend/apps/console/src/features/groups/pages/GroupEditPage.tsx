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

import {PageLoadingAnimation} from '@thunderid/components';
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
import useGetGroup from '../api/useGetGroup';
import useUpdateGroup from '../api/useUpdateGroup';
import EditGeneralSettings from '../components/edit-group/general-settings/EditGeneralSettings';
import EditMembersSettings from '../components/edit-group/members-settings/EditMembersSettings';
import GroupDeleteDialog from '../components/GroupDeleteDialog';
import type {Group} from '../models/group';

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
      id={`group-tabpanel-${index}`}
      aria-labelledby={`group-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

export default function GroupEditPage(): JSX.Element {
  const {groupId} = useParams<{groupId: string}>();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('GroupEditPage');
  const {showToast} = useToast();

  const {data: group, isLoading, error: fetchError, refetch} = useGetGroup(groupId ?? '');
  const updateGroup = useUpdateGroup();

  const [activeTab, setActiveTab] = useState(0);
  const [editedGroup, setEditedGroup] = useState<Partial<Group>>({});
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempName, setTempName] = useState('');
  const [tempDescription, setTempDescription] = useState('');
  const listUrl = '/groups';

  const handleBack = async (): Promise<void> => {
    await navigate(listUrl);
  };

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
  };

  const handleFieldChange = useCallback((field: keyof Group, value: unknown): void => {
    setEditedGroup((prev) => ({...prev, [field]: value}));
  }, []);

  const handleSave = useCallback(async (): Promise<void> => {
    if (!group || !groupId) return;

    const updatedData = {
      name: editedGroup.name ?? group.name,
      description: 'description' in editedGroup ? editedGroup.description : group.description,
      ouId: group.ouId,
    };

    try {
      await updateGroup.mutateAsync({
        groupId,
        data: updatedData,
      });
      setEditedGroup({});
      await refetch();
    } catch (err: unknown) {
      logger.error('Failed to update group', {error: err});
      const message = err instanceof Error ? err.message : t('groups:edit.page.saveError');
      showToast(message, 'error');
    }
  }, [group, groupId, editedGroup, updateGroup, refetch, logger, showToast, t]);

  const hasChanges = useMemo(
    () => Object.entries(editedGroup).some(([key, value]) => !isEqualIgnoringEmpty(value, group?.[key as keyof Group])),
    [editedGroup, group],
  );

  // Resolve the effective description accounting for user edits (including clearing).
  // 'description' in editedGroup means the user has touched the field; otherwise fall back to server value.
  const effectiveDescription =
    'description' in editedGroup ? (editedGroup.description ?? '') : (group?.description ?? '');

  const handleDeleteSuccess = (): void => {
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate after deleting group', {error: _error});
    });
  };

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (fetchError) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {fetchError.message ?? t('groups:edit.page.error')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('groups:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  if (!group) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('groups:edit.page.notFound')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('groups:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {group.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>{t('groups:edit.page.back')}</PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  const trimmedName = tempName.trim();
                  const currentName = (editedGroup.name ?? group.name).trim();
                  if (trimmedName && trimmedName !== currentName) {
                    handleFieldChange('name', trimmedName);
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    const trimmedName = tempName.trim();
                    const currentName = (editedGroup.name ?? group.name).trim();
                    if (trimmedName && trimmedName !== currentName) {
                      handleFieldChange('name', trimmedName);
                    }
                    setIsEditingName(false);
                  } else if (e.key === 'Escape') {
                    setTempName(editedGroup.name ?? group.name);
                    setIsEditingName(false);
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{editedGroup.name ?? group.name}</Typography>
                {!group.isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label="Edit group name"
                    onClick={() => {
                      setTempName(editedGroup.name ?? group.name);
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
                  if (trimmedDescription !== effectiveDescription) {
                    handleFieldChange('description', trimmedDescription || undefined);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmedDescription = tempDescription.trim();
                    if (trimmedDescription !== effectiveDescription) {
                      handleFieldChange('description', trimmedDescription || undefined);
                    }
                    setIsEditingDescription(false);
                  } else if (e.key === 'Escape') {
                    setTempDescription(effectiveDescription);
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('groups:edit.page.description.placeholder')}
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
                  {effectiveDescription || t('groups:edit.page.description.empty')}
                </Typography>
                {!group.isReadOnly && (
                  <IconButton
                    size="small"
                    aria-label="Edit group description"
                    onClick={() => {
                      setTempDescription(effectiveDescription);
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
      <Tabs value={activeTab} onChange={handleTabChange} aria-label="group settings tabs">
        <Tab
          label={t('groups:edit.page.tabs.general')}
          id="group-tab-0"
          aria-controls="group-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('groups:edit.page.tabs.members')}
          id="group-tab-1"
          aria-controls="group-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        <TabPanel value={activeTab} index={0}>
          <EditGeneralSettings
            group={group}
            onDeleteClick={group.isReadOnly ? undefined : () => setDeleteDialogOpen(true)}
          />
        </TabPanel>

        <TabPanel value={activeTab} index={1}>
          <EditMembersSettings group={group} />
        </TabPanel>
      </>

      {/* Delete Dialog */}
      <GroupDeleteDialog
        open={deleteDialogOpen}
        groupId={groupId ?? null}
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
              {t('groups:edit.page.unsavedChanges')}
            </Typography>
            <Button variant="outlined" color="error" onClick={() => setEditedGroup({})}>
              {t('groups:edit.page.reset')}
            </Button>
            <Button
              variant="contained"
              onClick={() => {
                handleSave().catch(() => null);
              }}
              disabled={updateGroup.isPending || group.isReadOnly === true}
            >
              {updateGroup.isPending ? t('groups:edit.page.saving') : t('groups:edit.page.save')}
            </Button>
          </Stack>
        </Paper>
      )}
    </PageContent>
  );
}
