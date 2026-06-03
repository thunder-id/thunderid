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

import {Stack} from '@wso2/oxygen-ui';
import {useState, useCallback} from 'react';
import type {JSX} from 'react';
import DangerZoneSection from './DangerZoneSection';
import OrganizationUnitSection from './OrganizationUnitSection';
import QuickCopySection from './QuickCopySection';
import {TokenEndpointAuthMethods} from '../../../../applications/models/oauth';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';
import AgentDeleteDialog from '../../AgentDeleteDialog';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import RegenerateSecretDialog from '../../RegenerateSecretDialog';

interface EditGeneralSettingsProps {
  agent: Agent;
  oauth2Config?: OAuthAgentConfig;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
  onDeleteSuccess?: () => void;
}

export default function EditGeneralSettings({
  agent,
  oauth2Config = undefined,
  copiedField,
  onCopyToClipboard,
  onDeleteSuccess = undefined,
}: EditGeneralSettingsProps): JSX.Element {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState<string>('');

  const isConfidentialClient =
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_BASIC ||
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_POST;

  const handleRegenerateSuccess = useCallback((clientSecret: string): void => {
    setNewClientSecret(clientSecret);
    setSecretDialogOpen(true);
  }, []);

  const handleSecretDialogClose = useCallback((): void => {
    setSecretDialogOpen(false);
    setNewClientSecret('');
  }, []);

  return (
    <>
      <Stack spacing={3}>
        <QuickCopySection
          agent={agent}
          oauth2Config={oauth2Config}
          copiedField={copiedField}
          onCopyToClipboard={onCopyToClipboard}
        />
        <OrganizationUnitSection agent={agent} copiedField={copiedField} onCopyToClipboard={onCopyToClipboard} />
        {!agent.isReadOnly && (
          <DangerZoneSection
            showRegenerateSecret={isConfidentialClient}
            onRegenerateClick={() => setRegenerateDialogOpen(true)}
            onDeleteClick={() => setDeleteDialogOpen(true)}
          />
        )}
      </Stack>

      <RegenerateSecretDialog
        open={regenerateDialogOpen}
        agentId={agent.id}
        onClose={() => setRegenerateDialogOpen(false)}
        onSuccess={handleRegenerateSuccess}
      />

      <ClientSecretSuccessDialog
        open={secretDialogOpen}
        clientSecret={newClientSecret}
        onClose={handleSecretDialogClose}
      />

      <AgentDeleteDialog
        open={deleteDialogOpen}
        agentId={agent.id}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={onDeleteSuccess}
      />
    </>
  );
}
