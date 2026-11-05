// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import DraftTextField from './DraftTextField';
import KeyValueEditor from './KeyValueEditor';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {omitProperty} from './utils';
import useResourceFieldError from '../../../../hooks/useResourceFieldError';
import type {StepData} from '../../../../models/steps';

// The only attributes flow save accepts as the match.
const MATCH_ATTRIBUTE_USERNAME = 'username';
const MATCH_ATTRIBUTE_USER_ID = 'userID';

// The complete rows with a unique attribute, the first row of an attribute winning.
const toMapping = (rows: [string, string][]): Record<string, string> => {
  const mapping: Record<string, string> = {};
  rows.forEach(([attribute, key]) => {
    const name = attribute.trim();
    if (name && key.trim() && !(name in mapping)) {
      mapping[name] = key;
    }
  });
  return mapping;
};

function ExtensionAuthResolverProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const sourceKeyError: string = useResourceFieldError(resource?.id, 'data.properties.sourceKey');
  const storedMappingEntries = Object.entries(
    (properties['attributeMapping'] as Record<string, string> | undefined) ?? {},
  );
  const storedMappingKey = JSON.stringify(storedMappingEntries);
  // The rows are kept here because the stored object cannot hold blank or repeated keys while they are typed.
  const [attributeMappingEntries, setAttributeMappingEntries] = useState<[string, string][]>(storedMappingEntries);

  // Follow the stored mapping when it changes from outside, such as an undo, unless it is what the rows hold.
  const [syncedMappingKey, setSyncedMappingKey] = useState(storedMappingKey);
  if (syncedMappingKey !== storedMappingKey) {
    setSyncedMappingKey(storedMappingKey);
    if (JSON.stringify(Object.entries(toMapping(attributeMappingEntries))) !== storedMappingKey) {
      setAttributeMappingEntries(storedMappingEntries);
    }
  }

  const matchesByUserId = properties['matchAttribute'] === MATCH_ATTRIBUTE_USER_ID;
  const hasDuplicateAttribute =
    new Set(attributeMappingEntries.map(([attribute]) => attribute.trim()).filter(Boolean)).size <
    attributeMappingEntries.filter(([attribute]) => attribute.trim()).length;

  // An empty value falls back to the server's default, so it is removed rather than stored.
  const handleOptionalPropertyChange = (property: string, value: unknown): void => {
    const empty = value === '' || (typeof value === 'object' && value !== null && Object.keys(value).length === 0);
    if (!empty) {
      onChange(`data.properties.${property}`, value, resource);
      return;
    }
    onChange('data.properties', omitProperty(properties, property), resource);
  };

  const updateAttributeMapping = (updater: (prev: [string, string][]) => [string, string][]): void => {
    const rows = updater(attributeMappingEntries);
    setAttributeMappingEntries(rows);

    handleOptionalPropertyChange('attributeMapping', toMapping(rows));
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.extensionAuthResolver.description',
          'Signs in the local user that the extension before this step verified.',
        )}
      </Typography>

      <div>
        <FormLabel htmlFor="extension-auth-resolver-source-key" required>
          {t('flows:core.executions.extensionAuthResolver.sourceKey.label', 'Source key')}
        </FormLabel>
        <DraftTextField
          id="extension-auth-resolver-source-key"
          value={(properties['sourceKey'] as string | undefined) ?? ''}
          onCommit={(value) => onChange('data.properties.sourceKey', value.trim(), resource)}
          placeholder={t(
            'flows:core.executions.extensionAuthResolver.sourceKey.placeholder',
            'extension.pin_check.verifiedUserId',
          )}
          error={!!sourceKeyError}
          fullWidth
          size="small"
        />
        <FormHelperText error={!!sourceKeyError}>
          {sourceKeyError ||
            t(
              'flows:core.executions.extensionAuthResolver.sourceKey.hint',
              'The key the extension before this step stored the verified identifier under, such as extension.pin_check.verifiedUserId.',
            )}
        </FormHelperText>
      </div>

      <div>
        <FormLabel id="extension-auth-resolver-match-attribute-label" htmlFor="extension-auth-resolver-match-attribute">
          {t('flows:core.executions.extensionAuthResolver.matchAttribute.label', 'Match attribute')}
        </FormLabel>
        <Select
          id="extension-auth-resolver-match-attribute"
          labelId="extension-auth-resolver-match-attribute-label"
          value={(properties['matchAttribute'] as string | undefined) ?? MATCH_ATTRIBUTE_USERNAME}
          onChange={(e) => {
            if (e.target.value === MATCH_ATTRIBUTE_USER_ID) {
              // The server refuses the provisioning options with a user ID, so they are dropped, not kept hidden.
              setAttributeMappingEntries([]);
              onChange(
                'data.properties',
                {
                  ...omitProperty(omitProperty(properties, 'allowAuthenticationWithoutLocalUser'), 'attributeMapping'),
                  matchAttribute: MATCH_ATTRIBUTE_USER_ID,
                },
                resource,
              );
              return;
            }
            handleOptionalPropertyChange('matchAttribute', '');
          }}
          fullWidth
          size="small"
        >
          <MenuItem value={MATCH_ATTRIBUTE_USERNAME}>
            {t('flows:core.executions.extensionAuthResolver.matchAttribute.username', 'Username')}
          </MenuItem>
          <MenuItem value={MATCH_ATTRIBUTE_USER_ID}>
            {t('flows:core.executions.extensionAuthResolver.matchAttribute.userId', 'User ID')}
          </MenuItem>
        </Select>
        <FormHelperText>
          {t(
            'flows:core.executions.extensionAuthResolver.matchAttribute.hint',
            'The user attribute the identifier is matched against. Only the user ID and the username can be used.',
          )}
        </FormHelperText>
      </div>

      {matchesByUserId ? (
        <FormHelperText>
          {t(
            'flows:core.executions.extensionAuthResolver.matchAttribute.userIdNote',
            'Provisioning options are not available when matching by user ID, because a user ID cannot belong to a user who does not exist yet.',
          )}
        </FormHelperText>
      ) : (
        <>
          <CheckboxWithHint
            checked={!!properties['allowAuthenticationWithoutLocalUser']}
            onChange={(checked) => onChange('data.properties.allowAuthenticationWithoutLocalUser', checked, resource)}
            label={t(
              'flows:core.executions.extensionAuthResolver.allowAuthenticationWithoutLocalUser.label',
              'Allow authentication without a local user',
            )}
            hint={t(
              'flows:core.executions.extensionAuthResolver.allowAuthenticationWithoutLocalUser.hint',
              'When no local user matches, continue so that a Provision User step can create the user.',
            )}
          />

          <div>
            <FormLabel>
              {t('flows:core.executions.extensionAuthResolver.attributeMapping.label', 'Attribute mapping')}
            </FormLabel>
            <KeyValueEditor
              entries={attributeMappingEntries}
              onAdd={() => updateAttributeMapping((prev) => [...prev, ['', '']])}
              onRemove={(index) => updateAttributeMapping((prev) => prev.filter((_, i) => i !== index))}
              onKeyChange={(index, newKey) =>
                updateAttributeMapping((prev) => prev.map((entry, i) => (i === index ? [newKey, entry[1]] : entry)))
              }
              onValueChange={(index, newValue) =>
                updateAttributeMapping((prev) => prev.map((entry, i) => (i === index ? [entry[0], newValue] : entry)))
              }
              keyLabel={t('flows:core.executions.extensionAuthResolver.attributeMapping.keyLabel', 'User attribute')}
              valueLabel={t('flows:core.executions.extensionAuthResolver.attributeMapping.valueLabel', 'Source key')}
              keyPlaceholder={t('flows:core.executions.extensionAuthResolver.attributeMapping.keyPlaceholder', 'email')}
              valuePlaceholder={t(
                'flows:core.executions.extensionAuthResolver.attributeMapping.valuePlaceholder',
                'extension.pin_check.email',
              )}
              addLabel={t('flows:core.executions.extensionAuthResolver.attributeMapping.addLabel', 'Add attribute')}
              removeLabel={t('common:actions.delete', 'Delete')}
            />
            {hasDuplicateAttribute && (
              <FormHelperText error>
                {t(
                  'flows:core.executions.extensionAuthResolver.attributeMapping.duplicate',
                  'Each user attribute can be mapped once. Only the first row for an attribute is kept.',
                )}
              </FormHelperText>
            )}
            <FormHelperText>
              {t(
                'flows:core.executions.extensionAuthResolver.attributeMapping.hint',
                'Attributes a Provision User step takes from the extension for a new user. Only these are taken, and never a credential.',
              )}
            </FormHelperText>
          </div>
        </>
      )}
    </Stack>
  );
}

export default ExtensionAuthResolverProperties;
