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

import {UnsavedChangesBar} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
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
import {useState, useCallback, useMemo, type SyntheticEvent, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import EditFlowsSettings from '../../applications/components/edit-application/flows-settings/EditFlowsSettings';
import EditTokenSettings from '../../applications/components/edit-application/token-settings/EditTokenSettings';
import type {Application} from '../../applications/models/application';
import useGetAgent from '../api/useGetAgent';
import useUpdateAgent from '../api/useUpdateAgent';
import EditAdvancedSettings from '../components/edit-agent/advanced-settings/EditAdvancedSettings';
import EditAgentAttributes from '../components/edit-agent/attributes/EditAgentAttributes';
import AllowedUserTypesSection from '../components/edit-agent/flows-settings/AllowedUserTypesSection';
import EditGeneralSettings from '../components/edit-agent/general-settings/EditGeneralSettings';
import type {Agent, OAuthAgentConfig} from '../models/agent';

interface TabPanelProps {
  children?: ReactNode;
  index: number;
  value: number;
}

function TabPanel({children = null, value, index, ...other}: TabPanelProps) {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`agent-tabpanel-${index}`}
      aria-labelledby={`agent-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

export default function AgentEditPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('AgentEditPage');
  const {agentId} = useParams<{agentId: string}>();

  const {data: agent, isLoading, error, isError, refetch} = useGetAgent(agentId ?? '');
  const updateAgent = useUpdateAgent();

  const [activeTab, setActiveTab] = useState(0);
  const [editedAgent, setEditedAgent] = useState<Partial<Agent>>({});
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempName, setTempName] = useState('');
  const [tempDescription, setTempDescription] = useState('');
  const [validationErrorSources, setValidationErrorSources] = useState<Record<string, boolean>>({});
  const handleValidationChange = useCallback(
    (source: string) =>
      (hasError: boolean): void => {
        setValidationErrorSources((prev) => {
          if (prev[source] === hasError) return prev;
          return {...prev, [source]: hasError};
        });
      },
    [],
  );
  const hasAnyValidationError = Object.values(validationErrorSources).some(Boolean);

  const handleBack = async () => {
    await navigate('/agents');
  };

  const handleTabChange = (_event: SyntheticEvent, newValue: number) => {
    setActiveTab(newValue);
  };

  const handleCopyToClipboard = useCallback(
    async (text: string, fieldName: string) => {
      try {
        await navigator.clipboard.writeText(text);
        setCopiedField(fieldName);
        setTimeout(() => setCopiedField(null), 2000);
      } catch {
        logger.error('Failed to copy to clipboard');
      }
    },
    [logger],
  );

  const handleFieldChange = useCallback((field: keyof Agent, value: unknown) => {
    setEditedAgent((prev) => ({...prev, [field]: value}));
  }, []);

  const handleSave = useCallback(async () => {
    if (!agent || !agentId) return;

    const updatedData = {...agent, ...editedAgent};

    try {
      await updateAgent.mutateAsync({agentId, data: updatedData});
      setEditedAgent({});
      await refetch();
    } catch {
      logger.error('Failed to update agent');
    }
  }, [agent, agentId, editedAgent, updateAgent, refetch, logger]);

  const hasChanges = useMemo(() => Object.keys(editedAgent).length > 0, [editedAgent]);

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px'}}>
        <CircularProgress />
      </Box>
    );
  }

  if (isError || error) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {error?.message ?? t('agents:edit.page.error', 'Failed to load agent')}
        </Alert>
        <Button onClick={() => void handleBack()} startIcon={<ArrowLeft size={16} />}>
          {t('agents:edit.page.back', 'Back to agents')}
        </Button>
      </PageContent>
    );
  }

  if (!agent) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('agents:edit.page.notFound', 'Agent not found')}
        </Alert>
        <Button onClick={() => void handleBack()} startIcon={<ArrowLeft size={16} />}>
          {t('agents:edit.page.back', 'Back to agents')}
        </Button>
      </PageContent>
    );
  }

  const oauth2Config: OAuthAgentConfig | undefined = (editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig)?.find(
    (config) => config.type === 'oauth2',
  )?.config;

  const hasOAuth = Boolean(oauth2Config);

  interface TabConfig {
    key: string;
    label: string;
    render: () => ReactNode;
  }

  const tabs: TabConfig[] = [
    {
      key: 'general',
      label: t('applications:edit.page.tabs.general', 'General'),
      render: () => (
        <EditGeneralSettings
          agent={agent}
          oauth2Config={oauth2Config}
          copiedField={copiedField}
          onCopyToClipboard={handleCopyToClipboard}
          onDeleteSuccess={() => {
            void handleBack();
          }}
        />
      ),
    },
    {
      key: 'attributes',
      label: t('agents:edit.page.tabs.attributes', 'Attributes'),
      render: () => (
        <EditAgentAttributes
          agent={agent}
          onSaved={() => {
            refetch().catch(() => null);
          }}
        />
      ),
    },
  ];

  if (hasOAuth) {
    // Reuse the application sections — agents share the same inbound-client shape (auth_flow_id,
    // assertion, login_consent, token config, scopes, etc.).
    const appLikeAgent = agent as unknown as Application;
    const appLikeEditedAgent = editedAgent as unknown as Partial<Application>;
    const appHandleFieldChange = handleFieldChange as unknown as (field: keyof Application, value: unknown) => void;

    tabs.push(
      {
        key: 'flows',
        label: t('applications:edit.page.tabs.flows', 'Flows'),
        render: () => (
          <Stack spacing={3}>
            <EditFlowsSettings
              application={appLikeAgent}
              editedApp={appLikeEditedAgent}
              onFieldChange={appHandleFieldChange}
              entityLabel="agent"
            />
            <AllowedUserTypesSection agent={agent} editedAgent={editedAgent} onFieldChange={handleFieldChange} />
          </Stack>
        ),
      },
      {
        key: 'token',
        label: t('applications:edit.page.tabs.token', 'Token'),
        render: () => (
          <EditTokenSettings
            application={appLikeAgent}
            oauth2Config={oauth2Config}
            onFieldChange={appHandleFieldChange}
            onValidationChange={handleValidationChange('token')}
            entityLabel="agent"
          />
        ),
      },
      {
        key: 'advanced',
        label: t('applications:edit.page.tabs.advanced', 'Advanced'),
        render: () => (
          <EditAdvancedSettings
            agent={agent}
            editedAgent={editedAgent}
            oauth2Config={oauth2Config}
            onFieldChange={handleFieldChange}
            onValidationChange={handleValidationChange('redirectUri')}
          />
        ),
      },
    );
  }

  const safeActiveTab = activeTab >= tabs.length ? 0 : activeTab;

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to="/agents" />}>
          {t('agents:edit.page.back', 'Back to agents')}
        </PageTitle.BackButton>
        <PageTitle.Avatar sx={{overflow: 'visible'}}>
          <Box
            sx={{
              width: 55,
              height: 55,
              borderRadius: '50%',
              bgcolor: 'primary.light',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '1.75rem',
            }}
          >
            🤖
          </Box>
        </PageTitle.Avatar>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  if (tempName.trim()) handleFieldChange('name', tempName.trim());
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    if (tempName.trim()) handleFieldChange('name', tempName.trim());
                    setIsEditingName(false);
                  } else if (e.key === 'Escape') {
                    setIsEditingName(false);
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{editedAgent.name ?? agent.name}</Typography>
                <IconButton
                  size="small"
                  onClick={() => {
                    setTempName(editedAgent.name ?? agent.name);
                    setIsEditingName(true);
                  }}
                  sx={{opacity: 0.6, '&:hover': {opacity: 1}}}
                >
                  <Edit size={16} />
                </IconButton>
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
                  const trimmed = tempDescription.trim();
                  const currentValue = editedAgent.description ?? agent.description ?? '';
                  if (trimmed !== currentValue) {
                    handleFieldChange('description', trimmed);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') setIsEditingDescription(false);
                }}
                size="small"
                placeholder={t('agents:edit.page.description.placeholder', 'Add a description')}
                sx={{maxWidth: '600px', '& .MuiInputBase-root': {fontSize: '0.875rem'}}}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {editedAgent.description ??
                    agent.description ??
                    t('agents:edit.page.description.empty', 'No description')}
                </Typography>
                <IconButton
                  size="small"
                  onClick={() => {
                    setTempDescription(editedAgent.description ?? agent.description ?? '');
                    setIsEditingDescription(true);
                  }}
                  sx={{opacity: 0.6, '&:hover': {opacity: 1}, mt: -0.5}}
                >
                  <Edit size={14} />
                </IconButton>
              </>
            )}
          </Stack>
        </PageTitle.SubHeader>
      </PageTitle>

      <Tabs value={safeActiveTab} onChange={handleTabChange} aria-label="agent settings tabs">
        {tabs.map((tab, idx) => (
          <Tab
            key={tab.key}
            label={tab.label}
            id={`agent-tab-${idx}`}
            aria-controls={`agent-tabpanel-${idx}`}
            sx={{textTransform: 'none'}}
          />
        ))}
      </Tabs>

      {tabs.map((tab, idx) => (
        <TabPanel key={tab.key} value={safeActiveTab} index={idx}>
          {tab.render()}
        </TabPanel>
      ))}

      {hasChanges && (
        <UnsavedChangesBar
          message={t('agents:edit.page.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('agents:edit.page.reset', 'Discard')}
          saveLabel={t('agents:edit.page.save', 'Save')}
          savingLabel={t('agents:edit.page.saving', 'Saving…')}
          isSaving={updateAgent.isPending}
          saveDisabled={hasAnyValidationError}
          onReset={() => setEditedAgent({})}
          onSave={() => {
            void handleSave();
          }}
        />
      )}
    </PageContent>
  );
}
