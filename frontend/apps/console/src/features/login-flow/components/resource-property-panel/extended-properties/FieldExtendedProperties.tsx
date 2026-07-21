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

import {
  Autocomplete,
  FormHelperText,
  FormLabel,
  Stack,
  TextField,
  type AutocompleteRenderInputParams,
} from '@wso2/oxygen-ui';
import {useMemo, useState, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from '@/features/flows/components/resource-property-panel/ResourceProperties';
import useResourceFieldError from '@/features/flows/hooks/useResourceFieldError';
import {ElementTypes, type Element} from '@/features/flows/models/elements';

/**
 * Props interface of {@link FieldExtendedProperties}
 */
export type FieldExtendedPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

/**
 * Extended properties for the field elements.
 *
 * @param props - Props injected to the component.
 * @returns The FieldExtendedProperties component.
 */
function FieldExtendedProperties({resource, onChange}: FieldExtendedPropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const attributes: string[] = useMemo(() => ['email', 'username', 'given_name'], []);
  const credentialAttributes: string[] = useMemo(() => ['password', 'pin', 'secret'], []);

  const resourceRef = (resource as Element & {ref?: string})?.ref;

  // Use local state to track the selected value immediately (avoids revert on blur due to debounced updates)
  // Initialize with the resourceRef value directly (supports free-solo values not in the predefined list)
  const [localSelectedValue, setLocalSelectedValue] = useState<string | null>(() => resourceRef ?? null);

  // Sync local state when resource changes (e.g., when switching to a different element)
  const [prevResourceRef, setPrevResourceRef] = useState(resourceRef);
  if (resourceRef !== prevResourceRef) {
    setPrevResourceRef(resourceRef);
    setLocalSelectedValue(resourceRef ?? null);
  }

  /**
   * Get the error message for the ref field.
   */
  const errorMessage: string = useResourceFieldError(resource?.id, 'ref');

  return (
    <Stack>
      <Autocomplete
        freeSolo={resource.type !== ElementTypes.PasswordInput}
        disablePortal
        key={resource.id}
        options={(resource.type === ElementTypes.PasswordInput ? credentialAttributes : attributes) ?? []}
        getOptionLabel={(attribute: string) => attribute}
        sx={{width: '100%'}}
        renderInput={(params: AutocompleteRenderInputParams) => (
          <>
            <FormLabel htmlFor="attribute-select">{t('flows:core.fieldExtendedProperties.attribute')}</FormLabel>
            <TextField
              {...params}
              id="attribute-select"
              placeholder={t('flows:core.fieldExtendedProperties.selectAttribute')}
              error={!!errorMessage}
            />
          </>
        )}
        value={localSelectedValue}
        onChange={(_: SyntheticEvent, attribute: string | null) => {
          setLocalSelectedValue(attribute);
          onChange('ref', attribute ?? '', resource);
        }}
        onInputChange={(_: SyntheticEvent, value: string, reason: string) => {
          // Handle free-form input (when user types a custom value)
          if (reason === 'input') {
            setLocalSelectedValue(value);
            onChange('ref', value, resource, true);
          }
        }}
      />
      {errorMessage && <FormHelperText error>{errorMessage}</FormHelperText>}
    </Stack>
  );
}

export default FieldExtendedProperties;
