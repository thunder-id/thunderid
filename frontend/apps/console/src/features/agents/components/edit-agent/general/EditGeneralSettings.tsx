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
import {useState, type JSX} from 'react';
import OwnerSummarySection from './OwnerSummarySection';
import type {Agent} from '../../../models/agent';
import AgentDeleteDialog from '../../AgentDeleteDialog';
import AttributesSummarySection from '../attributes/AttributesSummarySection';
import DangerZoneSection from '../general-settings/DangerZoneSection';
import OrganizationUnitSection from '../general-settings/OrganizationUnitSection';
import QuickCopySection from '../general-settings/QuickCopySection';

interface EditGeneralSettingsProps {
  agent: Agent;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
  onDeleteSuccess?: () => void;
}

export default function EditGeneralSettings({
  agent,
  copiedField,
  onCopyToClipboard,
  onDeleteSuccess = undefined,
}: EditGeneralSettingsProps): JSX.Element {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  return (
    <>
      <Stack spacing={3}>
        <QuickCopySection agent={agent} copiedField={copiedField} onCopyToClipboard={onCopyToClipboard} />
        <OwnerSummarySection agent={agent} />
        <AttributesSummarySection agent={agent} />
        <OrganizationUnitSection agent={agent} copiedField={copiedField} onCopyToClipboard={onCopyToClipboard} />
        {!agent.isReadOnly && <DangerZoneSection onDeleteClick={() => setDeleteDialogOpen(true)} />}
      </Stack>

      <AgentDeleteDialog
        open={deleteDialogOpen}
        agentId={agent.id}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={onDeleteSuccess}
      />
    </>
  );
}
