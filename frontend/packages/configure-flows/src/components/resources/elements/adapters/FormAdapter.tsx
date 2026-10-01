// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CollisionPriority} from '@dnd-kit/abstract';
import {Badge, Box, Button, Menu, MenuItem, Typography} from '@wso2/oxygen-ui';
import {PlusIcon} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type MouseEvent, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import useFlowPlugins from '../../../../hooks/useFlowPlugins';
import {ElementCategories, ElementTypes, type Element as FlowElement} from '../../../../models/elements';
import generateResourceId from '../../../../utils/generateResourceId';
import Droppable from '../../../dnd/Droppable';
import dashedAddButtonSx from '../../steps/view/dashedAddButtonSx';
import ReorderableFlowElement from '../../steps/view/ReorderableElement';

/**
 * Form element type.
 */
export type FormElement = FlowElement;

/**
 * Props interface of {@link FormAdapter}
 */
export interface FormAdapterPropsInterface {
  /**
   * The form element properties.
   */
  resource: FormElement;
  /**
   * The step id the resource resides on.
   */
  stepId: string;
  /**
   * List of available elements that can be added to the form.
   */
  availableElements?: FlowElement[];
  /**
   * Callback for adding an element to the form.
   * @param element - The element to add.
   * @param formId - The ID of the form to add to.
   */
  onAddElementToForm?: (element: FlowElement, formId: string) => void;
}

/**
 * Adapter for the Form component.
 *
 * @param props - Props injected to the component.
 * @returns The FormAdapter component.
 */
function FormAdapter({
  resource,
  stepId,
  availableElements = [],
  onAddElementToForm = undefined,
}: FormAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {emitElementFilter} = useFlowPlugins();
  const [addAnchorEl, setAddAnchorEl] = useState<null | HTMLElement>(null);

  const addableElements: FlowElement[] = availableElements.filter(
    (element: FlowElement) =>
      VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES.includes(element.type) &&
      element.display?.showOnResourcePanel !== false,
  );

  const hasInputFields = resource?.components?.some(
    (element: FlowElement) =>
      element.category === ElementCategories.Field || element.type === ElementTypes.DynamicInputPlaceholder,
  );

  const shouldShowFormFieldsPlaceholder = !hasInputFields && !resource?.components?.length;

  const filteredComponents = useMemo(() => {
    if (!resource?.components) return [];
    return resource.components.filter((component: FlowElement) => emitElementFilter(component));
  }, [resource?.components, emitElementFilter]);

  return (
    <Badge
      anchorOrigin={{
        horizontal: 'left',
        vertical: 'top',
      }}
      badgeContent={t('flows:core.adapters.form.badgeLabel')}
      data-testid="form-adapter"
      sx={{
        backgroundColor: 'background.paper',
        border: '1px dashed',
        borderColor: 'divider',
        borderRadius: 2,
        display: 'block',
        minHeight: 80,
        width: '100%',
        '& .MuiBadge-badge': {
          backgroundColor: 'background.default',
          color: 'text.secondary',
          fontWeight: 500,
          ml: '25px',
        },
      }}
    >
      <Box>
        <Droppable
          id={generateResourceId(`${VisualFlowConstants.FLOW_BUILDER_FORM_ID}_${stepId}`)}
          data={{droppedOn: resource, stepId}}
          collisionPriority={CollisionPriority.High}
          type={VisualFlowConstants.FLOW_BUILDER_DROPPABLE_FORM_ID}
          accept={[
            VisualFlowConstants.FLOW_BUILDER_DRAGGABLE_ID,
            ...VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES,
          ]}
        >
          {shouldShowFormFieldsPlaceholder && (
            <Box
              data-testid="form-adapter-placeholder"
              sx={{
                alignItems: 'center',
                backgroundColor: 'action.hover',
                border: '2px dashed',
                borderColor: 'divider',
                borderRadius: 0.5,
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'center',
                m: 2,
                minHeight: 80,
                p: 2,
                transition:
                  'background-color 200ms cubic-bezier(0.2, 0, 0, 1), border-color 200ms cubic-bezier(0.2, 0, 0, 1)',
                '&:hover': {backgroundColor: 'action.selected'},
              }}
            >
              <Typography variant="body2">{t('flows:core.adapters.form.placeholder')}</Typography>
            </Box>
          )}
          {filteredComponents.map((component: FlowElement, index: number) => (
            <ReorderableFlowElement
              key={component.id}
              id={component.id}
              index={index}
              element={component}
              group={resource.id}
              type={resource.id}
              accept={[resource.id, ...VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES]}
              availableElements={availableElements}
              onAddElementToForm={onAddElementToForm}
            />
          ))}
        </Droppable>
        {/* Rendered inside the form outline so the container encloses its own add
            affordance, the same way a stack does. */}
        {addableElements.length > 0 && (
          <Box sx={{px: 2, pb: 2}}>
            <Button
              fullWidth
              size="small"
              className="nodrag"
              data-testid="form-add-field-button"
              startIcon={<PlusIcon size={15} />}
              onClick={(event: MouseEvent<HTMLElement>) => {
                event.stopPropagation();
                setAddAnchorEl(event.currentTarget);
              }}
              sx={dashedAddButtonSx}
            >
              {t('flows:core.steps.view.addField', 'Add Field')}
            </Button>
          </Box>
        )}
        <Menu
          anchorEl={addAnchorEl}
          open={Boolean(addAnchorEl)}
          onClose={() => setAddAnchorEl(null)}
          anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
          transformOrigin={{vertical: 'top', horizontal: 'right'}}
        >
          {addableElements.map((element: FlowElement) => (
            <MenuItem
              key={`${element.type}-${element.id}`}
              onClick={() => {
                setAddAnchorEl(null);
                onAddElementToForm?.(element, resource.id);
              }}
              sx={{minWidth: 200}}
            >
              {element.display?.label ?? element.type}
            </MenuItem>
          ))}
        </Menu>
      </Box>
    </Badge>
  );
}

export default FormAdapter;
