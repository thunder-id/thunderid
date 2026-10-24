// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import {PlusIcon, Trash} from '@wso2/oxygen-ui-icons-react';
import {memo, useCallback, useMemo, useReducer, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {INPUT_TYPES, SENSITIVE_INPUT_TYPES} from './constants';
import DraftTextField from './DraftTextField';
import type {FlowNodeInput} from '../../../../models/responses';
import {BOOLEAN_INPUT_TYPE, hasInvalidInputConfig, toBooleanDefault} from '../../../../utils/executorInputs';
import generateResourceId from '../../../../utils/generateResourceId';
import LabelWithTooltip from '../../LabelWithTooltip';
import PanelActionButton from '../../PanelActionButton';

/**
 * Props interface of {@link ExecutorInputsEditor}
 */
export interface ExecutorInputsEditorProps {
  inputs: FlowNodeInput[];
  onChange: (inputs: FlowNodeInput[]) => void;
  /**
   * Validation message for the inputs, shown under the title.
   */
  errorMessage?: string;
}

let nextId = 0;
const generateId = (): string => {
  nextId += 1;
  return `ei-${nextId}`;
};

/**
 * Reducer that maintains a stable list of IDs matched to entries length.
 */
const idsReducer = (prev: string[], requiredLength: number): string[] => {
  if (prev.length === requiredLength) {
    return prev;
  }

  if (requiredLength > prev.length) {
    const newIds = Array.from({length: requiredLength - prev.length}, () => generateId());
    return [...prev, ...newIds];
  }

  return prev.slice(0, requiredLength);
};

/**
 * Drops the default and constant properties when they carry no value, when the input type is
 * sensitive and cannot hold them, or when a boolean input's default is not a boolean, so a saved
 * input lists only what is set.
 */
const normalizeInput = (input: FlowNodeInput): FlowNodeInput => {
  const {default: defaultValue, constant, ...rest} = input;

  if (SENSITIVE_INPUT_TYPES.has(input.type)) {
    return rest;
  }

  const keepDefault =
    Boolean(defaultValue) && (input.type !== BOOLEAN_INPUT_TYPE || toBooleanDefault(defaultValue) !== '');

  return {...rest, ...(keepDefault ? {default: defaultValue} : {}), ...(constant ? {constant} : {})};
};

interface InputRowProps {
  input: FlowNodeInput;
  index: number;
  onUpdate: (index: number, field: keyof FlowNodeInput, value: string | boolean) => void;
  onRemove: (index: number) => void;
}

/**
 * A single input card with type dropdown, identifier field, default value, required and constant
 * checkboxes, and a remove action.
 * Commits identifier to the parent only on blur to avoid input clobbering during fast typing.
 */
const InputRow = memo(function InputRow({input, index, onUpdate, onRemove}: InputRowProps): ReactNode {
  const {t} = useTranslation();
  const [localIdentifier, setLocalIdentifier] = useState(input.identifier);

  // Reset local state when the prop changes from outside (e.g. parent replaces the list).
  // useMemo runs synchronously during render, avoiding the cascading-render lint error
  // that useEffect + setState would cause.
  const [prevIdentifier, setPrevIdentifier] = useState(input.identifier);
  if (input.identifier !== prevIdentifier) {
    setPrevIdentifier(input.identifier);
    setLocalIdentifier(input.identifier);
  }

  const canHoldDefault = !SENSITIVE_INPUT_TYPES.has(input.type);
  const invalid = hasInvalidInputConfig(input);

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        p: 1.5,
      }}
    >
      <Stack gap={1.5}>
        <div>
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <FormLabel>{t('flows:core.executions.inputs.typeLabel')}</FormLabel>
            <Tooltip title={t('flows:core.executions.inputs.remove')}>
              <IconButton
                size="small"
                onClick={() => onRemove(index)}
                aria-label={t('flows:core.executions.inputs.remove')}
              >
                <Trash size={14} color="red" />
              </IconButton>
            </Tooltip>
          </Stack>
          <Select
            value={input.type}
            onChange={(e) => onUpdate(index, 'type', e.target.value)}
            size="small"
            displayEmpty
            fullWidth
          >
            <MenuItem value="" disabled>
              {t('flows:core.executions.inputs.typePlaceholder')}
            </MenuItem>
            {INPUT_TYPES.map((inputType) => (
              <MenuItem key={inputType.value} value={inputType.value}>
                {t(inputType.translationKey)}
              </MenuItem>
            ))}
          </Select>
        </div>
        <div>
          <FormLabel>{t('flows:core.executions.inputs.identifierLabel')}</FormLabel>
          <TextField
            value={localIdentifier}
            onChange={(e) => setLocalIdentifier(e.target.value)}
            onBlur={() => {
              if (localIdentifier !== input.identifier) {
                onUpdate(index, 'identifier', localIdentifier);
              }
            }}
            placeholder={t('flows:core.executions.inputs.identifierPlaceholder')}
            size="small"
            fullWidth
          />
        </div>
        {canHoldDefault && (
          <div>
            <FormLabel>
              <LabelWithTooltip
                label={t('flows:core.executions.inputs.defaultLabel')}
                tooltip={t('flows:core.executions.inputs.defaultTooltip')}
              />
            </FormLabel>
            {input.type === BOOLEAN_INPUT_TYPE ? (
              <Select
                value={toBooleanDefault(input.default)}
                onChange={(e) => onUpdate(index, 'default', e.target.value)}
                error={invalid}
                size="small"
                displayEmpty
                fullWidth
              >
                <MenuItem value="">{t('flows:core.executions.inputs.defaultNotSet')}</MenuItem>
                <MenuItem value="true">{t('flows:core.executions.inputs.defaultTrue')}</MenuItem>
                <MenuItem value="false">{t('flows:core.executions.inputs.defaultFalse')}</MenuItem>
              </Select>
            ) : (
              <DraftTextField
                value={input.default ?? ''}
                onCommit={(value) => onUpdate(index, 'default', value)}
                placeholder={t('flows:core.executions.inputs.defaultPlaceholder')}
                error={invalid}
                size="small"
                fullWidth
              />
            )}
          </div>
        )}
        <Stack>
          <FormControlLabel
            control={
              <Checkbox
                checked={input.required}
                onChange={(_, checked) => onUpdate(index, 'required', checked)}
                size="small"
              />
            }
            label={t('flows:core.executions.inputs.required')}
          />
          {canHoldDefault && (
            <FormControlLabel
              control={
                <Checkbox
                  checked={input.constant === true}
                  onChange={(_, checked) => onUpdate(index, 'constant', checked)}
                  size="small"
                />
              }
              label={
                <LabelWithTooltip
                  label={t('flows:core.executions.inputs.constant')}
                  tooltip={t('flows:core.executions.inputs.constantTooltip')}
                />
              }
            />
          )}
        </Stack>
      </Stack>
    </Box>
  );
});

