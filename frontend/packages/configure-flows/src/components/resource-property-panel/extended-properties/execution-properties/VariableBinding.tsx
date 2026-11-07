// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableDefinition} from '@thunderid/react';
import {
  Box,
  Divider,
  FormLabel,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Pencil, Plus, X} from '@wso2/oxygen-ui-icons-react';
import {useContext, useState, type MouseEvent, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowVariablesContext, {type VariablesGroup} from '../../../../context/FlowVariablesContext';
import type {StepData} from '../../../../models/steps';
import {createVariable} from '../../../variables-dialog/variableDefinitions';
import VariablesDialog from '../../../variables-dialog/VariablesDialog';

const VARIABLE_TOKEN = /^\{\{\s*var\(\s*([A-Za-z][A-Za-z0-9_]*)\s*\)\s*\}\}$/;

/**
 * Props interface of {@link VariableBinding}
 */
export interface VariableBindingProps extends CommonResourcePropertiesPropsInterface {
  /**
   * The input the property is set with, shown while the property is not bound to a variable.
   */
  children: ReactNode;
  /**
   * The id of the input, which the label points at.
   */
  htmlFor?: string;
  /**
   * Whether the input carries its own label, as a checkbox does. The pen then sits beside the input, and `label` is
   * only the name shown once the property is bound to a variable.
   */
  inline?: boolean;
  /**
   * The id of the label, for an input that is named by it.
   */
  labelId?: string;
  /**
   * The label of the property.
   */
  label: ReactNode;
  /**
   * The key of the property under the step's `properties`.
   */
  propertyKey: string;
  required?: boolean;
}

/**
 * A property of an executor with a pen beside its label. The pen offers the variables the flow declares, and one more
 * entry to declare another. A property bound to a variable holds its `{{var(identifier)}}` placeholder, and the
 * flow reads the value of that variable when the executor runs.
 *
 * @param props - Props injected to the component.
 * @returns The VariableBinding component.
 */
function VariableBinding({
  children,
  htmlFor = undefined,
  inline = false,
  label,
  labelId = undefined,
  onChange,
  propertyKey,
  required = false,
  resource,
}: VariableBindingProps): ReactElement {
  const {t} = useTranslation();
  const {groups, setGroups, variables} = useContext(FlowVariablesContext);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [isAdding, setIsAdding] = useState<boolean>(false);

  const path = `data.properties.${propertyKey}`;
  const value: unknown = (resource?.data as StepData | undefined)?.properties?.[propertyKey];
  const boundIdentifier: string | undefined =
    typeof value === 'string' ? VARIABLE_TOKEN.exec(value.trim())?.[1] : undefined;
  const declared: FlowVariableDefinition[] = variables ?? [];
  const bound: FlowVariableDefinition | undefined = declared.find(
    (variable: FlowVariableDefinition) => variable.identifier === boundIdentifier,
  );

  const bind = (identifier: string): void => {
    onChange(path, `{{var(${identifier})}}`, resource);
  };

  const unbind = (): void => {
    onChange(path, '', resource);
  };

  const handlePick = (identifier: string): void => {
    setAnchor(null);
    bind(identifier);
  };

  // A variable added from here goes into the first variables node, and the dialog opens on it.
  const firstNodeCount: number = groups[0]?.variables.length ?? 0;
  const dialogGroups: VariablesGroup[] = groups.map((group: VariablesGroup, index: number) =>
    index === 0 ? {...group, variables: [...group.variables, createVariable(declared)]} : group,
  );

  const handleApply = (next: VariablesGroup[]): void => {
    setGroups(next);

    const added: FlowVariableDefinition | undefined = next[0]?.variables[firstNodeCount];

    if (added) {
      bind(added.identifier);
    }
  };

  const pen = (
    <Tooltip title={t('flows:core.executions.variableBinding.tooltip', 'Use a variable')}>
      <IconButton
        aria-label={t('flows:core.executions.variableBinding.tooltip', 'Use a variable')}
        data-testid={`variable-binding-${propertyKey}`}
        size="small"
        color={boundIdentifier ? 'primary' : 'default'}
        onClick={(event: MouseEvent<HTMLElement>) => setAnchor(event.currentTarget)}
      >
        <Pencil size={14} />
      </IconButton>
    </Tooltip>
  );

  const boundBox = (
    <Box
      data-testid={`variable-binding-value-${propertyKey}`}
      sx={{
        alignItems: 'center',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        display: 'flex',
        gap: 1,
        justifyContent: 'space-between',
        minHeight: 40,
        px: 1.5,
      }}
    >
      <Typography variant="body2" sx={{fontFamily: 'monospace'}} noWrap>
        {bound ? `${bound.label?.trim() ? bound.label : bound.identifier} (${bound.identifier})` : boundIdentifier}
      </Typography>
      <Tooltip title={t('flows:core.executions.variableBinding.clear', 'Use a fixed value')}>
        <IconButton
          aria-label={t('flows:core.executions.variableBinding.clear', 'Use a fixed value')}
          size="small"
          onClick={unbind}
        >
          <X size={14} />
        </IconButton>
      </Tooltip>
    </Box>
  );

  const menuAndDialog = (
    <>
      <Menu anchorEl={anchor} open={anchor !== null} onClose={() => setAnchor(null)}>
        {declared.map((variable: FlowVariableDefinition) => (
          <MenuItem
            key={variable.identifier}
            selected={variable.identifier === boundIdentifier}
            onClick={() => handlePick(variable.identifier)}
          >
            <ListItemText
              primary={variable.label?.trim() ? variable.label : variable.identifier}
              secondary={variable.identifier}
            />
          </MenuItem>
        ))}
        {declared.length > 0 && <Divider />}
        <Tooltip
          title={
            variables
              ? ''
              : t('flows:core.executions.variableBinding.needsNode', 'Add a Variables step to the flow first.')
          }
          placement="left"
        >
          <span>
            <MenuItem
              disabled={!variables}
              onClick={() => {
                setAnchor(null);
                setIsAdding(true);
              }}
              data-testid="variable-binding-add"
            >
              <ListItemIcon>
                <Plus size={16} />
              </ListItemIcon>
              <ListItemText primary={t('flows:core.executions.variableBinding.add', 'Add variable')} />
            </MenuItem>
          </span>
        </Tooltip>
      </Menu>
      {variables && (
        <VariablesDialog
          open={isAdding}
          groups={dialogGroups}
          initialIndex={firstNodeCount}
          onClose={() => setIsAdding(false)}
          onApply={handleApply}
        />
      )}
    </>
  );

  if (inline) {
    return (
      <div>
        <Box sx={{alignItems: 'flex-start', display: 'flex', gap: 1, justifyContent: 'space-between'}}>
          <Box sx={{flex: 1, minWidth: 0}}>
            {boundIdentifier ? (
              <>
                <Typography variant="subtitle2" sx={{mb: 0.5}}>
                  {label}
                </Typography>
                {boundBox}
              </>
            ) : (
              children
            )}
          </Box>
          {pen}
        </Box>
        {menuAndDialog}
      </div>
    );
  }

  // The input draws its own label above itself, so the pen sits over the right end of that label row.
  return (
    <Box sx={{position: 'relative'}}>
      {boundIdentifier ? (
        <>
          <FormLabel id={labelId} htmlFor={htmlFor} required={required}>
            {label}
          </FormLabel>
          {boundBox}
        </>
      ) : (
        children
      )}
      <Box sx={{position: 'absolute', right: 0, top: -4}}>{pen}</Box>
      {menuAndDialog}
    </Box>
  );
}

export default VariableBinding;
