// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type Node, useReactFlow} from '@xyflow/react';
import cloneDeep from 'lodash-es/cloneDeep';
import set from 'lodash-es/set';
import {useCallback, useEffect} from 'react';
import useFlowPlugins from './useFlowPlugins';
import FlowBuilderElementConstants from '../constants/FlowBuilderElementConstants';
import type {Properties} from '../models/base';
import {BlockTypes, ElementTypes} from '../models/elements';
import type {Element} from '../models/elements';
import generateResourceId from '../utils/generateResourceId';

const useConfirmPasswordField = (): void => {
  const {getNode, updateNodeData} = useReactFlow();
  const {onPropertyChange, onPropertyPanelOpen, onNodeElementDelete} = useFlowPlugins();

  /**
   * Adds a confirm password field to the form when the password field's confirm checkbox is checked.
   *
   * @param propertyKey - The key of the property being changed.
   * @param newValue - The new value of the property.
   * @param element - The element being modified.
   * @param stepId - The ID of the step where the element is located.
   * @param renderProperties - Function to re-render properties after modification.
   * @returns Returns false if the confirm password field is added, true otherwise.
   */
  const addConfirmPasswordField = useCallback(
    (propertyKey: string, newValue: unknown, element: Element, stepId: string): boolean => {
      if (element.type === ElementTypes.PasswordInput) {
        if (propertyKey === 'requireConfirmation' && !newValue) {
          updateNodeData(stepId, (node: Node) => {
            if (!node.data['components']) {
              return {};
            }

            const components: Element[] = cloneDeep(node.data['components']) as Element[];
            let formFound = false;
            let passwordConfirmFieldIndex: number | undefined;

            components.every((component: Element) => {
              if (component.type === BlockTypes.Form) {
                component?.components?.every((c: Element) => {
                  if (c.id === element.id) {
                    formFound = true;

                    return false;
                  }

                  return true;
                });

                if (formFound) {
                  component?.components?.every((c: Element, index: number) => {
                    const cWithIdentifier = c as Element & {identifier?: string};
                    if (
                      cWithIdentifier?.identifier === FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER &&
                      formFound
                    ) {
                      passwordConfirmFieldIndex = index;

                      return false;
                    }

                    return true;
                  });
                }

                if (passwordConfirmFieldIndex === undefined) {
                  return true;
                }

                component?.components?.splice(passwordConfirmFieldIndex, 1);

                return false;
              }

              return true;
            });

            return {
              components,
            };
          });

          return false;
        }

        if (propertyKey === 'requireConfirmation' && newValue) {
          updateNodeData(stepId, (node: Node) => {
            if (!node.data['components']) {
              return {};
            }

            const components: Element[] = cloneDeep(node.data['components']) as Element[];
            let passwordFieldIndex: number;

            components.every((component: Element) => {
              if (component.type === BlockTypes.Form) {
                component?.components?.every((c: Element, index: number) => {
                  if (c.id === element.id) {
                    passwordFieldIndex = index;

                    return false;
                  }

                  return true;
                });

                if (passwordFieldIndex === undefined) {
                  return true;
                }

                const confirmPasswordField: Element = cloneDeep(element);

                // Set properties at top level (new format)
                (confirmPasswordField as Element & {identifier?: string}).identifier =
                  FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER;
                (confirmPasswordField as Element & {label?: string}).label = 'Confirm Password';
                (confirmPasswordField as Element & {placeholder?: string}).placeholder =
                  'Enter your password confirmation';
                confirmPasswordField.id = generateResourceId('field');

                component?.components?.splice(passwordFieldIndex + 1, 0, confirmPasswordField);

                return false;
              }

              return true;
            });

            return {
              components,
            };
          });

          return false;
        }
      }

      return true;
    },
    [updateNodeData],
  );

  /**
   * Adds properties to the confirm password field when confirmation is enabled for a password field.
   *
   * @param resource - The resource element to which properties are being added.
   * @param properties - The properties object to which confirm password field properties will be added.
   * @param stepId - The ID of the step where the resource is located.
   * @returns return true.
   */
  const addConfirmPasswordFieldProperties = useCallback(
    (resource: Element, properties: Properties, stepId: string): boolean => {
      if (resource.type === ElementTypes.PasswordInput) {
        const resourceIdentifier = (resource as Element & {identifier?: string})?.identifier;
        if (resourceIdentifier === FlowBuilderElementConstants.PASSWORD_IDENTIFIER) {
          let passwordConfirmationField: Element | undefined;
          let formFound = false;
          const node = getNode(stepId);

          if (node?.data?.['components']) {
            const components: Element[] = cloneDeep(node.data['components']) as Element[];

            components.every((component: Element) => {
              if (component.type === BlockTypes.Form) {
                component?.components?.every((c: Element) => {
                  if (c.id === resource.id) {
                    formFound = true;

                    return false;
                  }

                  return true;
                });

                if (formFound) {
                  component?.components?.every((c: Element) => {
                    const cIdentifier = (c as Element & {identifier?: string})?.identifier;
                    if (cIdentifier === FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER && formFound) {
                      passwordConfirmationField = c;

                      return false;
                    }

                    return true;
                  });
                }

                if (formFound) {
                  return false;
                }
              }

              return true;
            });
          }

          // Check both the node data (for existing confirm fields) and the resource property
          // The resource property is updated immediately when the checkbox is clicked,
          // while the node data update may be asynchronous
          const resourceWithConfirm = resource as Element & {requireConfirmation?: boolean};

          // If the resource has an explicit requireConfirmation value, use it (handles immediate UI updates)
          // Otherwise, fall back to checking if the confirm field exists in the node data
          let hasConfirmation: boolean;
          if (resourceWithConfirm.requireConfirmation !== undefined) {
            hasConfirmation = resourceWithConfirm.requireConfirmation;
          } else {
            hasConfirmation = !!passwordConfirmationField;
          }

          type FieldWithProps = Element & {hint?: string; label?: string; placeholder?: string};
          const fieldWithProps = passwordConfirmationField as FieldWithProps | undefined;

          Object.assign(properties, {
            requireConfirmation: hasConfirmation,
            // Show confirm field properties if confirmation is enabled
            ...(hasConfirmation && {
              confirmHint: fieldWithProps?.hint ?? '',
              confirmLabel: fieldWithProps?.label ?? 'Confirm Password',
              confirmPlaceholder: fieldWithProps?.placeholder ?? 'Enter your password confirmation',
            }),
          });
        }
      }

      return true;
    },
    [getNode],
  );

  /**
   * Updates the properties of the confirm password field when the password field's properties are changed.
   *
   * @param propertyKey - The key of the property being changed.
   * @param newValue - The new value of the property.
   * @param element - The element being modified.
   * @param stepId - The ID of the step where the element is located.
   * @returns Returns false if the confirm password field properties are updated, true otherwise.
   */
  const updateConfirmPasswordFieldProperties = useCallback(
    (propertyKey: string, newValue: unknown, element: Element, stepId: string): boolean => {
      if (element.type === ElementTypes.PasswordInput) {
        if (
          propertyKey === 'confirmHint' ||
          propertyKey === 'confirmLabel' ||
          propertyKey === 'confirmPlaceholder' ||
          propertyKey === 'required'
        ) {
          updateNodeData(stepId, (node: Node) => {
            if (!node.data['components']) {
              return {};
            }

            const components: Element[] = cloneDeep(node.data['components']) as Element[];
            let passwordFieldIndex: number;

            components.every((component: Element) => {
              if (component.type === BlockTypes.Form) {
                component?.components?.every((c: Element, index: number) => {
                  if (c.id === element.id) {
                    passwordFieldIndex = index;

                    return false;
                  }

                  return true;
                });

                if (passwordFieldIndex === undefined) {
                  return true;
                }

                // Extract the property name (e.g., 'confirmHint' -> 'hint', 'confirmLabel' -> 'label')
                let propertyName: string = propertyKey;

                if (propertyName.startsWith('confirm')) {
                  // Remove 'confirm' prefix and lowercase the first letter (e.g., 'confirmHint' -> 'hint')
                  propertyName = propertyName.substring(7);
                  propertyName = propertyName.charAt(0).toLowerCase() + propertyName.slice(1);
                }

                if (component.components) {
                  set(component.components[passwordFieldIndex + 1], propertyName, newValue);
                }

                return false;
              }

              return true;
            });

            return {
              components,
            };
          });

          if (propertyKey !== 'required') {
            return false;
          }
        }
      }

      return true;
    },
    [updateNodeData],
  );

  /**
   * Deletes the confirm password field from the form when the password field is removed.
   *
   * @param stepId - The ID of the step where the resource is located.
   * @param resource - The resource element to be checked.
   * @returns Returns true.
   */
  const deleteConfirmPasswordField = useCallback(
    (stepId: string, resource: Element): boolean => {
      const resourceIdentifier = (resource as Element & {identifier?: string})?.identifier;
      if (
        resource.type === ElementTypes.PasswordInput &&
        resourceIdentifier === FlowBuilderElementConstants.PASSWORD_IDENTIFIER
      ) {
        updateNodeData(stepId, (node: Node) => {
          if (!node.data['components']) {
            return {};
          }

          const components: Element[] = cloneDeep(node.data['components']) as Element[];
          let formFound = false;

          components.every((component: Element, componentIndex: number) => {
            if (component.type === BlockTypes.Form) {
              component?.components?.every((c: Element) => {
                if (c.id === resource.id) {
                  formFound = true;

                  return false;
                }

                return true;
              });

              if (formFound && component.components) {
                components[componentIndex] = {
                  ...component,
                  components: component.components.filter((c: Element) => {
                    const cIdentifier = (c as Element & {identifier?: string})?.identifier;
                    return cIdentifier !== FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER;
                  }),
                };

                return false;
              }
            }

            return true;
          });

          return {
            components,
          };
        });
      }

      return true;
    },
    [updateNodeData],
  );

  useEffect(() => onPropertyChange(addConfirmPasswordField), [onPropertyChange, addConfirmPasswordField]);
  useEffect(
    () => onPropertyChange(updateConfirmPasswordFieldProperties),
    [onPropertyChange, updateConfirmPasswordFieldProperties],
  );
  useEffect(
    () => onPropertyPanelOpen(addConfirmPasswordFieldProperties),
    [onPropertyPanelOpen, addConfirmPasswordFieldProperties],
  );
  useEffect(() => onNodeElementDelete(deleteConfirmPasswordField), [onNodeElementDelete, deleteConfirmPasswordField]);
};

export default useConfirmPasswordField;
