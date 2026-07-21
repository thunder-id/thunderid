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

import {EmbeddedFlowComponentType, EmbeddedFlowEventType, type ConsentPurpose} from '@thunderid/react';
import type {JSX} from 'react';
import BlockAdapter from './adapters/BlockAdapter';
import ConsentAdapter from './adapters/ConsentAdapter';
import CopyableTextAdapter from './adapters/CopyableTextAdapter';
import DividerAdapter from './adapters/DividerAdapter';
import IconAdapter from './adapters/IconAdapter';
import ImageAdapter from './adapters/ImageAdapter';
import QrCodeAdapter from './adapters/QrCodeAdapter';
import RichTextAdapter from './adapters/RichTextAdapter';
import StackAdapter from './adapters/StackAdapter';
import StandaloneTriggerAdapter from './adapters/StandaloneTriggerAdapter';
import TextAdapter from './adapters/TextAdapter';
import TimerAdapter from './adapters/TimerAdapter';
import type {FlowComponent, FlowComponentRendererProps} from '../../models/flow';

/**
 * Factory component that maps an embedded flow component to the appropriate adapter.
 *
 * Supported top-level types:
 * - `TEXT` → {@link TextAdapter}
 * - `RICH_TEXT` → {@link RichTextAdapter}
 * - `IMAGE` → {@link ImageAdapter}
 * - `ICON` → {@link IconAdapter}
 * - `STACK` → {@link StackAdapter}
 * - `DIVIDER` → {@link DividerAdapter}
 * - `BLOCK` (form or trigger) → {@link BlockAdapter}
 * - `ACTION / TRIGGER` (standalone) → {@link StandaloneTriggerAdapter}
 *
 * Consumers must wrap their submit/trigger handlers into the normalised
 * `onSubmit(action, inputs)` callback.  Setting a `key` on the rendered
 * `<FlowComponentRenderer>` is the caller's responsibility.
 */
export default function FlowComponentRenderer({
  component,
  index,
  values,
  touched,
  fieldErrors,
  isLoading,
  resolve,
  onInputChange,
  onSubmit,
  onValidate,
  maxImageSize,
  additionalData,
  signUpFallbackUrl,
  signInFallbackUrl,
  forgotPasswordFallbackUrl,
}: FlowComponentRendererProps): JSX.Element | null {
  const comp = component as FlowComponent;

  // TEXT
  if ((comp.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Text || comp.type === 'TEXT') {
    return <TextAdapter component={comp} resolve={resolve} />;
  }

  // RICH_TEXT
  if (comp.type === 'RICH_TEXT') {
    return (
      <RichTextAdapter
        component={comp}
        resolve={resolve}
        signUpFallbackUrl={signUpFallbackUrl}
        signInFallbackUrl={signInFallbackUrl}
        forgotPasswordFallbackUrl={forgotPasswordFallbackUrl}
        values={values}
        onSubmit={onSubmit}
      />
    );
  }

  // IMAGE
  if (comp.type === 'IMAGE') {
    return <ImageAdapter component={comp} resolve={resolve} maxWidth={maxImageSize} maxHeight={maxImageSize} />;
  }

  // ICON
  if (comp.type === 'ICON') {
    return <IconAdapter component={comp} />;
  }

  // STACK
  if (comp.type === 'STACK') {
    return (
      <StackAdapter
        component={comp}
        resolve={resolve}
        values={values}
        touched={touched}
        fieldErrors={fieldErrors}
        isLoading={isLoading}
        onInputChange={onInputChange}
        onSubmit={onSubmit}
        onValidate={onValidate}
        signUpFallbackUrl={signUpFallbackUrl}
        signInFallbackUrl={signInFallbackUrl}
        forgotPasswordFallbackUrl={forgotPasswordFallbackUrl}
      />
    );
  }

  // TIMER (standalone countdown timer component)
  if (comp.type === 'TIMER') {
    const stepTimeout = additionalData?.['stepTimeout'];
    // eslint-disable-next-line react-hooks/purity -- Date.now() is intentional for computing remaining time
    const expiresIn = stepTimeout != null ? Math.max(0, Math.floor((Number(stepTimeout) - Date.now()) / 1000)) : 0;
    const textTemplate = resolve(comp.label) ?? 'Time remaining: {time}';

    return <TimerAdapter expiresIn={expiresIn} textTemplate={textTemplate} />;
  }

  // DIVIDER
  if (comp.type === 'DIVIDER') {
    return <DividerAdapter component={comp} resolve={resolve} />;
  }

  // BLOCK (form block or trigger block)
  // When additionalData contains consent data, inject ConsentAdapter alongside the block.
  if ((comp.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Block || comp.type === 'BLOCK') {
    const hasConsent = additionalData?.['consentPrompt'] != null;
    const hasTimer = additionalData?.['stepTimeout'] != null;
    const stepTimeout = additionalData?.['stepTimeout'];
    // eslint-disable-next-line react-hooks/purity -- Date.now() is intentional for computing remaining time
    const expiresIn = stepTimeout != null ? Math.max(0, Math.floor((Number(stepTimeout) - Date.now()) / 1000)) : 0;
    const isExpiredOnMount = hasTimer && expiresIn <= 0;

    if (hasConsent) {
      return (
        <>
          <ConsentAdapter
            consentData={
              additionalData?.['consentPrompt'] as string | ConsentPurpose[] | {purposes: ConsentPurpose[]} | undefined
            }
            formValues={values}
            onInputChange={onInputChange}
          />
          <BlockAdapter
            component={component}
            index={index}
            values={values}
            touched={touched}
            fieldErrors={fieldErrors}
            isLoading={isLoading || isExpiredOnMount}
            resolve={resolve}
            onInputChange={onInputChange}
            onSubmit={onSubmit}
            onValidate={onValidate}
            signUpFallbackUrl={signUpFallbackUrl}
            signInFallbackUrl={signInFallbackUrl}
            forgotPasswordFallbackUrl={forgotPasswordFallbackUrl}
          />
        </>
      );
    }

    return (
      <BlockAdapter
        component={component}
        index={index}
        values={values}
        touched={touched}
        fieldErrors={fieldErrors}
        isLoading={isLoading || isExpiredOnMount}
        resolve={resolve}
        onInputChange={onInputChange}
        onSubmit={onSubmit}
        onValidate={onValidate}
        signUpFallbackUrl={signUpFallbackUrl}
        signInFallbackUrl={signInFallbackUrl}
        forgotPasswordFallbackUrl={forgotPasswordFallbackUrl}
      />
    );
  }

  // COPYABLE_TEXT
  if (comp.type === 'COPYABLE_TEXT') {
    return <CopyableTextAdapter component={comp} resolve={resolve} additionalData={additionalData} />;
  }

  // QR_CODE
  if (comp.type === 'QR_CODE') {
    return <QrCodeAdapter component={comp} additionalData={additionalData} />;
  }

  // Standalone ACTION / TRIGGER (outside of a block)
  if (
    (comp.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
    comp.eventType === EmbeddedFlowEventType.Trigger
  ) {
    return (
      <StandaloneTriggerAdapter
        component={comp}
        index={index}
        isLoading={isLoading}
        resolve={resolve}
        onSubmit={onSubmit}
        values={values}
      />
    );
  }

  return null;
}
