// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AggregatedUserTypeAttribute} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  FormLabel,
  Stack,
  TextField,
  Typography,
  type AutocompleteRenderInputParams,
  type SxProps,
  type Theme,
} from '@wso2/oxygen-ui';
import type {HTMLAttributes, ReactElement, ReactNode, SyntheticEvent} from 'react';

/**
 * Props interface of {@link UserAttributeAutocomplete}
 */
export interface UserAttributeAutocompletePropsInterface {
  /**
   * The input id, which the label points at.
   */
  id: string;
  /**
   * The field label.
   */
  label: ReactNode;
  /**
   * The input placeholder.
   */
  placeholder?: string;
  /**
   * The input size.
   */
  size?: 'small' | 'medium';
  /**
   * Whether the input shows an error.
   */
  error?: boolean;
  /**
   * The user type attributes suggested.
   */
  options: AggregatedUserTypeAttribute[];
  /**
   * Whether the suggestions are still loading.
   */
  loading?: boolean;
  /**
   * Whether the clear button is hidden.
   */
  disableClearable?: boolean;
  /**
   * The text in the input.
   */
  inputValue: string;
  /**
   * Called with the attribute name when a suggestion is picked or a typed value is submitted.
   */
  onChange: (attribute: string) => void;
  /**
   * Called when the text in the input changes, with the reason MUI gives, such as `input` or `clear`.
   */
  onInputChange: (value: string, reason: string) => void;
  /**
   * Called when the input loses focus.
   */
  onBlur?: () => void;
  /**
   * Styles for the Autocomplete root.
   */
  sx?: SxProps<Theme>;
}

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
 * Free-solo picker for a user attribute, suggesting the attributes the user types declare with the user
 * types that declare each one.
 *
 * The input text is controlled apart from the Autocomplete value, which stays empty. Feeding typed text
 * back in as the value makes MUI treat the input as a pristine selection and disable option filtering.
 *
 * @param props - Props injected to the component.
 * @returns The UserAttributeAutocomplete component.
 */
function UserAttributeAutocomplete({
  id,
  label,
  placeholder = undefined,
  size = undefined,
  error = false,
  options,
  loading = false,
  disableClearable = false,
  inputValue,
  onChange,
  onInputChange,
  onBlur = undefined,
  sx = undefined,
}: UserAttributeAutocompletePropsInterface): ReactElement {
  return (
    <Autocomplete
      id={id}
      freeSolo
      disablePortal
      disableClearable={disableClearable}
      options={options}
      loading={loading}
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
      sx={sx}
      renderInput={(params: AutocompleteRenderInputParams) => (
        <>
          <FormLabel htmlFor={id}>{label}</FormLabel>
          <TextField {...params} size={size} placeholder={placeholder} error={error} />
        </>
      )}
      value={null}
      inputValue={inputValue}
      // The value stays null and MUI skips onChange when it does not change, so a clear never arrives here as null;
      // it arrives as an input change.
      onChange={(_: SyntheticEvent, option: string | AggregatedUserTypeAttribute | null) =>
        onChange(getAttributeName(option!))
      }
      onInputChange={(_: SyntheticEvent, value: string, reason: string) => onInputChange(value, reason)}
      onBlur={onBlur}
    />
  );
}

export default UserAttributeAutocomplete;
