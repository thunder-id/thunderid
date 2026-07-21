/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import type {EmbeddedFlowComponent} from '@thunderid/react';
import {cn} from '@thunderid/utils';
import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import type {FlowComponent} from '../../../models/flow';
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
  onSubmit?: (action: EmbeddedFlowComponent, inputs: Record<string, string>) => void;
  onValidate?: (components: EmbeddedFlowComponent[]) => boolean;
  signUpFallbackUrl?: string;
  signInFallbackUrl?: string;
  forgotPasswordFallbackUrl?: string;
}

export default function StackAdapter({
  component,
  resolve,
  values = {},
  touched = undefined,
  fieldErrors = undefined,
  isLoading = false,
  onInputChange = () => null,
  onSubmit = () => null,
  onValidate = undefined,
  signUpFallbackUrl = undefined,
  signInFallbackUrl = undefined,
  forgotPasswordFallbackUrl = undefined,
}: StackAdapterProps): JSX.Element {
  const nestedComponents = (component.components ?? []) as FlowComponent[];

  return (
    <Stack
      id={component.id}
      className={[cn('Flow--stack'), component.classes].filter(Boolean).join(' ')}
      direction={component.direction ?? 'column'}
      spacing={component.gap ?? 2}
      alignItems={component.align ?? 'center'}
      justifyContent={component.justify ?? 'flex-start'}
    >
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
          onSubmit={onSubmit}
          onValidate={onValidate}
          maxImageSize={STACK_IMAGE_MAX_SIZE}
          signUpFallbackUrl={signUpFallbackUrl}
          signInFallbackUrl={signInFallbackUrl}
          forgotPasswordFallbackUrl={forgotPasswordFallbackUrl}
        />
      ))}
    </Stack>
  );
}
