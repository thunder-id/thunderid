// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import AccessSection from './AccessSection';
import type {Application} from '@thunderid/configure-applications';

/**
 * Props for the {@link EditAccessSettings} component.
 */
interface EditAccessSettingsProps {
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
   * Bumped by the parent on Save/Reset to force AccessSection to remount.
   */
  sectionResetKey?: number;
  /**
   * Callback function to handle validation changes
   * @param hasErrors - Boolean indicating if the access settings have validation errors
   */
  onValidationChange?: (hasErrors: boolean) => void;
  /**
   * Whether to show user-facing access config (allowed user types). Hidden for clients with no
   * user-facing grant.
   */
  showUserAccessConfig?: boolean;
}

/**
 * Container component for the Access tab of the application edit page.
 *
 * Displays sections for:
 * - Allowed user types
 * - Application access URL
 *
 * @param props - Component props
 * @returns Access settings sections
 */
export default function EditAccessSettings({
  application,
  editedApp,
  onFieldChange,
  sectionResetKey = 0,
  onValidationChange = undefined,
  showUserAccessConfig = true,
}: EditAccessSettingsProps): JSX.Element {
  return (
    <AccessSection
      key={sectionResetKey}
      application={application}
      editedApp={editedApp}
      onFieldChange={onFieldChange}
      onValidationChange={onValidationChange}
      showUserAccessConfig={showUserAccessConfig}
    />
  );
}
