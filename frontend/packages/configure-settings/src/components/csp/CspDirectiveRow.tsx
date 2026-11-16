// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Autocomplete, Box, Chip, IconButton, MenuItem, Select, Stack, TextField, Tooltip} from '@wso2/oxygen-ui';
import {RotateCcw, Trash} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {parseSources} from '../../utils/cspDirectiveRows';
import {isInvalidCspSource} from '../../utils/validateCspDirectiveRows';

/** Keeps the action column the same width on every row. */
const ROW_ACTION_WIDTH = 40;

const DIRECTIVE_SELECT_WIDTH = 200;

/**
 * The action a row offers: removing a directive the admin added, or resetting an overridden one.
 *
 * @public
 */
export type CspDirectiveRowAction = 'remove' | 'reset';

/**
 * Props for {@link CspDirectiveRow}.
 *
 * @public
 */
export interface CspDirectiveRowProps {
  /** The directive name, for example `img-src`. */
  directive: string;

  /** The directive's sources, in order. */
  sources: string[];

  /** Directives the row may switch to. When omitted, the directive cannot be changed. */
  directiveOptions?: string[];

  /** Already-translated error message. */
  error?: string;

  /** The action offered in the trailing column, if any. */
  action?: CspDirectiveRowAction;

  /** Test id applied to the row, so a row's controls can be located together. */
  testId?: string;

  onDirectiveChange?: (directive: string) => void;
  onSourcesChange?: (sources: string[]) => void;
  onAction?: () => void;
}

/**
 * A single CSP directive row: the directive, its sources as chips, and a remove or reset action. A source
 * is added on Enter, on a space, or when the field loses focus.
 *
 * @public
 */
export default function CspDirectiveRow({
  directive,
  sources,
  directiveOptions = undefined,
  error = undefined,
  action = undefined,
  testId = undefined,
  onDirectiveChange = undefined,
  onSourcesChange = undefined,
  onAction = undefined,
}: CspDirectiveRowProps): JSX.Element {
  const {t} = useTranslation();
  const [inputValue, setInputValue] = useState('');
  const actionLabel =
    action === 'reset'
      ? t('settings:csp.resetDirective', 'Reset to default')
      : t('settings:csp.removeDirective', 'Remove directive');

  const commitSources = (next: string[]): void => {
    const unique = [...new Set(next.flatMap(parseSources))];
    if (unique.join(' ') !== sources.join(' ')) {
      onSourcesChange?.(unique);
    }
  };

  const handleInputChange = (next: string): void => {
    if (!/\s/.test(next)) {
      setInputValue(next);
      return;
    }
    // Whitespace ends a source, so a typed space or a pasted list becomes chips straight away.
    const typed = parseSources(next);
    const pending = /\s$/.test(next) ? '' : (typed.pop() ?? '');
    commitSources([...sources, ...typed]);
    setInputValue(pending);
  };

  const options = directiveOptions ? [...new Set([directive, ...directiveOptions])] : [directive];

  return (
    <Stack direction="row" spacing={1} alignItems="flex-start" data-componentid={testId}>
      <Select
        size="small"
        value={directive}
        disabled={!directiveOptions}
        onChange={(event) => onDirectiveChange?.(event.target.value)}
        inputProps={{'aria-label': t('settings:csp.directive.label', 'Directive')}}
        sx={{width: DIRECTIVE_SELECT_WIDTH, flex: 'none', fontFamily: 'monospace'}}
      >
        {options.map((name) => (
          <MenuItem key={name} value={name} sx={{fontFamily: 'monospace'}}>
            {name}
          </MenuItem>
        ))}
      </Select>

      <Autocomplete
        multiple
        freeSolo
        fullWidth
        size="small"
        options={[]}
        value={sources}
        inputValue={inputValue}
        onChange={(_event, next) => commitSources(next)}
        onInputChange={(_event, next, reason) => {
          if (reason === 'input') {
            handleInputChange(next);
          } else {
            setInputValue(next);
          }
        }}
        onBlur={() => {
          commitSources([...sources, inputValue]);
          setInputValue('');
        }}
        renderValue={(values, getItemProps) =>
          values.map((source, index) => {
            const {key, onDelete, ...itemProps} = getItemProps({index});
            const invalid = isInvalidCspSource(source);
            return (
              <Chip
                key={key}
                {...itemProps}
                onDelete={onDelete}
                label={source}
                size="small"
                color={invalid ? 'error' : 'default'}
                variant={invalid ? 'outlined' : 'filled'}
                sx={{fontFamily: 'monospace'}}
              />
            );
          })
        }
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={
              sources.length === 0 ? t('settings:csp.sourcesPlaceholder', 'Add a source and press Enter') : undefined
            }
            error={Boolean(error)}
            helperText={error}
            inputProps={{
              ...params.inputProps,
              'aria-label': t('settings:csp.sources.label', 'Sources for {{directive}}', {directive}),
            }}
          />
        )}
        sx={{flex: 1, minWidth: 0}}
      />

      {action ? (
        <Tooltip title={actionLabel}>
          <IconButton
            aria-label={actionLabel}
            color={action === 'remove' ? 'error' : 'default'}
            onClick={onAction}
            sx={{width: ROW_ACTION_WIDTH, flex: 'none', mt: 0.5}}
          >
            {action === 'remove' ? <Trash size={18} /> : <RotateCcw size={18} />}
          </IconButton>
        </Tooltip>
      ) : (
        <Box aria-hidden sx={{width: ROW_ACTION_WIDTH, flex: 'none'}} />
      )}
    </Stack>
  );
}
