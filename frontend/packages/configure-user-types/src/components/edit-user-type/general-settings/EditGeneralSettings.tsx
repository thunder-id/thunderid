// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {useResolveDisplayName} from '@thunderid/hooks';
import {Stack, Typography, Select, MenuItem, Button} from '@wso2/oxygen-ui';
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
  eligibleDisplayProperties: SchemaPropertyInput[];
  scimCoreUserType: boolean;
  onRequestScimCoreChange: (next: boolean) => void;
}

/**
 * General settings tab content for the User Type edit page.
 * Displays Organization Unit, Self Registration, and Display Attribute sections.
 */
export default function EditGeneralSettings({
  userType,
  editedOuId,
  editedAllowSelfRegistration,
  editedDisplayAttribute,
  onFieldChange,
  eligibleDisplayProperties,
  scimCoreUserType,
  onRequestScimCoreChange,
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

      {/* SCIM Core User Type */}
      <SettingsCard
        title={t('userTypes:edit.general.scimCore.title', 'SCIM Core User Type')}
        description={
          scimCoreUserType
            ? t(
                'userTypes:edit.general.scimCore.activeDescription',
                'This is the SCIM core user type. Its schema attributes can be mapped to SCIM fields in the SCIM Mapping tab.',
              )
            : t('userTypes:edit.general.scimCore.inactiveDescription', 'Not currently the SCIM core user type.')
        }
        headerAction={
          <Button
            variant={scimCoreUserType ? 'outlined' : 'contained'}
            size="small"
            disabled={userType.isReadOnly}
            onClick={() => onRequestScimCoreChange(!scimCoreUserType)}
          >
            {scimCoreUserType
              ? t('userTypes:edit.general.scimCore.removeButton', 'Remove as SCIM Core Type')
              : t('userTypes:edit.general.scimCore.setButton', 'Set as SCIM Core Type')}
          </Button>
        }
      >
        <Typography variant="body2" color="text.secondary">
          {t(
            'userTypes:edit.general.scimCore.hint',
            'Only one user type across the system can be the SCIM core type at a time.',
          )}
        </Typography>
      </SettingsCard>
    </Stack>
  );
}
