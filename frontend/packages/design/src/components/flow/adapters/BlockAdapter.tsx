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

import {EmbeddedFlowComponentType, EmbeddedFlowEventType, type EmbeddedFlowComponent} from '@thunderid/react';
import {cn} from '@thunderid/utils';
import {Box, Button} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DividerAdapter from './DividerAdapter';
import OtpInputAdapter from './OtpInputAdapter';
import PasswordInputAdapter from './PasswordInputAdapter';
import RichTextAdapter from './RichTextAdapter';
import SelectAdapter from './SelectAdapter';
import TextInputAdapter from './TextInputAdapter';
import type {FlowComponent, FlowFieldProps} from '../../../models/flow';
import getIntegrationIcon from '../../../utils/getIntegrationIcon';

interface BlockContext {
  values: Record<string, string>;
  touched?: Record<string, boolean>;
  fieldErrors?: Record<string, string>;
  isLoading: boolean;
  resolve: (template: string | undefined) => string | undefined;
  onInputChange: (field: string, value: string) => void;
  onSubmit: (action: EmbeddedFlowComponent, inputs: Record<string, string>) => void;
  onValidate?: (components: EmbeddedFlowComponent[]) => boolean;
  passwordAutoComplete?: 'current-password' | 'new-password';
  blockComponents?: EmbeddedFlowComponent[];
  /** When true, non-primary submit buttons use onClick instead of form submit */
  hasMultipleSubmits?: boolean;
  /** ID of the primary submit action that stays as type="submit" */
  primarySubmitId?: string;
  /** Fallback sign-up URL for RICH_TEXT elements that embed `application.sign_up_url` */
  signUpFallbackUrl?: string;
  /** Fallback sign-in URL for RICH_TEXT elements that embed `application.sign_in_url` */
  signInFallbackUrl?: string;
  /** Fallback forgot-password URL for RICH_TEXT elements that embed `application.forgot_password_url` */
  forgotPasswordFallbackUrl?: string;
}

interface SubmitButtonAdapterProps {
  component: FlowComponent;
  isLoading: boolean;
  resolve: (template: string | undefined) => string | undefined;
  /** When provided, the button fires its own action via onClick instead of form submit */
  onClick?: () => void;
}

function SubmitButtonAdapter({
  component,
  isLoading,
  resolve,
  onClick = undefined,
}: SubmitButtonAdapterProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Button
      type={onClick ? 'button' : 'submit'}
      fullWidth
      className={cn(
        'Flow--submitButton',
        'Button--root',
        component.variant === 'PRIMARY' ? 'Button--primary' : 'Button--outlined',
      )}
      variant={component.variant === 'PRIMARY' ? 'contained' : 'outlined'}
      disabled={isLoading}
      onClick={onClick}
      sx={{mt: 2}}
    >
      {t(resolve(component.label)!)}
    </Button>
  );
}

interface ResendButtonAdapterProps {
  component: FlowComponent;
  isLoading: boolean;
  resolve: (template: string | undefined) => string | undefined;
}

function ResendButtonAdapter({component, isLoading, resolve}: ResendButtonAdapterProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Button
      type="submit"
      fullWidth
      className={cn('Flow--resendButton', 'Button--root')}
      variant="text"
      disabled={isLoading}
      sx={{mt: 1}}
    >
      {t(resolve(component.label)!)}
    </Button>
  );
}

interface TriggerButtonAdapterProps {
  component: FlowComponent;
  isLoading: boolean;
  resolve: (template: string | undefined) => string | undefined;
  onSubmit: (action: EmbeddedFlowComponent, inputs: Record<string, string>) => void;
  values: Record<string, string>;
  blockComponents?: EmbeddedFlowComponent[];
  onValidate?: (components: EmbeddedFlowComponent[]) => boolean;
}

