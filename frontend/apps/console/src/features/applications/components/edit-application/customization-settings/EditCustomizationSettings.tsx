/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import AppearanceSection from './AppearanceSection';
import ContactsSection from './ContactsSection';
import UrlsSection from './UrlsSection';
import type {Application} from '../../../models/application';

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
