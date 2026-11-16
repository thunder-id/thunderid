// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetUserTypeAttributes, type AggregatedUserTypeAttribute} from '@thunderid/configure-user-types';
import {FormHelperText, Stack} from '@wso2/oxygen-ui';
import {useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import UserAttributeAutocomplete from './UserAttributeAutocomplete';
import useResourceFieldError from '../../../hooks/useResourceFieldError';
import {ElementTypes, type Element} from '../../../models/elements';
import type {CommonResourcePropertiesPropsInterface} from '../CommonResourceProperties';

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

  // The user type applicable at runtime is not known while the flow is being authored, so
  // attributes from every user type are offered as suggestions and custom values stay allowed.
  const {attributes: allAttributes, isLoading} = useGetUserTypeAttributes();

  const isCredentialField: boolean = resource.type === ElementTypes.PasswordInput;

  const options: AggregatedUserTypeAttribute[] = useMemo(
    () => allAttributes.filter((attribute) => attribute.credential === isCredentialField),
    [allAttributes, isCredentialField],
  );

  const resourceRef = (resource as Element & {ref?: string})?.ref;

  // Local state keeps the text stable on blur, since updates upstream are debounced.
  const [inputValue, setInputValue] = useState<string>(() => resourceRef ?? '');

  // Sync local state when resource changes (e.g., when switching to a different element)
  const [prevResourceRef, setPrevResourceRef] = useState(resourceRef);
  if (resourceRef !== prevResourceRef) {
    setPrevResourceRef(resourceRef);
    setInputValue(resourceRef ?? '');
  }

  /**
   * Get the error message for the ref field.
   */
  const errorMessage: string = useResourceFieldError(resource?.id, 'ref');

  return (
    <Stack>
      <UserAttributeAutocomplete
        key={resource.id}
        id="attribute-select"
        label={t('flows:core.fieldExtendedProperties.attribute')}
        placeholder={t('flows:core.fieldExtendedProperties.selectAttribute')}
        error={!!errorMessage}
        options={options}
        loading={isLoading}
        sx={{width: '100%'}}
        inputValue={inputValue}
        onChange={(attribute: string) => {
          setInputValue(attribute);
          onChange('ref', attribute, resource);
        }}
        onInputChange={(value: string, reason: string) => {
          // Handle free-form input (when user types a custom value)
          if (reason === 'input') {
            setInputValue(value);
            onChange('ref', value, resource, true);
          } else if (reason === 'clear') {
            // The Autocomplete value is always null, so clearing does not reach onChange.
            setInputValue('');
            onChange('ref', '', resource);
          }
        }}
      />
      {errorMessage && <FormHelperText error>{errorMessage}</FormHelperText>}
    </Stack>
  );
}

export default FieldExtendedProperties;