/**
 * Dynamic list editor for configuring executor inputs.
 * Allows adding/removing input field definitions with type, identifier, required flag, default value
 * and constant flag.
 *
 * @param props - Props injected to the component.
 * @returns The ExecutorInputsEditor component.
 */
function ExecutorInputsEditor({
  inputs: inputsProp,
  onChange,
  errorMessage = undefined,
}: ExecutorInputsEditorProps): ReactNode {
  const inputs = useMemo(() => (Array.isArray(inputsProp) ? inputsProp : []), [inputsProp]);
  const {t} = useTranslation();

  // Stable IDs for each entry — used as React keys so rows survive re-renders.
  const [ids, dispatchIds] = useReducer(idsReducer, inputs.length, (len) =>
    Array.from({length: len}, () => generateId()),
  );

  const syncedIds = useMemo(() => {
    if (ids.length !== inputs.length) {
      dispatchIds(inputs.length);
    }
    return ids.length === inputs.length ? ids : idsReducer(ids, inputs.length);
  }, [ids, inputs.length]);

  const handleUpdate = useCallback(
    (index: number, field: keyof FlowNodeInput, value: string | boolean) => {
      const updated = [...inputs];
      updated[index] = normalizeInput({...updated[index], [field]: value});
      onChange(updated);
    },
    [inputs, onChange],
  );

  const handleRemove = useCallback(
    (index: number) => {
      dispatchIds(inputs.length - 1);
      const updated = inputs.filter((_, i) => i !== index);
      onChange(updated);
    },
    [inputs, onChange],
  );

  const handleAdd = useCallback(() => {
    const newInput: FlowNodeInput = {
      ref: generateResourceId('input'),
      type: 'TEXT_INPUT',
      identifier: '',
      required: true,
    };
    onChange([...inputs, newInput]);
  }, [inputs, onChange]);

  return (
    <Stack gap={1.5}>
      <FormLabel>{t('flows:core.executions.inputs.title')}</FormLabel>
      {errorMessage && <FormHelperText error>{errorMessage}</FormHelperText>}
      {inputs.length === 0 && <Alert severity="info">{t('flows:core.executions.inputs.empty')}</Alert>}
      {inputs.map((input, index) => (
        <InputRow key={syncedIds[index]} input={input} index={index} onUpdate={handleUpdate} onRemove={handleRemove} />
      ))}
      <PanelActionButton startIcon={<PlusIcon size={16} />} onClick={handleAdd}>
        {t('flows:core.executions.inputs.add')}
      </PanelActionButton>
    </Stack>
  );
}

export default ExecutorInputsEditor;
