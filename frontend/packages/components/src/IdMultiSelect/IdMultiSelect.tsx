// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Autocomplete, Chip, TextField, type AutocompleteRenderInputParams} from '@wso2/oxygen-ui';
import {useMemo, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import QueryErrorNotice from '../QueryErrorNotice/QueryErrorNotice';

/**
 * A resource the picker offers: stored by its ID, shown by its name.
 */
export interface IdMultiSelectOption {
  id: string;
  name: string;
}

/**
 * Props interface of {@link IdMultiSelect}
 */
export interface IdMultiSelectProps {
  /**
   * The error of the read that loads the options, shown with a retry in place of a silent empty list.
   */
  error?: Error | null;

  /**
   * The i18n key and default text of the message shown when the options fail to load.
   */
  errorFallback: {key: string; defaultValue: string};

  /**
   * The id of the input, which a label can point at.
   */
  id: string;

  /**
   * Whether the options are still loading.
   */
  loading: boolean;

  /**
   * The text shown when no option matches what was typed.
   */
  noOptionsText: string;

  /**
   * Called with the selected IDs when the selection changes.
   */
  onChange: (ids: string[]) => void;

  /**
   * Retries loading the options.
   */
  onRetry: () => void;

  /**
   * The resources that can be selected.
   */
  options: IdMultiSelectOption[];

  /**
   * The text shown while nothing is selected.
   */
  placeholder: string;

  /**
   * The label of a selected ID that matches none of the loaded options, such as a resource deleted since it was
   * selected.
   */
  unavailableLabel: (id: string) => string;

  /**
   * The selected IDs.
   */
  value: string[];
}

/**
 * A searchable multi-select of resources by name, for a value that stores their IDs. An ID that matches none of the
 * options once they have loaded is kept as a chip marked unavailable, so it can be seen and removed rather than
 * silently dropped.
 *
 * @param props - Props injected to the component.
 * @returns The IdMultiSelect component.
 */
export default function IdMultiSelect({
  error = null,
  errorFallback,
  id,
  loading,
  noOptionsText,
  onChange,
  onRetry,
  options,
  placeholder,
  unavailableLabel,
  value,
}: IdMultiSelectProps): JSX.Element {
  const {t} = useTranslation('common');

  // Only once the options have loaded can an ID that matches none of them be told apart as deleted.
  const optionsLoaded = !loading && !error;

  // A selected ID that matches no option keeps a chip of its own, or the next change would drop it unseen.
  const selected: IdMultiSelectOption[] = useMemo(
    () =>
      value.map(
        (selectedId: string) =>
          options.find((option) => option.id === selectedId) ?? {id: selectedId, name: selectedId},
      ),
    [value, options],
  );

  return (
    <>
      <Autocomplete
        multiple
        filterSelectedOptions
        fullWidth
        size="small"
        id={id}
        options={options}
        value={selected}
        loading={loading}
        loadingText={t('status.loading', 'Loading...')}
        noOptionsText={noOptionsText}
        getOptionLabel={(option: IdMultiSelectOption) => option.name}
        isOptionEqualToValue={(option: IdMultiSelectOption, current: IdMultiSelectOption) => option.id === current.id}
        onChange={(_event: SyntheticEvent, next: IdMultiSelectOption[]) => onChange(next.map((option) => option.id))}
        renderTags={(tags: IdMultiSelectOption[], getTagProps) =>
          tags.map((tag, index) => {
            const unavailable = optionsLoaded && !options.some((option) => option.id === tag.id);
            const {key, ...tagProps} = getTagProps({index});

            return (
              <Chip
                key={key}
                {...tagProps}
                size="small"
                color={unavailable ? 'error' : 'default'}
                label={unavailable ? unavailableLabel(tag.id) : tag.name}
              />
            );
          })
        }
        renderInput={(params: AutocompleteRenderInputParams) => (
          <TextField {...params} placeholder={selected.length === 0 ? placeholder : undefined} />
        )}
      />
      {error && (
        <QueryErrorNotice
          error={error}
          t={t}
          variant="inline"
          fallbackKey={errorFallback.key}
          fallbackDefaultValue={errorFallback.defaultValue}
          onRetry={onRetry}
        />
      )}
    </>
  );
}
