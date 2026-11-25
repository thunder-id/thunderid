// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableDefinition, FlowVariableType} from '@thunderid/react';
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
import {Plus, Variable, X} from '@wso2/oxygen-ui-icons-react';
import {useContext, useState, type MouseEvent, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowVariablesContext from '../../../../context/FlowVariablesContext';
import type {StepData} from '../../../../models/steps';
import {VARIABLE_PLACEHOLDER_PATTERN} from '../../../variables-dialog/variableIdentifier';

/**
 * What {@link VariableBinding} hands the input it draws, so the key of the property and its label are stated once.
 */
export interface VariableBindingInput {
  /**
   * Sets the property to a value, at the path the binding builds from its key.
   */
  commit: (value: unknown) => void;
  /**
   * The label of the property, as the binding was given it.
   */
  label: ReactNode;
  /**
   * The value the property holds now, or `undefined` while it is unset.
   */
  value: unknown;
}

/**
 * Props interface of {@link VariableBinding}
 */
export interface VariableBindingProps extends CommonResourcePropertiesPropsInterface {
  /**
   * The variable types the property can be bound to, which the menu is limited to. Left out, every variable is
   * offered. A property already bound to another type still shows as bound.
   */
  accepts?: FlowVariableType[];
  /**
   * Draws the input the property is set with, shown while the property is not bound to a variable.
   */
  children: (input: VariableBindingInput) => ReactNode;
  /**
   * For a property that names a connection: the type of connection it takes, such as `google`. The menu then offers
   * only the connection variables that allow it, or that do not limit the types.
   */
  connectionType?: string;
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
  /**
   * Draws `label` above the input while the property is not bound. Leave it off for an input that carries its own
   * label, such as a text field with a top-placed label, or the label shows twice.
   */
  showLabel?: boolean;
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
  accepts = undefined,
  children,
  connectionType = undefined,
  htmlFor = undefined,
  inline = false,
  label,
  labelId = undefined,
  onChange,
  propertyKey,
  required = false,
  resource,
  showLabel = false,
}: VariableBindingProps): ReactElement {
  const {t} = useTranslation();
  const {openVariablesNode, variables, variablesBefore, variablesNodeBefore} = useContext(FlowVariablesContext);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  const path = `data.properties.${propertyKey}`;
  const value: unknown = (resource?.data as StepData | undefined)?.properties?.[propertyKey];
  const boundIdentifier: string | undefined =
    typeof value === 'string' ? VARIABLE_PLACEHOLDER_PATTERN.exec(value.trim())?.[1] : undefined;
  const declared: FlowVariableDefinition[] = variables ?? [];
  // The server accepts a step to refer only to variables declared by a node every route passes before it.
  const inScope: FlowVariableDefinition[] = resource?.id ? variablesBefore(resource.id) : declared;
  // The menu offers what the step can use and the property can take, while a variable already bound is found among
  // every declared one.
  const offered: FlowVariableDefinition[] = inScope.filter(
    (variable: FlowVariableDefinition) =>
      (!accepts || accepts.includes(variable.type)) &&
      (!connectionType ||
        variable.type !== 'CONNECTION_SELECT' ||
        (variable.connectionTypes ?? []).length === 0 ||
        (variable.connectionTypes ?? []).includes(connectionType.toLowerCase())),
  );
  const bound: FlowVariableDefinition | undefined = declared.find(
    (variable: FlowVariableDefinition) => variable.identifier === boundIdentifier,
  );

  const commit = (next: unknown): void => onChange(path, next, resource);

  const bind = (identifier: string): void => commit(`{{var(${identifier})}}`);

  // The property is removed, not stored as '', which would read as an empty value for a number or an option and fail
  // validation on save. The input then shows its default again, and the executor uses its own default.
  const unbind = (): void => {
    const properties = (resource?.data as StepData | undefined)?.properties ?? {};

    onChange(
      'data.properties',
      Object.fromEntries(Object.entries(properties).filter(([key]) => key !== propertyKey)),
      resource,
    );
  };

  const handlePick = (identifier: string): void => {
    setAnchor(null);
    bind(identifier);
  };

  // A variable is declared on a variables node the flow runs before this step, so that is where adding one leads.
  const variablesNodeId: string | undefined = resource?.id ? variablesNodeBefore(resource.id) : undefined;

  // Every property has one of these buttons, so the name says which property it binds.
  const penName: string =
    typeof label === 'string'
      ? t('flows:core.executions.variableBinding.tooltipFor', 'Use a variable for {{label}}', {label})
      : t('flows:core.executions.variableBinding.tooltip', 'Use a variable');

  const pen = (
    <Tooltip title={t('flows:core.executions.variableBinding.tooltip', 'Use a variable')}>
      <IconButton
        aria-label={penName}
        data-testid={`variable-binding-${propertyKey}`}
        size="small"
        color={boundIdentifier ? 'primary' : 'default'}
        onClick={(event: MouseEvent<HTMLElement>) => setAnchor(event.currentTarget)}
      >
        <Variable size={14} />
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

  const menu = (
    <Menu anchorEl={anchor} open={anchor !== null} onClose={() => setAnchor(null)}>
      {offered.map((variable: FlowVariableDefinition) => (
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
      {offered.length === 0 && (
        <MenuItem disabled data-testid="variable-binding-none">
          <ListItemText
            primary={
              inScope.length > 0
                ? t('flows:core.executions.variableBinding.noneMatching', 'No variable of a matching type')
                : declared.length > 0
                  ? t('flows:core.executions.variableBinding.noneBefore', 'No variables declared before this step')
                  : t('flows:core.executions.variableBinding.noneDeclared', 'No variables declared yet')
            }
          />
        </MenuItem>
      )}
      <Divider />
      {/* Not `disabled`, which would take the item out of arrow-key navigation: it stays reachable, says it is
          unavailable, and gives the reason in its own text. */}
      <MenuItem
        aria-disabled={!variablesNodeId}
        onClick={() => {
          if (!variablesNodeId) return;
          setAnchor(null);
          openVariablesNode(variablesNodeId);
        }}
        sx={variablesNodeId ? undefined : {cursor: 'default', opacity: 0.6}}
        data-testid="variable-binding-add"
      >
        <ListItemIcon>
          <Plus size={16} />
        </ListItemIcon>
        <ListItemText
          primary={t('flows:core.executions.variableBinding.add', 'Add variable')}
          secondary={
            variablesNodeId
              ? undefined
              : t(
                  'flows:core.executions.variableBinding.needsNode',
                  'Add a Variables step before this step in the flow first.',
                )
          }
        />
      </MenuItem>
    </Menu>
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
              children({commit, label, value})
            )}
          </Box>
          {pen}
        </Box>
        {menu}
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
        <>
          {showLabel && (
            <FormLabel id={labelId} htmlFor={htmlFor} required={required}>
              {label}
            </FormLabel>
          )}
          {children({commit, label, value})}
        </>
      )}
      <Box sx={{position: 'absolute', right: 0, top: -4}}>{pen}</Box>
      {menu}
    </Box>
  );
}

export default VariableBinding;
