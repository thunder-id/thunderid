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
  FieldType,
  FlowMetadataResponse,
  EmbeddedFlowComponentV2 as EmbeddedFlowComponent,
  EmbeddedFlowComponentTypeV2 as EmbeddedFlowComponentType,
  EmbeddedFlowTextVariantV2 as EmbeddedFlowTextVariant,
  EmbeddedFlowEventTypeV2 as EmbeddedFlowEventType,
  resolveFlowTemplateLiterals,
  extractEmojiFromUri,
  isEmojiUri,
  ConsentPurposeDataV2 as ConsentPurposeData,
  ConsentPromptDataV2 as ConsentPromptData,
  ConsentDecisionsV2 as ConsentDecisions,
  ConsentPurposeDecisionV2 as ConsentPurposeDecision,
  ConsentAttributeElementV2 as ConsentAttributeElement,
} from '@thunderid/browser';
import DOMPurify from 'dompurify';
import {h, type VNode} from 'vue';
import {createVueLogger} from '../../../utils/logger';
import FacebookButton from '../../adapters/FacebookButton';
import GitHubButton from '../../adapters/GitHubButton';
import GoogleButton from '../../adapters/GoogleButton';
import MicrosoftButton from '../../adapters/MicrosoftButton';
import {createField} from '../../factories/FieldFactory';
import Button from '../../primitives/Button';
import Divider from '../../primitives/Divider';
import Select from '../../primitives/Select/Select';
import Typography from '../../primitives/Typography';

const logger: ReturnType<typeof createVueLogger> = createVueLogger('AuthOptionFactory');

/**
 * Inline helper for consent optional attribute key (mirrors ConsentCheckboxList.getConsentOptionalKey).
 */
const getConsentOptionalKey = (purposeId: string | number, attr: string): string => `consent_${purposeId}_${attr}`;

/**
 * Replaces `emoji:` URIs embedded in HTML before DOMPurify sanitization.
 *
 * DOMPurify strips unknown URI schemes from attributes (e.g. `src="emoji:🦊"` → `src=""`).
 * Converting them to inline spans first preserves the emoji content through sanitization.
 *
 * Converts:
 *   - `<img src="emoji:X" alt="Y">` → `<span role="img" aria-label="Y">X</span>`
 *   - Any remaining `emoji:X` text occurrences → `X`
 */
