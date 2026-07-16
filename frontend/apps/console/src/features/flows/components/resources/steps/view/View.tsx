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
  Box,
  Button,
  FormGroup,
  IconButton,
  Menu,
  MenuItem,
  Paper,
  Tooltip,
  Typography,
  type Theme,
} from '@wso2/oxygen-ui';
import {CogIcon, PlusIcon, TrashIcon} from '@wso2/oxygen-ui-icons-react';
import {Handle, Position, useNodeId, useReactFlow} from '@xyflow/react';
import {
  Fragment,
  memo,
  useCallback,
  useMemo,
  useState,
  type HTMLAttributes,
  type MouseEvent,
  type ReactElement,
} from 'react';
import {useTranslation} from 'react-i18next';
import dashedAddButtonSx from './dashedAddButtonSx';
import ReorderableViewElement from './ReorderableElement';
import Droppable from '../../../dnd/Droppable';
import GapDropZone from '../../../dnd/GapDropZone';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import useFlowPlugins from '@/features/flows/hooks/useFlowPlugins';
import type {Element} from '@/features/flows/models/elements';
import type {StepData} from '@/features/flows/models/steps';
import generateResourceId from '@/features/flows/utils/generateResourceId';

/**
 * Props interface of {@link View}
 */
export interface ViewPropsInterface extends Omit<HTMLAttributes<HTMLDivElement>, 'resource'> {
  /**
   * Resources for the view (required by parent components but not used here).
   * @internal
   */
  resources?: unknown;
  /**
   * Data for the view component.
   */
  data?: StepData;
  /**
   * Name of the view.
   */
  heading?: string;
  /**
   * Droppable allowed resource list.
   */
  droppableAllowedTypes?: string[];
  /**
   * Droppable restricted resource list that should not be accepted.
   * @deprecated Currently unused but kept for future implementation
   */
  // eslint-disable-next-line react/no-unused-prop-types
  droppableRestrictedTypes?: string[];
  /**
   * Flag to enable source handle.
   */
  enableSourceHandle?: boolean;
  /**
   * Event handler for double click on the action panel.
   *
   * @param event - The mouse event.
   */
  onActionPanelDoubleClick?: (event: MouseEvent<HTMLDivElement>) => void;
  /**
   * Is the view deletable.
   */
  deletable?: boolean;
  /**
   * Does the view has configurations.
   */
  configurable?: boolean;
  /**
   * Callback for configure action.
   */
  onConfigure?: () => void;
  /**
   * Callback for adding an element to the view.
   * @param element - The element to add.
   * @param viewId - The ID of the view to add to.
   */
  onAddElement?: (element: Element, viewId: string) => void;
  /**
   * List of available elements that can be added to the view.
   */
  availableElements?: Element[];
  /**
   * Callback for adding an element to a form.
   * @param element - The element to add.
   * @param formId - The ID of the form to add to.
   */
  onAddElementToForm?: (element: Element, formId: string) => void;
}

/**
 * Node for representing an empty view as a step in the flow builder.
 * TEST 12: Restore full View features (menus, buttons, Droppable).
 *
 * @param props - Props injected to the component.
 * @returns Step Node component.
 */
