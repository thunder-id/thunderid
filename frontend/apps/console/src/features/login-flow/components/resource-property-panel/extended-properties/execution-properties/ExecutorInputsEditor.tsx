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
  Alert,
  Box,
  Checkbox,
  FormControlLabel,
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
import {INPUT_TYPES} from './constants';
import PanelActionButton from '@/features/flows/components/resource-property-panel/PanelActionButton';
import type {FlowNodeInput} from '@/features/flows/models/responses';
import generateResourceId from '@/features/flows/utils/generateResourceId';

/**
 * Props interface of {@link ExecutorInputsEditor}
 */
export interface ExecutorInputsEditorProps {
  inputs: FlowNodeInput[];
  onChange: (inputs: FlowNodeInput[]) => void;
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

interface InputRowProps {
  input: FlowNodeInput;
  index: number;
  onUpdate: (index: number, field: keyof FlowNodeInput, value: string | boolean) => void;
  onRemove: (index: number) => void;
}

/**
 * A single input card with type dropdown, identifier field, required checkbox, and remove action.
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
      </Stack>
    </Box>
  );
});

/**
 * Dynamic list editor for configuring executor inputs.
 * Allows adding/removing input field definitions with type, identifier, and required flag.
 *
 * @param props - Props injected to the component.
 * @returns The ExecutorInputsEditor component.
 */
function ExecutorInputsEditor({inputs: inputsProp, onChange}: ExecutorInputsEditorProps): ReactNode {
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
      updated[index] = {...updated[index], [field]: value};
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
