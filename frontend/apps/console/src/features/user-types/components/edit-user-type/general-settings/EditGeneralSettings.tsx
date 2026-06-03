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

import {SettingsCard} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {useResolveDisplayName} from '@thunderid/hooks';
import {Stack, Typography, Button, Select, MenuItem} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState, useCallback, useRef, useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import QuickCopySection from './QuickCopySection';
import type {ApiUserType, SchemaPropertyInput} from '../../../types/user-types';

export interface EditGeneralSettingsProps {
  userType: ApiUserType;
  editedOuId: string | undefined;
  editedAllowSelfRegistration: boolean | undefined;
  editedDisplayAttribute: string | undefined;
  onFieldChange: (field: string, value: unknown) => void;
  onDeleteClick?: () => void;
  eligibleDisplayProperties: SchemaPropertyInput[];
}

/**
 * General settings tab content for the User Type edit page.
 * Displays Organization Unit, Self Registration, Display Attribute, and Danger Zone sections.
 */
export default function EditGeneralSettings({
  userType,
  editedOuId,
  editedAllowSelfRegistration,
  editedDisplayAttribute,
  onFieldChange,
  onDeleteClick = undefined,
  eligibleDisplayProperties,
}: EditGeneralSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    },
    [],
  );

  const handleCopyToClipboard = useCallback(async (text: string, fieldName: string): Promise<void> => {
    await navigator.clipboard.writeText(text);
    setCopiedField(fieldName);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => {
      setCopiedField(null);
    }, 2000);
  }, []);

  const effectiveOuId = editedOuId ?? userType.ouId;
  const effectiveAllowSelfRegistration = editedAllowSelfRegistration ?? userType.allowSelfRegistration;
  const effectiveDisplayAttribute = editedDisplayAttribute ?? userType.systemAttributes?.display ?? '';

  return (
    <Stack spacing={3}>
      <QuickCopySection userType={userType} copiedField={copiedField} onCopyToClipboard={handleCopyToClipboard} />

      {/* Organization Unit */}
      <SettingsCard
        title={t('userTypes:edit.general.organizationUnit.title', 'Organization Unit')}
        description={t(
          'userTypes:edit.general.organizationUnit.description',
          'The organization unit this user type belongs to.',
        )}
      >
        <OrganizationUnitTreePicker
          value={effectiveOuId}
          onChange={userType.isReadOnly ? () => undefined : (selectedOuId) => onFieldChange('ouId', selectedOuId)}
          maxHeight={400}
        />
      </SettingsCard>

      {/* Self Registration */}
      <SettingsCard
        title={t('userTypes:edit.general.selfRegistration.title', 'Self Registration')}
        description={t(
          'userTypes:edit.general.selfRegistration.description',
          'Allow users to self-register with this user type.',
        )}
        enabled={effectiveAllowSelfRegistration}
        onToggle={userType.isReadOnly ? undefined : (enabled) => onFieldChange('allowSelfRegistration', enabled)}
      >
        <Typography variant="body2" color="text.secondary">
          {t('userTypes:edit.general.selfRegistration.enabledHint', 'Users can register themselves as this user type.')}
        </Typography>
      </SettingsCard>

      {/* Display Attribute */}
      <SettingsCard
        title={t('userTypes:edit.general.displayAttribute.title', 'Display Attribute')}
        description={t(
          'userTypes:edit.general.displayAttribute.description',
          'The attribute used to display user identity.',
        )}
      >
        <Select
          value={effectiveDisplayAttribute}
          onChange={(event) => onFieldChange('displayAttribute', event.target.value)}
          disabled={userType.isReadOnly}
          size="small"
          fullWidth
          displayEmpty
          renderValue={(selected) => {
            const value = typeof selected === 'string' ? selected : '';
            if (!value) {
              return (
                <Typography variant="body2" color="text.secondary">
                  {t('userTypes:selectDisplayAttribute', 'Select a display attribute')}
                </Typography>
              );
            }
            const matchedProp = eligibleDisplayProperties.find((p) => p.name.trim() === value);
            const resolved = matchedProp?.displayName ? resolveDisplayName(matchedProp.displayName) : '';
            return resolved && resolved !== value ? `${resolved} (${value})` : value;
          }}
        >
          <MenuItem value="">
            <Typography variant="body2" color="text.secondary">
              {t('common:none', 'None')}
            </Typography>
          </MenuItem>
          {eligibleDisplayProperties.map((prop) => {
            const propName = prop.name.trim();
            const resolved = prop.displayName ? resolveDisplayName(prop.displayName) : '';
            return (
              <MenuItem key={prop.id} value={propName}>
                {resolved && resolved !== propName ? `${resolved} (${propName})` : propName}
              </MenuItem>
            );
          })}
        </Select>
      </SettingsCard>

      {/* Danger Zone */}
      {onDeleteClick && (
        <SettingsCard
          title={t('userTypes:edit.general.dangerZone.title', 'Danger Zone')}
          description={t('userTypes:edit.general.dangerZone.description', 'Irreversible actions for this user type.')}
        >
          <Typography variant="h6" gutterBottom color="error">
            {t('userTypes:edit.general.dangerZone.deleteUserType', 'Delete User Type')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {t(
              'userTypes:edit.general.dangerZone.deleteUserTypeDescription',
              'Permanently delete this user type and all associated schema definitions. This action cannot be undone.',
            )}
          </Typography>
          <Button variant="contained" color="error" onClick={onDeleteClick}>
            {t('common:actions.delete')}
          </Button>
        </SettingsCard>
      )}
    </Stack>
  );
}