function View({
  heading = 'View',
  droppableAllowedTypes = undefined,
  // droppableRestrictedTypes = undefined,
  enableSourceHandle = false,
  data = undefined,
  onActionPanelDoubleClick = undefined,
  className,
  deletable = true,
  configurable = false,
  onConfigure = undefined,
  resources = undefined,
  onAddElement = undefined,
  availableElements = [],
  onAddElementToForm = undefined,
}: ViewPropsInterface): ReactElement {
  // Suppress unused variable warning - resources is required by interface but not used
  // @ts-expect-error - intentionally unused
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const unusedResources = resources;
  const {t} = useTranslation();
  const stepId: string | null = useNodeId();
  const {deleteElements, getNode} = useReactFlow();
  const {emitElementFilter} = useFlowPlugins();

  // Get current node - use getNode instead of useNodesData to avoid re-renders
  const currentNode = useMemo(() => (stepId ? getNode(stepId) : undefined), [stepId, getNode]);

  // Memoize Droppable data to prevent re-creation on every render
  const droppableData = useMemo(
    () => ({
      stepId,
      droppedOn: currentNode,
    }),
    [stepId, currentNode],
  );
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const menuOpen = Boolean(anchorEl);

  // Filter availableElements to only show items where showOnResourcePanel is not false
  const filteredAvailableElements = useMemo(
    () => availableElements.filter((element) => element.display?.showOnResourcePanel !== false),
    [availableElements],
  );

  const handleMenuOpen = useCallback((event: MouseEvent<HTMLElement>): void => {
    setAnchorEl(event.currentTarget);
  }, []);

  const handleMenuClose = useCallback((): void => {
    setAnchorEl(null);
  }, []);

  const handleAddResource = useCallback(
    (element: Element): void => {
      if (onAddElement && stepId) {
        onAddElement(element, stepId);
      }
      setAnchorEl(null);
    },
    [onAddElement, stepId],
  );

  // Filter components using plugin interceptors.
  const components = data?.components ?? [];
  const filteredComponents = components.filter((component: Element) => emitElementFilter(component));

  return (
    // <ValidationErrorBoundary disableErrorBoundaryOnHover={false} resource={node}>
    <Box
      className={`flow-builder-step ${className ?? ''}`.trim()}
      sx={(theme: Theme) => ({
        overflow: 'hidden',
        borderRadius: 2,
        ...theme.applyStyles('light', {
          boxShadow:
            '0 10px 22px 0 rgba(6,6,14,0.1), 0 24px 48px 0 rgba(199,211,234,0.05) inset, 0 1px 1px 0 rgba(199,211,234,0.12) inset',
        }),
        ...theme.applyStyles('dark', {
          boxShadow:
            '0 24px 32px 0 rgba(6,6,14,0.7), 0 24px 48px 0 rgba(199,211,234,0.05) inset, 0 1px 1px 0 rgba(199,211,234,0.12) inset',
        }),
      })}
    >
      <Box
        data-testid="step-action-panel"
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        onDoubleClick={onActionPanelDoubleClick}
        sx={{
          backgroundColor: 'secondary.main',
          px: 2,
          py: 1.25,
          height: 44,
        }}
      >
        <Typography
          variant="body2"
          sx={{
            color: 'common.white',
            fontWeight: 500,
          }}
        >
          {heading ?? 'View'}
        </Typography>
        <Box display="flex" gap={0.5}>
          {filteredAvailableElements.length > 0 && (
            <Tooltip title={t('flows:core.steps.view.addComponent')}>
              <IconButton
                size="small"
                onClick={handleMenuOpen}
                sx={(theme: Theme) => ({
                  color: 'common.white',
                  '&:hover': {
                    ...theme.applyStyles('dark', {
                      backgroundColor: 'rgba(0, 0, 0, 0.2)',
                      color: 'common.white',
                    }),
                    ...theme.applyStyles('light', {
                      backgroundColor: 'rgba(0, 0, 0, 0.1)',
                      color: 'common.white',
                    }),
                  },
                })}
              >
                <PlusIcon size={18} />
              </IconButton>
            </Tooltip>
          )}
          {configurable && (
            <Tooltip title={t('flows:core.steps.view.configure')}>
              <IconButton
                size="small"
                onClick={() => {
                  onConfigure?.();
                }}
                sx={(theme: Theme) => ({
                  color: 'common.white',
                  '&:hover': {
                    ...theme.applyStyles('dark', {
                      backgroundColor: 'rgba(0, 0, 0, 0.2)',
                      color: 'common.white',
                    }),
                    ...theme.applyStyles('light', {
                      backgroundColor: 'rgba(0, 0, 0, 0.1)',
                      color: 'common.white',
                    }),
                  },
                })}
              >
                <CogIcon size={18} />
              </IconButton>
            </Tooltip>
          )}
          {deletable && (
            <Tooltip title={t('flows:core.steps.view.remove')}>
              <IconButton
                size="small"
                onClick={() => {
                  if (stepId) {
                    // eslint-disable-next-line @typescript-eslint/no-floating-promises
                    deleteElements({nodes: [{id: stepId}]});
                  }
                }}
                sx={(theme: Theme) => ({
                  color: 'common.white',
                  '&:hover': {
                    ...theme.applyStyles('dark', {
                      backgroundColor: 'rgba(0, 0, 0, 0.2)',
                      color: 'common.white',
                    }),
                    ...theme.applyStyles('light', {
                      backgroundColor: 'rgba(0, 0, 0, 0.1)',
                      color: 'common.white',
                    }),
                  },
                })}
              >
                <TrashIcon size={18} />
              </IconButton>
            </Tooltip>
          )}
        </Box>
      </Box>

      {/* Context Menu for adding components */}
      <Menu
        anchorEl={anchorEl}
        open={menuOpen}
        onClose={handleMenuClose}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
      >
        {filteredAvailableElements && filteredAvailableElements.length > 0 ? (
          filteredAvailableElements.map((element: Element, index: number) => (
            <MenuItem
              key={element.id || `${element.type}-${element.category}-${index}`}
              onClick={() => handleAddResource(element)}
              sx={{
                minWidth: 200,
              }}
            >
              {element.display?.label || element.type}
            </MenuItem>
          ))
        ) : (
          <MenuItem disabled sx={{minWidth: 200}}>
            {t('flows:core.steps.view.noComponentsAvailable')}
          </MenuItem>
        )}
      </Menu>

      <Handle type="target" position={Position.Left} />
      <Box
        className="flow-builder-step-content"
        sx={{
          display: 'flex',
          flexFlow: 'column nowrap',
          alignContent: 'center',
          justifyContent: 'center',
          alignItems: 'center',
          textAlign: 'left',
        }}
      >
        <Paper
          elevation={0}
          sx={{
            backgroundColor: 'background.paper',
            borderRadius: '0 0 8px 8px',
            border: 'none',
            width: 350,
            minHeight: 50,
            cursor: 'auto',
          }}
        >
          <Box className="flow-builder-step-content-form">
            <FormGroup>
              <Droppable
                id={generateResourceId(VisualFlowConstants.FLOW_BUILDER_VIEW_ID)}
                type={VisualFlowConstants.FLOW_BUILDER_DROPPABLE_VIEW_ID}
                accept={droppableAllowedTypes ?? VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES}
                data={droppableData}
                sx={{p: 1}}
              >
                {filteredComponents.map((component: Element, index: number) => (
                  <Fragment key={component.id}>
                    {index > 0 && (
                      <GapDropZone
                        id={`${stepId}-gap-${index}`}
                        accept={droppableAllowedTypes ?? VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES}
                        data={{...droppableData, isReordering: true, insertBeforeElementId: component.id}}
                      />
                    )}
                    <ReorderableViewElement
                      id={component.id}
                      index={index}
                      element={component}
                      group={stepId ?? undefined}
                      type={stepId ?? VisualFlowConstants.FLOW_BUILDER_DRAGGABLE_ID}
                      accept={[
                        stepId ?? VisualFlowConstants.FLOW_BUILDER_DRAGGABLE_ID,
                        ...(droppableAllowedTypes ?? VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES),
                      ]}
                      availableElements={availableElements}
                      onAddElementToForm={onAddElementToForm}
                    />
                  </Fragment>
                ))}
              </Droppable>
              {filteredAvailableElements.length > 0 && (
                <Box sx={{px: 1, pb: 1}}>
                  <Button
                    fullWidth
                    size="small"
                    className="nodrag"
                    data-testid="view-add-element-button"
                    startIcon={<PlusIcon size={15} />}
                    onClick={handleMenuOpen}
                    sx={dashedAddButtonSx}
                  >
                    {t('flows:core.steps.view.addComponent', 'Add Component')}
                  </Button>
                </Box>
              )}
            </FormGroup>
          </Box>
        </Paper>
      </Box>
      {enableSourceHandle && (
        <Handle
          type="source"
          position={Position.Right}
          id={`${stepId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        />
      )}
    </Box>
    // </ValidationErrorBoundary>
  );
}

// Memoize View to prevent re-renders when parent re-renders with same props
// Custom comparator checks data.components by reference since they're the main changing prop
const MemoizedView = memo(View, (prevProps, nextProps) => {
  // Re-render if data changed (components array reference)
  if (prevProps.data !== nextProps.data) {
    return false;
  }
  // Re-render if heading changed
  if (prevProps.heading !== nextProps.heading) {
    return false;
  }
  // Re-render if deletable/configurable changed
  if (prevProps.deletable !== nextProps.deletable || prevProps.configurable !== nextProps.configurable) {
    return false;
  }
  // Re-render if enableSourceHandle changed
  if (prevProps.enableSourceHandle !== nextProps.enableSourceHandle) {
    return false;
  }
  // Re-render if availableElements changed — the persistent add button and its
  // menu render from it, so a late-loaded list must not be swallowed.
  if (prevProps.availableElements !== nextProps.availableElements) {
    return false;
  }
  // Don't re-render for callback changes (onAddElement, etc.)
  // These are read via refs or stable references in the component
  return true;
});

export default MemoizedView;
