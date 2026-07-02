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

import {useResolveDisplayName} from '@thunderid/hooks';
import {FormControl, FormLabel, MenuItem, Paper, Select, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useCallback, useEffect, useMemo, useRef} from 'react';
import {useTranslation} from 'react-i18next';
import type {SchemaPropertyInput} from '../../types/user-types';
import SchemaPropertyEditor from '../shared/SchemaPropertyEditor';

/**
 * Props for the {@link ConfigureProperties} component.
 *
 * @public
 */
export interface ConfigurePropertiesProps {
  properties: SchemaPropertyInput[];
  onPropertiesChange: (properties: SchemaPropertyInput[]) => void;
  enumInput: Record<string, string>;
  onEnumInputChange: (enumInput: Record<string, string>) => void;
  displayAttribute: string;
  onDisplayAttributeChange: (displayAttribute: string) => void;
  onReadyChange?: (isReady: boolean) => void;
  userTypeName?: string;
}

/**
 * Step 3 of the user type creation wizard: configure schema properties.
 *
 * @public
 */
export default function ConfigureProperties({
  properties,
  onPropertiesChange,
  enumInput,
  onEnumInputChange,
  displayAttribute,
  onDisplayAttributeChange,
  onReadyChange = undefined,
  userTypeName = undefined,
}: ConfigurePropertiesProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  // Eligible properties for display attribute: string/number/enum type, non-credential, with a name
  const eligibleDisplayProperties = useMemo(
    () =>
      properties.filter(
        (p) =>
          (p.type === 'string' || p.type === 'number' || p.type === 'enum') &&
          !p.credential &&
          p.name.trim().length > 0,
      ),
    [properties],
  );

  // Track whether user has explicitly cleared the display attribute.
  const userClearedRef = useRef(false);

  const handleDisplayAttributeChange = useCallback(
    (value: string): void => {
      userClearedRef.current = !value;
      onDisplayAttributeChange(value);
    },
    [onDisplayAttributeChange],
  );

  // Auto-select when exactly one eligible property (unless user explicitly cleared), clear if selected becomes ineligible
  useEffect((): void => {
    const eligibleNames = eligibleDisplayProperties.map((p) => p.name.trim());
    if (eligibleNames.length === 1 && !displayAttribute && !userClearedRef.current) {
      onDisplayAttributeChange(eligibleNames[0]);
    } else if (displayAttribute && !eligibleNames.includes(displayAttribute)) {
      onDisplayAttributeChange('');
    }
  }, [eligibleDisplayProperties, displayAttribute, onDisplayAttributeChange]);

  // Broadcast readiness - ready when at least one property has a name
  useEffect((): void => {
    if (onReadyChange) {
      const hasValidProperty = properties.some((prop) => prop.name.trim().length > 0);
      onReadyChange(hasValidProperty);
    }
  }, [properties, onReadyChange]);

  return (
    <Stack direction="column" spacing={4} data-testid="configure-properties">
      <Stack direction="column" spacing={1}>
        <Typography variant="h1" gutterBottom>
          {t('userTypes:createWizard.properties.title')}
        </Typography>
        <Typography variant="subtitle1" color="text.secondary">
          {t('userTypes:createWizard.properties.subtitle')}
        </Typography>
      </Stack>

      <SchemaPropertyEditor
        properties={properties}
        onPropertiesChange={onPropertiesChange}
        enumInput={enumInput}
        onEnumInputChange={onEnumInputChange}
        userTypeName={userTypeName}
        footer={
          eligibleDisplayProperties.length > 0 ? (
            <Paper variant="outlined" sx={{px: 3, py: 3, borderRadius: 2}}>
              <FormControl fullWidth>
                <FormLabel>{t('userTypes:displayAttribute', 'Display Attribute')}</FormLabel>
                <Typography variant="body2" color="text.secondary" sx={{mb: 1}}>
                  {t(
                    'userTypes:displayAttributeHint',
                    'The property used to display user names in listings and references',
                  )}
                </Typography>
                <Select
                  value={displayAttribute}
                  onChange={(e) => handleDisplayAttributeChange(e.target.value)}
                  size="small"
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
                    const name = prop.name.trim();
                    const resolved = prop.displayName ? resolveDisplayName(prop.displayName) : '';
                    return (
                      <MenuItem key={prop.id} value={name}>
                        {resolved && resolved !== name ? `${resolved} (${name})` : name}
                      </MenuItem>
                    );
                  })}
                </Select>
              </FormControl>
            </Paper>
          ) : undefined
        }
      />
    </Stack>
  );
}
