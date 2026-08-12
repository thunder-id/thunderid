// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation, QueryErrorNotice, UnsavedChangesBar} from '@thunderid/components';
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
import useGetGroup from '../api/useGetGroup';
import useUpdateGroup from '../api/useUpdateGroup';
import EditAdvancedSettings from '../components/edit-group/advanced-settings/EditAdvancedSettings';
import EditGeneralSettings from '../components/edit-group/general-settings/EditGeneralSettings';
import EditMembersSettings from '../components/edit-group/members-settings/EditMembersSettings';
import GroupDeleteDialog from '../components/GroupDeleteDialog';
import GroupConstraints from '../constants/group-constraints';
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
  const {t} = useTranslation('groups');
  const logger = useLogger('GroupEditPage');

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

  const handleFieldChange = useCallback(
    (field: keyof Group, value: unknown): void => {
      updateGroup.reset(); // a save error is stale once the form changes
      setEditedGroup((prev) => ({...prev, [field]: value}));
    },
    [updateGroup],
  );

  const commitName = useCallback(
    (value: string, currentName: string): void => {
      const trimmedName = value.trim();
      // The API rejects names outside these bounds, so an out of range rename is discarded here.
      if (
        trimmedName === currentName ||
        trimmedName.length < GroupConstraints.NAME_MIN_LENGTH ||
        trimmedName.length > GroupConstraints.NAME_MAX_LENGTH
      ) {
        return;
      }
      handleFieldChange('name', trimmedName);
    },
    [handleFieldChange],
  );

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
    }
  }, [group, groupId, editedGroup, updateGroup, refetch, logger]);

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
        <QueryErrorNotice
          error={fetchError}
          t={t}
          variant="block"
          title={t('edit.page.error', 'Failed to load group')}
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
              {t('edit.page.back', 'Back to Groups')}
            </Button>
          }
        />
      </PageContent>
    );
  }

  if (!group) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('edit.page.notFound', 'Group not found')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch((error: unknown) => {
              logger.error('Failed to navigate back', {error});
            });
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('edit.page.back', 'Back to Groups')}
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
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('edit.page.back', 'Back to Groups')}
        </PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  commitName(tempName, (editedGroup.name ?? group.name).trim());
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    commitName(tempName, (editedGroup.name ?? group.name).trim());
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
                placeholder={t('edit.page.description.placeholder', 'Add a description...')}
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
                  {effectiveDescription || t('edit.page.description.empty', 'No description')}
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
          label={t('edit.page.tabs.general', 'General')}
          id="group-tab-0"
          aria-controls="group-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('edit.page.tabs.members', 'Members')}
          id="group-tab-1"
          aria-controls="group-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('edit.page.tabs.advanced', 'Advanced')}
          id="group-tab-2"
          aria-controls="group-tabpanel-2"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <>
        <TabPanel value={activeTab} index={0}>
          <EditGeneralSettings group={group} />
        </TabPanel>

        <TabPanel value={activeTab} index={1}>
          <EditMembersSettings group={group} />
        </TabPanel>

        <TabPanel value={activeTab} index={2}>
          <EditAdvancedSettings onDeleteClick={group.isReadOnly ? undefined : () => setDeleteDialogOpen(true)} />
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
        <UnsavedChangesBar
          message={t('edit.page.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('edit.page.reset', 'Reset')}
          saveLabel={t('edit.page.save', 'Save Changes')}
          savingLabel={t('edit.page.saving', 'Saving...')}
          isSaving={updateGroup.isPending}
          saveDisabled={group.isReadOnly === true}
          error={
            updateGroup.error
              ? getErrorMessage(updateGroup.error, t, 'update.error', 'Failed to update group. Please try again.')
              : undefined
          }
          onReset={() => {
            updateGroup.reset();
            setEditedGroup({});
          }}
          onSave={() => {
            handleSave().catch(() => null);
          }}
        />
      )}
    </PageContent>
  );
}
