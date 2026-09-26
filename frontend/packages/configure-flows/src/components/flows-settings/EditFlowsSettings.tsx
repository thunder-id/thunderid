// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Application} from '@thunderid/configure-applications';
import {Stack} from '@wso2/oxygen-ui';
import AuthenticationFlowSection from './AuthenticationFlowSection';
import RecoveryFlowSection from './RecoveryFlowSection';
import RegistrationFlowSection from './RegistrationFlowSection';
import SignOutFlowSection from './SignOutFlowSection';

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
      <SignOutFlowSection
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
      />
    </Stack>
  );
}