const resolveEmojiUrisInHtml = (html: string): string => {
  const withEmojiImages: string = html.replace(
    /<img([^>]*)src="(emoji:[^"]+)"([^>]*)\/?>/gi,
    (_match: string, pre: string, src: string, post: string): string => {
      const emoji: string = extractEmojiFromUri(src);
      if (!emoji) {
        return _match;
      }
      const altMatch: RegExpMatchArray | null = /alt="([^"]*)"/i.exec(pre + post);
      const label: string = altMatch ? altMatch[1] : emoji;
      return `<span role="img" aria-label="${label}">${emoji}</span>`;
    },
  );
  return withEmojiImages.replace(/emoji:([^\s"<>&]+)/g, (_: string, rest: string): string =>
    isEmojiUri(`emoji:${rest}`) ? rest : `emoji:${rest}`,
  );
};

type TranslationFn = (key: string, params?: Record<string, string | number>) => string;

/**
 * Get the appropriate FieldType for an input component.
 */
const getFieldType = (variant: EmbeddedFlowComponentType): FieldType => {
  switch (variant) {
    case EmbeddedFlowComponentType.EmailInput:
      return FieldType.Email;
    case EmbeddedFlowComponentType.PasswordInput:
      return FieldType.Password;
    case EmbeddedFlowComponentType.TextInput:
    default:
      return FieldType.Text;
  }
};

/**
 * Get typography variant from component variant.
 */
const getTypographyVariant = (variant: string): any => {
  const variantMap: Record<string, string> = {
    BODY_1: 'body1',
    BODY_2: 'body2',
    BUTTON_TEXT: 'body2',
    CAPTION: 'caption',
    HEADING_1: 'h1',
    HEADING_2: 'h2',
    HEADING_3: 'h3',
    HEADING_4: 'h4',
    HEADING_5: 'h5',
    HEADING_6: 'h6',
    OVERLINE: 'overline',
    SUBTITLE_1: 'subtitle1',
    SUBTITLE_2: 'subtitle2',
  } as Record<EmbeddedFlowTextVariant, string>;

  return variantMap[variant] || 'h3';
};

/**
 * Check if a button text or action matches a social provider.
 */
const matchesSocialProvider = (actionId: string, eventType: string, buttonText: string, provider: string): boolean => {
  const providerId = `${provider}_auth`;
  const providerMatches: boolean = actionId === providerId || eventType === providerId;

  if (buttonText.toLowerCase().includes(provider)) {
    return true;
  }

  return providerMatches;
};

/**
 * Create an auth component (VNode) from a flow component configuration.
 */
const createAuthComponentFromFlow = (
  component: EmbeddedFlowComponent,
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options: {
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inStack?: boolean;
    inputClassName?: string;
    isTimeoutDisabled?: boolean;
    key?: string | number;
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    t?: TranslationFn;
    variant?: any;
  } = {},
): VNode | null => {
  const key: string | number = options.key ?? component.id;

  /** Resolve any remaining {{t()}} or {{meta()}} template expressions in a string at render time. */
  const resolve = (text: string | undefined): string => {
    if (!text || (!options.t && !options.meta)) {
      return text || '';
    }
    return resolveFlowTemplateLiterals(text, {meta: options.meta, t: options.t || ((k: string): string => k)});
  };

  switch (component.type) {
    case EmbeddedFlowComponentType.TextInput:
    case EmbeddedFlowComponentType.PasswordInput:
    case EmbeddedFlowComponentType.EmailInput: {
      const identifier: string = component.ref;
      const value: string = formValues[identifier] || '';
      const isTouched: boolean = touchedFields[identifier] || false;
      const error: string | undefined = isTouched ? formErrors[identifier] : undefined;
      const fieldType: FieldType = getFieldType(component.type);

      return createField({
        className: options.inputClassName,
        error,
        label: resolve(component.label) || '',
        name: identifier,
        onBlur: () => options.onInputBlur?.(identifier),
        onChange: (newValue: string) => onInputChange(identifier, newValue),
        placeholder: resolve(component.placeholder) || '',
        required: component.required || false,
        type: fieldType,
        value,
      });
    }

    case EmbeddedFlowComponentType.Action: {
      const actionId: string = component.id;
      const eventType: string = component.eventType || '';
      const buttonText: string = resolve(component.label);
      const componentVariant: string = component.variant || '';

      const shouldSkipValidation: boolean = eventType.toUpperCase() === EmbeddedFlowEventType.Trigger;

      const handleClick = (): void => {
        if (options.onSubmit) {
          const formData: Record<string, any> = {};
          Object.keys(formValues).forEach((field: string) => {
            formData[field] = formValues[field];
          });

          const consentPrompt: ConsentPromptData | undefined = options.additionalData?.['consentPrompt'] as
            | ConsentPromptData
            | undefined;
          if (consentPrompt && eventType.toUpperCase() === EmbeddedFlowEventType.Submit) {
            const isDeny: boolean = componentVariant.toLowerCase() !== 'primary';
            const decisions: ConsentDecisions = {
              purposes: consentPrompt.purposes.map(
                (p: ConsentPurposeData): ConsentPurposeDecision => ({
                  approved: !isDeny,
                  elements: [
                    ...p.essential.map((e): ConsentAttributeElement => ({approved: !isDeny, name: e.name})),
                    ...p.optional.map(
                      (e): ConsentAttributeElement => ({
                        approved: isDeny ? false : formValues[getConsentOptionalKey(p.purposeId, e.name)] !== 'false',
                        name: e.name,
                      }),
                    ),
                  ],
                  purposeName: p.purposeName,
                }),
              ),
            };
            formData['consent_decisions'] = JSON.stringify(decisions);
          }

          options.onSubmit(component, formData, shouldSkipValidation);
        }
      };

      // Render branded social login buttons for known action IDs
      if (matchesSocialProvider(actionId, eventType, buttonText, 'google')) {
        return h(GoogleButton, {class: options.buttonClassName, key, onClick: handleClick});
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'github')) {
        return h(GitHubButton, {class: options.buttonClassName, key, onClick: handleClick});
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'facebook')) {
        return h(FacebookButton, {class: options.buttonClassName, key, onClick: handleClick});
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'microsoft')) {
        return h(MicrosoftButton, {class: options.buttonClassName, key, onClick: handleClick});
      }

      // Generic button for other providers / actions
      const startIconVNode: VNode | null = component.startIcon
        ? h('img', {
            alt: '',
            'aria-hidden': 'true',
            src: component.startIcon,
            style: {height: '1.25em', objectFit: 'contain', width: '1.25em'},
          })
        : null;

      const endIconVNode: VNode | null = component.endIcon
        ? h('img', {
            alt: '',
            'aria-hidden': 'true',
            src: component.endIcon,
            style: {height: '1.25em', objectFit: 'contain', width: '1.25em'},
          })
        : null;

      return h(
        Button,
        {
          class: options.buttonClassName,
          color: component.variant?.toLowerCase() === 'primary' ? 'primary' : 'secondary',
          'data-testid': 'thunderid-signin-submit',
          disabled:
            isLoading ||
            (!isFormValid && !shouldSkipValidation) ||
            options.isTimeoutDisabled ||
            (component as any).config?.disabled,
          endIcon: endIconVNode ?? undefined,
          fullWidth: true,
          key,
          onClick: handleClick,
          startIcon: startIconVNode ?? undefined,
          variant: component.variant?.toLowerCase() === 'primary' ? 'solid' : 'outline',
        },
        {default: () => buttonText || 'Submit'},
      );
    }

    case EmbeddedFlowComponentType.Text: {
      const variant: any = getTypographyVariant(component.variant);
      const align: string = typeof (component as any).align === 'string' ? (component as any).align : 'left';

      return h(
        Typography,
        {
          key,
          style: {marginBottom: '0.5rem', textAlign: align},
          variant,
        },
        {default: () => resolve(component.label)},
      );
    }

    case EmbeddedFlowComponentType.Divider: {
      const dividerLabel: string = resolve(component.label) || '';
      return h(Divider, {key}, dividerLabel ? {default: () => dividerLabel} : undefined);
    }

    case EmbeddedFlowComponentType.Select: {
      const identifier: string = component.ref;
      const value: string = formValues[identifier] || '';
      const isTouched: boolean = touchedFields[identifier] || false;
      const error: string | undefined = isTouched ? formErrors[identifier] : undefined;

      const selectOptions: {label: string; value: string}[] = ((component as any).options || []).map((opt: any) => ({
        label: typeof opt === 'string' ? opt : String(opt.label ?? opt.value ?? ''),
        value: typeof opt === 'string' ? opt : String(opt.value ?? ''),
      }));

      return h(Select, {
        class: options.inputClassName,
        error,
        key,
        label: resolve(component.label) || '',
        modelValue: value,
        name: identifier,
        onBlur: () => options.onInputBlur?.(identifier),
        'onUpdate:modelValue': (val: string) => onInputChange(identifier, val),
        options: selectOptions,
        placeholder: resolve(component.placeholder),
        required: component.required,
      });
    }

    case EmbeddedFlowComponentType.Block: {
      if (component.components && component.components.length > 0) {
        const blockChildren: (VNode | null)[] = component.components
          .map((childComponent: any, index: number) =>
            createAuthComponentFromFlow(
              childComponent,
              formValues,
              touchedFields,
              formErrors,
              isLoading,
              isFormValid,
              onInputChange,
              {
                ...options,
                key: childComponent.id || `${component.id}_${index}`,
              },
            ),
          )
          .filter(Boolean);

        return h('form', {id: component.id, key}, blockChildren);
      }
      return null;
    }

    case EmbeddedFlowComponentType.RichText: {
      // NOTE: Content comes from ThunderID's own servers (server-driven UI).
      // Emoji URIs are resolved first because DOMPurify strips unknown URI schemes.
      // Manually sanitizes with `DOMPurify` before setting innerHTML (defense-in-depth).
      return h('div', {
        innerHTML: DOMPurify.sanitize(resolveEmojiUrisInHtml(resolve(component.label))),
        key,
        style: {overflowWrap: 'anywhere'},
      });
    }

    case EmbeddedFlowComponentType.Image: {
      const explicitHeight: string = resolve((component as any).height?.toString());
      const explicitWidth: string = resolve((component as any).width?.toString());
      return h('img', {
        alt: resolve((component as any).alt) || resolve(component.label) || 'Image',
        key,
        src: resolve((component as any).src),
        style: {
          height: explicitHeight || (options.inStack ? '50px' : 'auto'),
          objectFit: 'contain',
          width: explicitWidth || (options.inStack ? '50px' : '100%'),
        },
      });
    }

    case EmbeddedFlowComponentType.Icon: {
      // Flow icon registry is not yet available in the Vue SDK.
      logger.warn(`Icon component type is not yet supported in the Vue SDK. Skipping render.`);
      return null;
    }

    case EmbeddedFlowComponentType.Stack: {
      const direction: string = (component as any).direction || 'row';
      const gap: number = (component as any).gap ?? 2;
      const align: string = (component as any).align || 'center';
      const justify: string = (component as any).justify || 'flex-start';

      const stackStyle: Record<string, string> = {
        alignItems: align,
        display: 'flex',
        flexDirection: direction,
        flexWrap: 'wrap',
        gap: `${gap * 0.5}rem`,
        justifyContent: justify,
      };

      const stackChildren: (VNode | null)[] = component.components
        ? component.components.map((childComponent: any, index: number) =>
            createAuthComponentFromFlow(
              childComponent,
              formValues,
              touchedFields,
              formErrors,
              isLoading,
              isFormValid,
              onInputChange,
              {
                ...options,
                inStack: true,
                key: childComponent.id || `${component.id}_${index}`,
              },
            ),
          )
        : [];

      return h('div', {key, style: stackStyle}, stackChildren.filter(Boolean));
    }

    case EmbeddedFlowComponentType.Consent: {
      // Consent component is not yet implemented in the Vue SDK.
      logger.warn(`Consent component type is not yet fully supported in the Vue SDK.`);
      return null;
    }

    case EmbeddedFlowComponentType.Timer: {
      const textTemplate: string = resolve((component as any).label) || 'Time remaining: {time}';
      const timeoutMs: number = Number(options.additionalData?.['stepTimeout']) || 0;
      const expiresIn: number = timeoutMs > 0 ? Math.max(0, Math.floor((timeoutMs - Date.now()) / 1000)) : 0;
      const timerText: string = textTemplate.replace('{time}', String(expiresIn));

      return h('div', {class: 'thunderid-flow-timer', key}, timerText);
    }

    default:
      logger.warn(`Unsupported component type: ${(component as any).type}. Skipping render.`);
      return null;
  }
};

