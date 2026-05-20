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

import {EmbeddedFlowComponent, EmbeddedFlowComponentType, FieldType} from '@thunderid/browser';
import {type VNode, h} from 'vue';
import FacebookButton from '../../../../adapters/FacebookButton';
import GitHubButton from '../../../../adapters/GitHubButton';
import GoogleButton from '../../../../adapters/GoogleButton';
import MicrosoftButton from '../../../../adapters/MicrosoftButton';
import {createField} from '../../../../factories/FieldFactory';
import Button from '../../../../primitives/Button';
import Typography from '../../../../primitives/Typography';

/**
 * Mirrors the logic in `packages/react/.../SignUp/v1/SignUpOptionFactory.tsx` —
 * renders the V1 flow component shapes (`TYPOGRAPHY`, `INPUT`, `BUTTON`,
 * `FORM`, `SELECT`, `DIVIDER`, `IMAGE`, `RICH_TEXT`) returned by the ThunderID
 * `/api/server/v1/flow/execute` endpoint.
 *
 * Each leaf component returns a Vue VNode (or null for unknown types). Branch
 * components (`FORM`) recurse so children render as a flat list.
 */

/**
 * Resolve the form-field name for an input component.
 * ThunderID V1 stores the bound parameter name in `config.identifier` (e.g.
 * `http://wso2.org/claims/emailaddress`), with `config.name` used as a fallback.
 */
const getInputName = (component: any): string => {
  const cfg: any = component.config || {};
  return (cfg.name as string) || (cfg.identifier as string) || (component.id as string);
};

/**
 * Map V1 INPUT variants/types to the SDK's internal `FieldType` so the existing
 * `createField` factory (used by the V1 sign-in flow too) produces the right
 * primitive (`TextField`, `PasswordField`, `Checkbox`, etc.).
 */
const inferFieldType = (component: any): FieldType => {
  const variant: string = String(component.variant || '').toUpperCase();
  const cfg: any = component.config || {};
  const cfgType: string = String(cfg.type || '').toLowerCase();

  if (variant === 'EMAIL' || cfgType === 'email') return FieldType.Email;
  if (variant === 'PASSWORD' || cfgType === 'password') return FieldType.Password;
  if (variant === 'TELEPHONE' || cfgType === 'tel') return FieldType.Text;
  if (variant === 'NUMBER' || cfgType === 'number') return FieldType.Number;
  if (variant === 'DATE' || cfgType === 'date') return FieldType.Date;
  if (variant === 'CHECKBOX' || cfgType === 'checkbox') return FieldType.Checkbox;
  return FieldType.Text;
};

/**
 * Map TYPOGRAPHY variants (H1-H6, BODY, CAPTION etc.) to the Vue Typography
 * primitive's variant prop.
 */
const inferTypographyVariant = (
  component: any,
): 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6' | 'subtitle1' | 'subtitle2' | 'body1' | 'body2' | 'caption' | 'overline' => {
  const variant: string = String(component.variant || '').toUpperCase();
  switch (variant) {
    case 'H1':
      return 'h1';
    case 'H2':
      return 'h2';
    case 'H3':
      return 'h3';
    case 'H4':
      return 'h4';
    case 'H5':
      return 'h5';
    case 'H6':
      return 'h6';
    case 'SUBTITLE1':
      return 'subtitle1';
    case 'SUBTITLE2':
      return 'subtitle2';
    case 'BODY2':
      return 'body2';
    case 'CAPTION':
      return 'caption';
    case 'OVERLINE':
      return 'overline';
    default:
      return 'body1';
  }
};

/**
 * Detect whether a BUTTON looks like a known social-login provider so we can
 * render a branded button (matches the React V1 factory's behaviour).
 */
const matchesSocialProvider = (component: any, provider: 'google' | 'github' | 'microsoft' | 'facebook'): boolean => {
  const text: string = String(component?.config?.text || component?.config?.label || '').toLowerCase();
  const variant: string = String(component?.variant || '').toUpperCase();
  return variant === 'SOCIAL' && text.includes(provider);
};

/**
 * Props shared by all sign-up component renderers.
 */
export interface BaseSignUpOptionProps {
  buttonClassName?: string;
  component: EmbeddedFlowComponent;
  formErrors: Record<string, string>;
  formValues: Record<string, string>;
  inputClassName?: string;
  isFormValid: boolean;
  isLoading: boolean;
  onInputChange: (name: string, value: string) => void;
  onSubmit: (component: EmbeddedFlowComponent, data?: Record<string, any>) => void;
  size?: 'small' | 'medium' | 'large';
  touchedFields: Record<string, boolean>;
}

/**
 * Build a VNode for a single V1 flow component. Returns `null` for unknown
 * types (caller filters these out).
 */
