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

import {Stack} from '@wso2/oxygen-ui';
import AuthenticationFlowSection from './AuthenticationFlowSection';
import RecoveryFlowSection from './RecoveryFlowSection';
import RegistrationFlowSection from './RegistrationFlowSection';
import type {Application} from '../../../models/application';

/**
 * Props for the {@link EditFlowsSettings} component.
 */
interface EditFlowsSettingsProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Partial application object containing edited fields
   */
  editedApp: Partial<Application>;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
}

/**
 * Container component for authentication and registration flow settings.
 *
 * Displays sections for:
 * - Authentication flow selection
 * - Registration flow selection (with enable/disable toggle)
 *
 * @param props - Component props
 * @returns Flow settings sections wrapped in a Stack
 */
export default function EditFlowsSettings({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: EditFlowsSettingsProps) {
  return (
    <Stack spacing={3}>
      <AuthenticationFlowSection
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
      />
      <RegistrationFlowSection
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
      />
      <RecoveryFlowSection
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
      />
    </Stack>
  );
}
