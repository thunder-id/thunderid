// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack} from '@wso2/oxygen-ui';
import AppearanceSection from './AppearanceSection';
import ContactsSection from './ContactsSection';
import UrlsSection from './UrlsSection';
import type {Application} from '@thunderid/configure-applications';

/**
 * Props for the {@link EditCustomizationSettings} component.
 */
interface EditCustomizationSettingsProps {
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
   * Bumped by the parent on Save/Reset to force UrlsSection to remount and drop its stale
   * react-hook-form defaults.
   */
  sectionResetKey?: number;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
  /**
   * Callback function to handle validation changes
   * @param hasErrors - Boolean indicating if the customization settings have validation errors
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Container component for application customization settings.
 *
 * Displays sections for:
 * - Appearance (theme/layout selection)
 * - URLs (Terms of Service, Privacy Policy)
 * - Contact information (email addresses)
 *
 * @param props - Component props
 * @returns Customization settings sections wrapped in a Stack
 */
export default function EditCustomizationSettings({
  application,
  editedApp,
  onFieldChange,
  sectionResetKey = 0,
  entityLabel = 'application',
  onValidationChange = undefined,
}: EditCustomizationSettingsProps) {
  return (
    <Stack spacing={3}>
      <AppearanceSection
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
      />
      <UrlsSection
        key={sectionResetKey}
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
        onValidationChange={onValidationChange}
      />
      <ContactsSection
        application={application}
        editedApp={editedApp}
        onFieldChange={onFieldChange}
        entityLabel={entityLabel}
      />
    </Stack>
  );
}
