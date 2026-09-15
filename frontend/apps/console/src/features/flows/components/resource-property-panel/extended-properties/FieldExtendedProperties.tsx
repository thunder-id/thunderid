// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetUserTypeAttributes, type AggregatedUserTypeAttribute} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  FormHelperText,
  FormLabel,
  Stack,
  TextField,
  Typography,
  type AutocompleteRenderInputParams,
} from '@wso2/oxygen-ui';
import {useMemo, useState, type HTMLAttributes, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from '@/features/flows/components/resource-property-panel/CommonResourceProperties';
import useResourceFieldError from '@/features/flows/hooks/useResourceFieldError';
import {ElementTypes, type Element} from '@/features/flows/models/elements';

/**
 * Props interface of {@link FieldExtendedProperties}
 */
export type FieldExtendedPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

/**
 * Read the attribute name off an option, which is a free-solo string when the author typed a value
 * that no user type declares.
 *
 * @param option - The Autocomplete option.
 * @returns The attribute name.
 */
function getAttributeName(option: string | AggregatedUserTypeAttribute): string {
  return typeof option === 'string' ? option : option.attribute;
}

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

  // The input text is tracked separately from the Autocomplete value. Feeding typed text back in as
  // the value makes MUI treat the input as a pristine selection and disable option filtering, which
  // would leave the suggestion list unfiltered while typing.
  // Local state also keeps the text stable on blur, since updates upstream are debounced.
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
      <Autocomplete
        freeSolo
        disablePortal
        key={resource.id}
        options={options}
        loading={isLoading}
        getOptionLabel={getAttributeName}
        renderOption={(
          props: HTMLAttributes<HTMLLIElement> & {key?: string},
          option: string | AggregatedUserTypeAttribute,
        ) => (
          <li {...props} key={getAttributeName(option)}>
            <Stack>
              <Typography variant="body2">{getAttributeName(option)}</Typography>
              {typeof option !== 'string' && (
                <Typography variant="caption" color="text.secondary">
                  {option.userTypes.join(', ')}
                </Typography>
              )}
            </Stack>
          </li>
        )}
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
        value={null}
        inputValue={inputValue}
        onChange={(_: SyntheticEvent, attribute: string | AggregatedUserTypeAttribute | null) => {
          const selected: string = attribute === null ? '' : getAttributeName(attribute);
          setInputValue(selected);
          onChange('ref', selected, resource);
        }}
        onInputChange={(_: SyntheticEvent, value: string, reason: string) => {
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
