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

import {PageLoadingAnimation, ResourceAvatar, UnsavedChangesBar} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Button,
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
import RouteConfig from '../../../configs/RouteConfig';
import useGetAgent from '../api/useGetAgent';
import useUpdateAgent from '../api/useUpdateAgent';
import EditAccessSettings from '../components/edit-agent/access/EditAccessSettings';
import EditAdvancedSettings from '../components/edit-agent/advanced-settings/EditAdvancedSettings';
import EditAgentAttributes from '../components/edit-agent/attributes/EditAgentAttributes';
import EditCredentialsSettings from '../components/edit-agent/credentials/EditCredentialsSettings';
import EditFlowsSettings from '../components/edit-agent/flows/EditFlowsSettings';
import EditGeneralSettings from '../components/edit-agent/general/EditGeneralSettings';
import EditTokensSettings from '../components/edit-agent/tokens/EditTokensSettings';
import AgentConstants from '../constants/agent-constants';
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
  // Bumped on Save/Reset to force EditAgentAttributes to remount with a clean form — it keeps
  // its own react-hook-form state locally, which a `setEditedAgent({})` alone wouldn't reset.
  const [attributesResetKey, setAttributesResetKey] = useState(0);
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
  const hasAnyOtherValidationError = Object.values(validationErrorSources).some(Boolean);

  const handleBack = async () => {
    await navigate(RouteConfig.agents.list());
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

    const {certificate, ...updatedData} = {...agent, ...editedAgent} as Agent & {certificate?: unknown};
    void certificate;

    try {
      await updateAgent.mutateAsync({agentId, data: updatedData});
      setEditedAgent({});
      setAttributesResetKey((key) => key + 1);
      await refetch();
    } catch {
      logger.error('Failed to update agent');
    }
  }, [agent, agentId, editedAgent, updateAgent, refetch, logger]);

  const hasChanges = useMemo(() => Object.keys(editedAgent).length > 0, [editedAgent]);

  if (isLoading) {
    return <PageLoadingAnimation />;
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

  // Computed directly from state rather than reported by the Advanced/Access tab content, since
  // that content unmounts when its tab isn't active — a callback-based report would be stale or
  // never fire if the user never visits the tab before saving.
  const hasAuthorizationCodeGrant = oauth2Config?.grantTypes?.includes('authorization_code') ?? false;
  const hasValidRedirectUri = (oauth2Config?.redirectUris ?? []).some((uri) => {
    if (!uri.trim()) return false;
    try {
      return Boolean(new URL(uri));
    } catch {
      return false;
    }
  });
  const isMissingRedirectUri = hasAuthorizationCodeGrant && !hasValidRedirectUri;
  const allowedUserTypes = editedAgent.allowedUserTypes ?? agent.allowedUserTypes ?? [];
  const isMissingAllowedUserType = hasAuthorizationCodeGrant && allowedUserTypes.length === 0;
  const isMissingCertificate =
    oauth2Config?.tokenEndpointAuthMethod === 'private_key_jwt' && !oauth2Config?.certificate?.value;
  const hasAnyValidationError =
    hasAnyOtherValidationError || isMissingRedirectUri || isMissingAllowedUserType || isMissingCertificate;

  // List every failing check by name rather than a single generic message, so the user knows
  // exactly what to fix instead of guessing which tab has the problem.
  const validationIssues: string[] = [];
  if (isMissingRedirectUri) {
    validationIssues.push(t('agents:edit.page.validation.missingRedirectUri', 'add a redirect URI'));
  }
  if (isMissingAllowedUserType) {
    validationIssues.push(
      t('agents:edit.page.validation.missingAllowedUserType', 'select at least one allowed user type'),
    );
  }
  if (isMissingCertificate) {
    validationIssues.push(t('agents:edit.page.validation.missingCertificate', 'add a certificate'));
  }
  if (hasAnyOtherValidationError) {
    validationIssues.push(t('agents:edit.page.validation.tokenSettings', 'fix the token settings'));
  }

  const formatIssueList = (issues: string[]): string => {
    if (issues.length <= 1) return issues[0] ?? '';
    if (issues.length === 2) return `${issues[0]} and ${issues[1]}`;
    return `${issues.slice(0, -1).join(', ')}, and ${issues[issues.length - 1]}`;
  };

  const unsavedChangesMessage =
    validationIssues.length > 0
      ? t('agents:edit.page.unsavedChangesInvalid', 'Before saving, {{issues}}.', {
          issues: formatIssueList(validationIssues),
        })
      : t('agents:edit.page.unsavedChanges', 'You have unsaved changes');

  interface TabConfig {
    key: string;
    label: string;
    render: () => ReactNode;
  }

  const tabs: TabConfig[] = [
    {
      key: 'general',
      label: t('agents:edit.page.tabs.general', 'General'),
      render: () => (
        <EditGeneralSettings
          agent={agent}
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
          key={attributesResetKey}
          agent={agent}
          editedAgent={editedAgent}
          onFieldChange={handleFieldChange}
        />
      ),
    },
  ];

  if (hasOAuth) {
    tabs.push({
      key: 'credentials',
      label: t('agents:edit.page.tabs.credentials', 'Credentials'),
      render: () => (
        <EditCredentialsSettings
          agent={agent}
          editedAgent={editedAgent}
          oauth2Config={oauth2Config}
          copiedField={copiedField}
          onCopyToClipboard={handleCopyToClipboard}
          onFieldChange={handleFieldChange}
        />
      ),
    });
  }

  tabs.push({
    key: 'access',
    label: t('agents:edit.page.tabs.access', 'Access'),
    render: () => <EditAccessSettings agent={agent} />,
  });

  if (hasOAuth) {
    tabs.push({
      key: 'flows',
      label: t('agents:edit.page.tabs.flows', 'Flows'),
      render: () => (
        <EditFlowsSettings
          agent={agent}
          editedAgent={editedAgent}
          oauth2Config={oauth2Config}
          onFieldChange={handleFieldChange}
        />
      ),
    });

    tabs.push({
      key: 'tokens',
      label: t('agents:edit.page.tabs.tokens', 'Tokens'),
      render: () => (
        <EditTokensSettings
          agent={agent}
          editedAgent={editedAgent}
          oauth2Config={oauth2Config}
          onFieldChange={handleFieldChange}
          onValidationChange={handleValidationChange('token')}
        />
      ),
    });

    tabs.push({
      key: 'advanced',
      label: t('agents:edit.page.tabs.advanced', 'Advanced'),
      render: () => (
        <EditAdvancedSettings
          agent={agent}
          editedAgent={editedAgent}
          oauth2Config={oauth2Config}
          onFieldChange={handleFieldChange}
        />
      ),
    });
  }

  const safeActiveTab = activeTab >= tabs.length ? 0 : activeTab;

  return (
    <PageContent>
      {agent.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={RouteConfig.agents.list()} />}>
          {t('agents:edit.page.back', 'Back to agents')}
        </PageTitle.BackButton>
        <PageTitle.Avatar sx={{overflow: 'visible'}}>
          <ResourceAvatar size={55} fallback={AgentConstants.DEFAULT_AVATAR} />
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
                {!agent.isReadOnly && (
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
                {!agent.isReadOnly && (
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
                )}
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
            sx={{textTransform: 'none', minHeight: 48}}
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
          message={unsavedChangesMessage}
          resetLabel={t('agents:edit.page.reset', 'Reset')}
          saveLabel={t('agents:edit.page.save', 'Save')}
          savingLabel={t('agents:edit.page.saving', 'Saving…')}
          isSaving={updateAgent.isPending}
          saveDisabled={hasAnyValidationError || agent.isReadOnly === true}
          onReset={() => {
            setEditedAgent({});
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
