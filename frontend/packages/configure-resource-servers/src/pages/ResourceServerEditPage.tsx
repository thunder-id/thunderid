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

import {PageLoadingAnimation, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Button,
  Chip,
  IconButton,
  PageContent,
  PageTitle,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams, useSearchParams} from 'react-router';
import useGetResourceServer from '../api/useGetResourceServer';
import useUpdateResourceServer from '../api/useUpdateResourceServer';
import AdvancedTab from '../components/resource-server-detail/AdvancedTab';
import ResourceTree from '../components/resource-tree/ResourceTree';
import ResourceServerDeleteDialog from '../components/ResourceServerDeleteDialog';
import {getResourceServerTypeIcon, getResourceServerTypeLabel} from '../config/resource-server-types';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel({children = undefined, value, index}: TabPanelProps): JSX.Element {
  return (
    <Box
      role="tabpanel"
      hidden={value !== index}
      sx={{pt: 3, height: value === index ? 'auto' : 0, overflow: 'hidden'}}
    >
      {value === index && children}
    </Box>
  );
}

const TAB_RESOURCES = 0;
const TAB_ADVANCED = 1;

export default function ResourceServerEditPage(): JSX.Element {
  const {resourceServerId} = useParams<{resourceServerId: string}>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('ResourceServerEditPage');

  const {data: resourceServer, isLoading, error, refetch} = useGetResourceServer(resourceServerId ?? '');
  const updateRs = useUpdateResourceServer();

  const initialTab = searchParams.get('tab') === 'advanced' ? TAB_ADVANCED : TAB_RESOURCES;
  const [activeTab, setActiveTab] = useState(initialTab);

  const [editedFields, setEditedFields] = useState<Partial<{name: string; description: string; identifier: string}>>(
    {},
  );
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempName, setTempName] = useState('');
  const [tempDescription, setTempDescription] = useState('');
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleTabChange = (_e: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
  };

  const handleFieldChange = (field: 'name' | 'description' | 'identifier', value: string): void => {
    if (!resourceServer) return;
    const original =
      field === 'name'
        ? resourceServer.name
        : field === 'description'
          ? (resourceServer.description ?? '')
          : (resourceServer.identifier ?? '');
    if (value === original) {
      setEditedFields((prev) => {
        const next = {...prev};
        delete next[field];
        return next;
      });
    } else {
      setEditedFields((prev) => ({...prev, [field]: value}));
    }
  };

  const hasChanges = Object.keys(editedFields).length > 0;

  const handleSave = (): void => {
    if (!resourceServer) return;

    const nextIdentifier =
      'identifier' in editedFields ? (editedFields.identifier ?? '').trim() : (resourceServer.identifier ?? '').trim();
    if (!nextIdentifier) {
      showToast(t('resourceServers:edit.identifierRequired', 'Identifier is required.'), 'error');
      return;
    }

    updateRs.mutate(
      {
        id: resourceServer.id,
        data: {
          name: editedFields.name ?? resourceServer.name,
          description:
            'description' in editedFields
              ? editedFields.description?.trim()
                ? editedFields.description
                : null
              : (resourceServer.description ?? null),
          identifier: 'identifier' in editedFields ? nextIdentifier : resourceServer.identifier,
          ouId: resourceServer.ouId,
        },
      },
      {
        onSuccess: () => {
          setEditedFields({});
          void refetch();
        },
        onError: (err: Error) => {
          logger.error('Failed to update resource server', {error: err});
          showToast(t('resourceServers:edit.saveError', 'Failed to save changes.'), 'error');
        },
      },
    );
  };

  const listUrl = '/resource-servers';

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (error || !resourceServer) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {error?.message ?? t('resourceServers:edit.notFound', 'Resource server not found.')}
        </Alert>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() => {
            (async (): Promise<void> => {
              await navigate(listUrl);
            })().catch((err: unknown) => {
              logger.error('Failed to navigate back', {error: err});
            });
          }}
        >
          {t('resourceServers:edit.back', 'Back to resource servers')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {resourceServer.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}

      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('resourceServers:edit.back', 'Back to resource servers')}
        </PageTitle.BackButton>
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
                    setIsEditingName(false);
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{editedFields.name ?? resourceServer.name}</Typography>
                {!resourceServer.isReadOnly && (
                  <IconButton
                    size="small"
                    onClick={() => {
                      setTempName(editedFields.name ?? resourceServer.name);
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
                  const currentValue = editedFields.description ?? resourceServer.description ?? '';
                  if (trimmedDescription !== currentValue) {
                    handleFieldChange('description', trimmedDescription);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmedDescription = tempDescription.trim();
                    const currentValue = editedFields.description ?? resourceServer.description ?? '';
                    if (trimmedDescription !== currentValue) {
                      handleFieldChange('description', trimmedDescription);
                    }
                    setIsEditingDescription(false);
                  } else if (e.key === 'Escape') {
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('resourceServers:edit.descriptionPlaceholder', 'Add a description')}
                sx={{maxWidth: '600px', '& .MuiInputBase-root': {fontSize: '0.875rem'}}}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {editedFields.description ??
                    resourceServer.description ??
                    t('resourceServers:edit.noDescription', 'No description')}
                </Typography>
                {!resourceServer.isReadOnly && (
                  <IconButton
                    size="small"
                    onClick={() => {
                      setTempDescription(editedFields.description ?? resourceServer.description ?? '');
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
          <Box sx={{mt: 1, display: 'flex', gap: 1, alignItems: 'center'}}>
            <Chip
              label={getResourceServerTypeLabel(resourceServer.type, t)}
              size="small"
              variant="outlined"
              icon={
                <Box sx={{display: 'flex', alignItems: 'center', '& > *': {width: 16, height: 16}}}>
                  {getResourceServerTypeIcon(resourceServer.type)}
                </Box>
              }
            />
            {resourceServer.isReadOnly && (
              <Chip label={t('resourceServers:edit.systemResourceServer', 'System')} size="small" color="default" />
            )}
          </Box>
        </PageTitle.SubHeader>
      </PageTitle>

      <Tabs
        value={activeTab}
        onChange={handleTabChange}
        aria-label={t('resourceServers:edit.tabs', 'Resource server settings')}
      >
        <Tab
          label={
            resourceServer.type === 'MCP'
              ? t('resourceServers:edit.tab.capabilities', 'Capabilities')
              : t('resourceServers:edit.tab.resources', 'Resources')
          }
          id="resource-server-tab-0"
          aria-controls="resource-server-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('resourceServers:edit.tab.advanced', 'Advanced Settings')}
          id="resource-server-tab-1"
          aria-controls="resource-server-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      <TabPanel value={activeTab} index={TAB_RESOURCES}>
        <Box sx={{height: 'calc(100vh - 540px)', minHeight: 300}}>
          <ResourceTree
            resourceServer={resourceServer}
            onRefresh={() => {
              void refetch();
            }}
          />
        </Box>

        {!resourceServer.isReadOnly && (
          <SettingsCard
            title={t('resourceServers:edit.dangerZone.title', 'Danger Zone')}
            description={
              resourceServer.type === 'MCP'
                ? t('resourceServers:edit.dangerZone.descriptionMcp', 'Irreversible actions for this MCP server.')
                : t('resourceServers:edit.dangerZone.description', 'Irreversible actions for this resource server.')
            }
            slotProps={{root: {sx: {mt: 3}}}}
          >
            <Typography variant="h6" gutterBottom color="error">
              {resourceServer.type === 'MCP'
                ? t('resourceServers:edit.dangerZone.deleteServer.titleMcp', 'Delete MCP server')
                : t('resourceServers:edit.dangerZone.deleteServer.title', 'Delete resource server')}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
              {resourceServer.type === 'MCP'
                ? t(
                    'resourceServers:edit.dangerZone.deleteServer.descriptionMcp',
                    'Permanently delete this MCP server and all associated data. This action cannot be undone.',
                  )
                : t(
                    'resourceServers:edit.dangerZone.deleteServer.description',
                    'Permanently delete this resource server and all associated data. This action cannot be undone.',
                  )}
            </Typography>
            <Button variant="contained" color="error" onClick={() => setDeleteDialogOpen(true)}>
              {resourceServer.type === 'MCP'
                ? t('resourceServers:edit.dangerZone.deleteServerMcp', 'Delete MCP server')
                : t('resourceServers:edit.dangerZone.deleteServer', 'Delete resource server')}
            </Button>
          </SettingsCard>
        )}

        <ResourceServerDeleteDialog
          open={deleteDialogOpen}
          resourceServer={resourceServer}
          onClose={() => setDeleteDialogOpen(false)}
          onSuccess={() => {
            (async (): Promise<void> => {
              await navigate(listUrl);
            })().catch((err: unknown) => {
              logger.error('Failed to navigate after delete', {error: err});
            });
          }}
        />
      </TabPanel>

      <TabPanel value={activeTab} index={TAB_ADVANCED}>
        <AdvancedTab
          key={resourceServer.id}
          resourceServer={resourceServer}
          identifier={editedFields.identifier ?? resourceServer.identifier ?? ''}
          onIdentifierChange={(v) => handleFieldChange('identifier', v)}
        />
      </TabPanel>

      {hasChanges && (
        <UnsavedChangesBar
          message={t('resourceServers:edit.unsavedChanges', 'You have unsaved changes.')}
          resetLabel={t('common:discard', 'Discard')}
          saveLabel={t('common:save', 'Save')}
          savingLabel={t('common:saving', 'Saving…')}
          isSaving={updateRs.isPending}
          saveDisabled={resourceServer.isReadOnly}
          onReset={() => setEditedFields({})}
          onSave={handleSave}
        />
      )}
    </PageContent>
  );
}