export const createSignUpComponent = (props: BaseSignUpOptionProps): VNode | VNode[] | null => {
  const {
    component,
    formValues,
    touchedFields,
    formErrors,
    isLoading,
    isFormValid,
    onInputChange,
    onSubmit,
    inputClassName,
    buttonClassName,
  } = props;

  const cfg: any = (component as any).config || {};

  switch (component.type) {
    case EmbeddedFlowComponentType.Typography: {
      const text = String(cfg.text || cfg.label || '');
      return h(
        Typography,
        {style: 'margin-bottom:0.5rem', variant: inferTypographyVariant(component)},
        {default: () => text},
      );
    }

    case EmbeddedFlowComponentType.Input: {
      const name: string = getInputName(component);
      const fieldType: FieldType = inferFieldType(component);
      const value: string = formValues[name] || '';
      const isTouched: boolean = touchedFields[name] || false;
      const error: string | undefined = isTouched ? formErrors[name] : undefined;

      return createField({
        className: inputClassName,
        disabled: isLoading,
        error,
        label: String(cfg.label || ''),
        name,
        onChange: (newValue: string) => onInputChange(name, newValue),
        placeholder: String(cfg.placeholder || ''),
        required: Boolean(cfg.required),
        touched: isTouched,
        type: fieldType,
        value,
      });
    }

    case EmbeddedFlowComponentType.Button: {
      const text = String(cfg.text || cfg.label || 'Submit');
      const variant: string = String(component.variant || 'PRIMARY').toUpperCase();
      const isPrimary: boolean = variant === 'PRIMARY';
      const handleClick = (): void => onSubmit(component, undefined);

      // Branded social-login buttons for known providers
      if (matchesSocialProvider(component, 'google')) {
        return h(GoogleButton, {class: buttonClassName, isLoading, onClick: handleClick});
      }
      if (matchesSocialProvider(component, 'github')) {
        return h(GitHubButton, {class: buttonClassName, isLoading, onClick: handleClick});
      }
      if (matchesSocialProvider(component, 'microsoft')) {
        return h(MicrosoftButton, {class: buttonClassName, isLoading, onClick: handleClick});
      }
      if (matchesSocialProvider(component, 'facebook')) {
        return h(FacebookButton, {class: buttonClassName, isLoading, onClick: handleClick});
      }

      // Generic submit/secondary button
      return h(
        Button,
        {
          class: buttonClassName,
          color: isPrimary ? 'primary' : 'secondary',
          'data-testid': 'thunderid-signup-submit',
          disabled: isLoading || (!isFormValid && cfg.type === 'submit'),
          fullWidth: true,
          loading: isLoading,
          onClick: handleClick,
          type: cfg.type === 'submit' ? 'submit' : 'button',
          variant: isPrimary ? 'solid' : 'outline',
        },
        {default: () => text},
      );
    }

    case EmbeddedFlowComponentType.Form: {
      // Recursively render child components inline (one form wrapper at the
      // BaseSignUp level handles native `<form>` semantics — we just unwrap
      // children here to keep the render flat).
      const children: any[] = (component as any).components || [];
      const nodes: VNode[] = [];
      children.forEach((child: EmbeddedFlowComponent) => {
        const rendered: VNode | VNode[] | null = createSignUpComponent({...props, component: child});
        if (rendered === null) return;
        if (Array.isArray(rendered)) nodes.push(...rendered);
        else nodes.push(rendered);
      });
      return nodes;
    }

    case EmbeddedFlowComponentType.Divider: {
      return h('hr', {
        class: 'thunderid-signup__divider',
        style: 'margin:0.75rem 0;border:0;border-top:1px solid #e5e7eb',
      });
    }

    case EmbeddedFlowComponentType.Image: {
      const src = String(cfg.src || cfg.url || '');
      const alt = String(cfg.alt || '');
      if (!src) return null;
      return h('img', {alt, src, style: 'max-width:100%;height:auto;display:block;margin:0.5rem auto'});
    }

    default: {
      // ThunderID's V1 flow API also returns 'RICH_TEXT' which is not in the V1
      // component-type enum (the enum predates it). Render its raw HTML so
      // links and inline copy show up.
      if (String(component.type).toUpperCase() === 'RICH_TEXT') {
        const html = String(cfg.text || cfg.label || '');
        return h('div', {class: 'thunderid-signup__rich-text', innerHTML: html});
      }
      return null;
    }
  }
};

/**
 * Render an array of V1 flow components as Vue VNodes, flattening nested
 * containers (FORM) into a single list.
 */
export const renderSignUpComponents = (
  components: EmbeddedFlowComponent[],
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  onSubmit: (component: EmbeddedFlowComponent, data?: Record<string, any>) => void,
  options?: {
    buttonClassName?: string;
    inputClassName?: string;
    size?: 'small' | 'medium' | 'large';
  },
): VNode[] => {
  const result: VNode[] = [];
  components.forEach((component: EmbeddedFlowComponent) => {
    const rendered: VNode | VNode[] | null = createSignUpComponent({
      buttonClassName: options?.buttonClassName,
      component,
      formErrors,
      formValues,
      inputClassName: options?.inputClassName,
      isFormValid,
      isLoading,
      onInputChange,
      onSubmit,
      size: options?.size,
      touchedFields,
    });
    if (rendered === null) return;
    if (Array.isArray(rendered)) result.push(...rendered);
    else result.push(rendered);
  });
  return result;
};