export type {TranslationFn};

/**
 * Processes an array of components and renders them as VNodes for sign-in.
 */
export const renderSignInComponents = (
  components: EmbeddedFlowComponent[],
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options?: {
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inputClassName?: string;
    isTimeoutDisabled?: boolean;
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    t?: TranslationFn;
    variant?: any;
  },
): VNode[] =>
  components
    .map((component: any, index: number) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter(Boolean);

/**
 * Processes an array of components and renders them as VNodes for sign-up.
 * Identical to renderSignInComponents — separated for semantic clarity.
 */
export const renderSignUpComponents = (
  components: EmbeddedFlowComponent[],
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options?: {
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inputClassName?: string;
    isTimeoutDisabled?: boolean;
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    t?: TranslationFn;
    variant?: any;
  },
): VNode[] =>
  components
    .map((component: any, index: number) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter(Boolean);

/**
 * Processes an array of components and renders them as VNodes for invite-user flows.
 * Identical to renderSignInComponents — separated for semantic clarity.
 */
export const renderInviteUserComponents = (
  components: EmbeddedFlowComponent[],
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options?: {
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inputClassName?: string;
    isTimeoutDisabled?: boolean;
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    t?: TranslationFn;
    variant?: any;
  },
): VNode[] =>
  components
    .map((component: any, index: number) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter(Boolean);
