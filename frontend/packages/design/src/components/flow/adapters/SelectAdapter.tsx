// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Box, Button, CircularProgress, FormControl, FormLabel, MenuItem, Select, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useEndOfListTrigger from './useEndOfListTrigger';
import type {FlowFieldProps} from '../../../models/flow';

export interface SelectAdapterOption {
  label: string;
  value: string;
  disabled?: boolean;
}

export interface SelectAdapterProps extends FlowFieldProps {
  /** Used in place of `component.options`. */
  options?: SelectAdapterOption[];
  /** Called when the end of the open list becomes visible. The end of the list is watched only when this is set. */
  onLoadMore?: () => void;
  /** Shows a loading row and holds back further `onLoadMore` calls. */
  loading?: boolean;
  /**
   * The last load failed. A load more button then ends the list, so the user can ask again without scrolling away
   * from the end; the end of the list asks only once per list length.
   */
  loadFailed?: boolean;
  /** Called each time the menu opens, so a wrapper can load the first page. */
  onOpen?: () => void;
  /** Label for the current value while that value is not among the loaded options. */
  selectedOption?: Pick<SelectAdapterOption, 'label' | 'value'>;
}

function getOptionValue(option: unknown): string {
  if (typeof option === 'string') return option;
  if (typeof option === 'object' && option !== null && 'value' in option) {
    const {value} = option as {value: unknown};
    if (typeof value === 'string') return value;
    return JSON.stringify(value ?? option);
  }
  return JSON.stringify(option);
}

function getOptionLabel(option: unknown): string {
  if (typeof option === 'string') return option;
  if (typeof option === 'object' && option !== null && 'label' in option) {
    const {label} = option as {label: unknown};
    if (typeof label === 'string') return label;
    return JSON.stringify(label ?? option);
  }
  return JSON.stringify(option);
}

export default function SelectAdapter({
  component,
  values,
  touched,
  fieldErrors,
  isLoading,
  resolve,
  onInputChange,
  options: loadedOptions = undefined,
  onLoadMore = undefined,
  loading = false,
  loadFailed = false,
  onOpen = undefined,
  selectedOption = undefined,
}: SelectAdapterProps): JSX.Element | null {
  const {t} = useTranslation();
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [menuPaper, setMenuPaper] = useState<HTMLElement | null>(null);
  const [lastOption, setLastOption] = useState<HTMLElement | null>(null);
  const {ref} = component;
  // Resolved as the text input resolves its hint, without a second pass through t().
  const hint = component.hint ? (resolve(component.hint) ?? component.hint) : undefined;
  const options = loadedOptions ?? component.options;
  const watchesEnd = onLoadMore !== undefined;

  useEndOfListTrigger({
    enabled: isMenuOpen && watchesEnd,
    itemCount: options?.length ?? 0,
    loading,
    onLoadMore,
    root: menuPaper,
    sentinel: lastOption,
  });

  if (!ref || typeof ref !== 'string' || !options) return null;

  // A select whose options come with the flow and are empty has nothing to offer. An optional one is not drawn. A
  // required one says so, since the step cannot go on without it. Loaded options are different: an empty list there
  // is a first page still on its way, or a real empty result.
  if (loadedOptions === undefined && options.length === 0) {
    if (component.required !== true) return null;

    return (
      <FormControl fullWidth required className={cn('Flow--select', 'FormControl--root')}>
        <FormLabel htmlFor={ref} className={cn('Label--root')}>
          {t(resolve(component.label)!)}
        </FormLabel>
        <Typography variant="caption" color="text.secondary">
          {t('common:messages.noOptions', 'No options available')}
        </Typography>
      </FormControl>
    );
  }

  const hasError = !!(touched?.[ref] && fieldErrors?.[ref]);
  const value = values[ref] ?? '';
  const isValueMissingFromOptions =
    loadedOptions !== undefined && value !== '' && !loadedOptions.some((option) => option.value === value);
  const missingValueLabel = selectedOption?.value === value ? selectedOption.label : value;
  const isEmpty = loadedOptions !== undefined && options.length === 0 && !loading && !watchesEnd;

  return (
    <FormControl fullWidth className={cn('Flow--select', 'FormControl--root')}>
      <FormLabel htmlFor={ref} className={cn('Label--root')}>
        {t(resolve(component.label)!)}
      </FormLabel>
      <Select
        displayEmpty
        size="small"
        className={cn('Select--root')}
        id={ref}
        name={ref}
        required={component.required}
        fullWidth
        disabled={isLoading}
        error={hasError}
        value={value}
        open={isMenuOpen}
        onOpen={() => {
          setIsMenuOpen(true);
          onOpen?.();
        }}
        onClose={() => setIsMenuOpen(false)}
        onChange={(e) => onInputChange(ref, e.target.value)}
        MenuProps={watchesEnd ? {slotProps: {paper: {ref: setMenuPaper}}} : undefined}
      >
        <MenuItem value="" disabled={component.required === true}>
          {t(resolve(component.placeholder) ?? 'Select an option')}
        </MenuItem>
        {isValueMissingFromOptions && (
          // Disabled as well as hidden, so arrow-key navigation skips it instead of stalling on a row it cannot focus.
          <MenuItem value={value} disabled sx={{display: 'none'}}>
            {missingValueLabel}
          </MenuItem>
        )}
        {options.map((option: unknown, index: number) => (
          <MenuItem
            key={getOptionValue(option)}
            ref={watchesEnd && index === options.length - 1 ? setLastOption : undefined}
            value={getOptionValue(option)}
            disabled={loadedOptions?.[index]?.disabled}
          >
            {getOptionLabel(option)}
          </MenuItem>
        ))}
        {loading && (
          <MenuItem disabled>
            <CircularProgress size={16} sx={{mr: 1}} />
            {t('common:status.loading', 'Loading...')}
          </MenuItem>
        )}
        {loadFailed && !loading && onLoadMore && (
          // Not an option: without a tabindex the select ignores clicks on this row, so pressing the button asks for
          // more without choosing anything or closing the list.
          <Box component="li" sx={{px: 2, py: 0.5}}>
            <Button size="small" onClick={onLoadMore}>
              {t('common:actions.loadMore', 'Load more')}
            </Button>
          </Box>
        )}
        {isEmpty && <MenuItem disabled>{t('common:messages.noResults', 'No results found')}</MenuItem>}
      </Select>
      {hasError && (
        <Typography variant="caption" color="error.main" sx={{mt: 0.5}}>
          {fieldErrors?.[ref]}
        </Typography>
      )}
      {hint && (
        <Typography variant="caption" color="text.secondary">
          {hint}
        </Typography>
      )}
    </FormControl>
  );
}
