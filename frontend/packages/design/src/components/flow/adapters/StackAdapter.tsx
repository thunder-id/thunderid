// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {EmbeddedFlowComponent} from '@thunderid/react';
import type {JSX} from 'react';
import StackContainer from './StackContainer';
import type {FlowComponent, RenderFlowField} from '../../../models/flow';
import FlowComponentRenderer from '../FlowComponentRenderer';

const STACK_IMAGE_MAX_SIZE = 80;

interface StackAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
  values?: Record<string, string>;
  touched?: Record<string, boolean>;
  fieldErrors?: Record<string, string>;
  isLoading?: boolean;
  onInputChange?: (field: string, value: string) => void;
  onBlur?: (field: string) => void;
  onSubmit?: (action: EmbeddedFlowComponent, inputs: Record<string, string>) => void;
  onValidate?: (components: EmbeddedFlowComponent[]) => boolean;
  renderField?: RenderFlowField;
  isSubmitDisabled?: boolean;
}

export default function StackAdapter({
  component,
  resolve,
  values = {},
  touched = undefined,
  fieldErrors = undefined,
  isLoading = false,
  onInputChange = () => null,
  onBlur = undefined,
  onSubmit = () => null,
  onValidate = undefined,
  renderField = undefined,
  isSubmitDisabled = undefined,
}: StackAdapterProps): JSX.Element {
  const nestedComponents = (component.components ?? []) as FlowComponent[];

  return (
    <StackContainer component={component}>
      {nestedComponents.map((nested: FlowComponent, nestedIndex: number) => (
        <FlowComponentRenderer
          key={nested.id ?? nestedIndex}
          component={nested}
          index={nestedIndex}
          values={values}
          touched={touched}
          fieldErrors={fieldErrors}
          isLoading={isLoading}
          resolve={resolve}
          onInputChange={onInputChange}
          onBlur={onBlur}
          onSubmit={onSubmit}
          onValidate={onValidate}
          renderField={renderField}
          isSubmitDisabled={isSubmitDisabled}
          maxImageSize={STACK_IMAGE_MAX_SIZE}
        />
      ))}
    </StackContainer>
  );
}
