// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useResolveDisplayName} from '@thunderid/hooks';
import {getContactTypesMissingRegex} from '@thunderid/utils';
import {FormControl, FormLabel, MenuItem, Paper, Select, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useCallback, useEffect, useMemo, useRef} from 'react';
import {useTranslation} from 'react-i18next';
import type {SchemaPropertyInput} from '../../models/property-definition';
import isDisplayableProperty from '../../utils/isDisplayableProperty';
import EditSchemaSettings from '../edit-agent-type/schema-settings/EditSchemaSettings';

/**
 * Props for the {@link ConfigureProperties} component.
 *
 * @public
 */
export interface ConfigurePropertiesProps {
  /**
   * The schema properties being defined.
   */
  properties: SchemaPropertyInput[];
  /**
   * Invoked when the schema properties change.
   */
  onPropertiesChange: (properties: SchemaPropertyInput[]) => void;
  /**
   * The property used to display agents of this type.
   */
  displayAttribute: string;
  /**
   * Invoked when the display attribute changes.
   */
  onDisplayAttributeChange: (displayAttribute: string) => void;
  /**
   * Invoked whenever the step becomes ready or stops being ready to submit.
   */
  onReadyChange?: (isReady: boolean) => void;
  /**
   * The agent type's handle, used to suggest translation keys for property display names.
   */
  agentTypeHandle?: string;
}

/**
 * The properties step of the agent type creation wizard: configure schema properties and the
 * display attribute.
 *
 * @public
 */
export default function ConfigureProperties({
  properties,
  onPropertiesChange,
  displayAttribute,
  onDisplayAttributeChange,
  onReadyChange = undefined,
  agentTypeHandle = '',
}: ConfigurePropertiesProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const eligibleDisplayProperties = useMemo(() => properties.filter(isDisplayableProperty), [properties]);

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
      onReadyChange(hasValidProperty && getContactTypesMissingRegex(properties).length === 0);
    }
  }, [properties, onReadyChange]);

  return (
    <Stack direction="column" spacing={4} data-testid="configure-properties">
      <Stack direction="column" spacing={1}>
        <Typography variant="h1" gutterBottom>
          {t('agentTypes:createWizard.properties.title', 'Define your schema properties')}
        </Typography>
        <Typography variant="subtitle1" color="text.secondary">
          {t('agentTypes:createWizard.properties.subtitle', 'Add the fields that make up this agent type')}
        </Typography>
      </Stack>

      <EditSchemaSettings
        properties={properties}
        onPropertiesChange={onPropertiesChange}
        agentTypeHandle={agentTypeHandle}
      />

      {eligibleDisplayProperties.length > 0 && (
        <Paper variant="outlined" sx={{px: 3, py: 3, borderRadius: 2}}>
          <FormControl fullWidth>
            <FormLabel id="agent-type-display-attribute-label">
              {t('agentTypes:displayAttribute', 'Display attribute')}
            </FormLabel>
            <Typography variant="body2" color="text.secondary" sx={{mb: 1}}>
              {t(
                'agentTypes:displayAttributeHint',
                'The property used to display agent names in listings and references',
              )}
            </Typography>
            <Select
              value={displayAttribute}
              onChange={(e) => handleDisplayAttributeChange(e.target.value)}
              size="small"
              displayEmpty
              labelId="agent-type-display-attribute-label"
              renderValue={(selected) => {
                const value = typeof selected === 'string' ? selected : '';
                if (!value) {
                  return (
                    <Typography variant="body2" color="text.secondary">
                      {t('agentTypes:selectDisplayAttribute', 'Select a display attribute')}
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
                const propertyName = prop.name.trim();
                const resolved = prop.displayName ? resolveDisplayName(prop.displayName) : '';
                return (
                  <MenuItem key={prop.id} value={propertyName}>
                    {resolved && resolved !== propertyName ? `${resolved} (${propertyName})` : propertyName}
                  </MenuItem>
                );
              })}
            </Select>
          </FormControl>
        </Paper>
      )}
    </Stack>
  );
}