function TriggerButtonAdapter({
  component,
  isLoading,
  resolve,
  onSubmit,
  values,
  blockComponents = undefined,
  onValidate = undefined,
}: TriggerButtonAdapterProps): JSX.Element {
  const {t} = useTranslation();
  const resolvedStartIcon = resolve(component.startIcon ?? component.image ?? '');

  const iconElement =
    resolvedStartIcon && /^https?:\/\//i.test(resolvedStartIcon) ? (
      <Box component="img" src={resolvedStartIcon} sx={{width: 20, height: 20, objectFit: 'contain'}} />
    ) : (
      getIntegrationIcon(String(component.label ?? ''), resolvedStartIcon ?? '')
    );

  return (
    <Button
      fullWidth
      className={cn(
        'Flow--triggerButton',
        'Button--root',
        component.variant === 'PRIMARY' ? 'Button--primary' : 'Button--secondary',
      )}
      variant={component.variant === 'PRIMARY' ? 'contained' : 'outlined'}
      disabled={isLoading}
      startIcon={iconElement}
      onClick={() => {
        if (onValidate && blockComponents && !onValidate(blockComponents)) return;
        onSubmit(component, values);
      }}
    >
      {t(resolve(component.label)!)}
    </Button>
  );
}

function renderFormSubComponent(
  subComponent: EmbeddedFlowComponent,
  compIndex: number,
  ctx: BlockContext,
): JSX.Element | null {
  const sub = subComponent as FlowComponent;
  const fieldProps: FlowFieldProps = {
    component: sub,
    values: ctx.values,
    touched: ctx.touched,
    fieldErrors: ctx.fieldErrors,
    isLoading: ctx.isLoading,
    resolve: ctx.resolve,
    onInputChange: ctx.onInputChange,
  };

  if (
    (sub.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.TextInput ||
    sub.type === 'TEXT_INPUT' ||
    sub.type === 'EMAIL_INPUT' ||
    sub.type === 'PHONE_INPUT'
  ) {
    return <TextInputAdapter key={sub.id ?? compIndex} {...fieldProps} />;
  }

  if (
    (sub.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PasswordInput ||
    sub.type === 'PASSWORD_INPUT'
  ) {
    return (
      <PasswordInputAdapter
        key={sub.id ?? compIndex}
        {...fieldProps}
        passwordAutoComplete={ctx.passwordAutoComplete ?? 'current-password'}
      />
    );
  }

  if (sub.type === 'OTP_INPUT') {
    return <OtpInputAdapter key={sub.id ?? compIndex} {...fieldProps} />;
  }

  if (sub.type === 'SELECT') {
    return <SelectAdapter key={sub.id ?? compIndex} {...fieldProps} />;
  }

  if (sub.type === 'RICH_TEXT') {
    return (
      <RichTextAdapter
        key={sub.id ?? compIndex}
        component={sub}
        resolve={ctx.resolve}
        signUpFallbackUrl={ctx.signUpFallbackUrl}
        signInFallbackUrl={ctx.signInFallbackUrl}
        forgotPasswordFallbackUrl={ctx.forgotPasswordFallbackUrl}
      />
    );
  }

  if (
    (sub.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
    sub.eventType === EmbeddedFlowEventType.Submit
  ) {
    return (
      <SubmitButtonAdapter
        key={sub.id ?? compIndex}
        component={sub}
        isLoading={ctx.isLoading}
        resolve={ctx.resolve}
        onClick={
          ctx.hasMultipleSubmits && sub.id !== ctx.primarySubmitId
            ? () => {
                if (ctx.onValidate && ctx.blockComponents && !ctx.onValidate(ctx.blockComponents)) return;
                ctx.onSubmit(sub, ctx.values);
              }
            : undefined
        }
      />
    );
  }

  if (sub.type === 'RESEND' && sub.eventType === EmbeddedFlowEventType.Submit) {
    return (
      <ResendButtonAdapter key={sub.id ?? compIndex} component={sub} isLoading={ctx.isLoading} resolve={ctx.resolve} />
    );
  }

  if (
    (sub.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
    sub.eventType === EmbeddedFlowEventType.Trigger
  ) {
    return (
      <TriggerButtonAdapter
        key={sub.id ?? compIndex}
        component={sub}
        isLoading={ctx.isLoading}
        resolve={ctx.resolve}
        onSubmit={ctx.onSubmit}
        values={ctx.values}
        blockComponents={ctx.blockComponents}
        onValidate={ctx.onValidate}
      />
    );
  }

  if (sub.type === 'DIVIDER') {
    return <DividerAdapter key={sub.id ?? compIndex} component={sub} resolve={ctx.resolve} />;
  }

  return null;
}

interface FormBlockAdapterProps extends BlockContext {
  component: EmbeddedFlowComponent;
  index: number;
}

function FormBlockAdapter({component, index, ...ctx}: FormBlockAdapterProps): JSX.Element {
  const blockComponents: EmbeddedFlowComponent[] = component.components ?? [];

  const submitActions = blockComponents.filter(
    (c) =>
      (c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
      c.eventType === EmbeddedFlowEventType.Submit,
  );
  const hasMultipleSubmits = submitActions.length > 1;

  // The primary (or first) submit action is the default form submit target,
  // ensuring Enter-key submission works even when there are multiple actions.
  const primarySubmit = submitActions.find((c) => (c as FlowComponent).variant === 'PRIMARY') ?? submitActions[0];

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (ctx.onValidate && !ctx.onValidate(blockComponents)) return;
    if (primarySubmit) ctx.onSubmit(primarySubmit, ctx.values);
  };

  return (
    <Box
      key={component.id ?? index}
      component="form"
      className={cn('Flow--form')}
      onSubmit={handleSubmit}
      noValidate
      sx={{display: 'flex', flexDirection: 'column', width: '100%', gap: 2}}
    >
      {blockComponents.map((subComponent, compIndex) =>
        renderFormSubComponent(subComponent, compIndex, {
          ...ctx,
          blockComponents,
          hasMultipleSubmits,
          primarySubmitId: primarySubmit?.id,
        }),
      )}
    </Box>
  );
}

interface TriggerBlockAdapterProps extends BlockContext {
  component: EmbeddedFlowComponent;
  index: number;
}

function TriggerBlockAdapter({component, index, ...ctx}: TriggerBlockAdapterProps): JSX.Element {
  const blockComponents: EmbeddedFlowComponent[] = component.components ?? [];

  return (
    <Box
      key={component.id ?? index}
      className={cn('Flow--triggerBlock')}
      sx={{display: 'flex', flexDirection: 'column', width: '100%', gap: 2, mt: 2}}
    >
      {blockComponents.map((actionComponent, actionIndex) => {
        const sub = actionComponent as FlowComponent;
        if (
          (sub.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
          sub.eventType === EmbeddedFlowEventType.Trigger
        ) {
          return (
            <TriggerButtonAdapter
              key={sub.id ?? actionIndex}
              component={sub}
              isLoading={ctx.isLoading}
              resolve={ctx.resolve}
              onSubmit={ctx.onSubmit}
              values={ctx.values}
            />
          );
        }
        if (sub.type === 'DIVIDER') {
          return <DividerAdapter key={sub.id ?? actionIndex} component={sub} resolve={ctx.resolve} />;
        }
        return null;
      })}
    </Box>
  );
}

interface BlockAdapterProps extends BlockContext {
  component: EmbeddedFlowComponent;
  index: number;
}

export default function BlockAdapter({
  component,
  index,
  blockComponents: outerBlockComponents = undefined,
  onValidate = undefined,
  ...ctx
}: BlockAdapterProps): JSX.Element | null {
  const blockComponents: EmbeddedFlowComponent[] = component.components ?? [];

  const hasSubmit = blockComponents.some(
    (c) =>
      ((c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
        c.eventType === EmbeddedFlowEventType.Submit) ||
      (c.type === 'RESEND' && c.eventType === EmbeddedFlowEventType.Submit),
  );

  const hasTrigger = blockComponents.some(
    (c) =>
      (c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.Action &&
      c.eventType === EmbeddedFlowEventType.Trigger,
  );

  if (hasSubmit)
    return (
      <FormBlockAdapter
        component={component}
        index={index}
        blockComponents={outerBlockComponents}
        onValidate={onValidate}
        {...ctx}
      />
    );
  if (hasTrigger)
    return (
      <TriggerBlockAdapter
        component={component}
        index={index}
        blockComponents={outerBlockComponents}
        onValidate={onValidate}
        {...ctx}
      />
    );
  return null;
}
