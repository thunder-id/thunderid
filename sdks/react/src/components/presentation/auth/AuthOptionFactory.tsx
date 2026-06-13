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

import {css} from '@emotion/css';
import {
  FieldType,
  FlowMetadataResponse,
  OrganizationUnitListResponse,
  EmbeddedFlowComponentV2 as EmbeddedFlowComponent,
  EmbeddedFlowComponentTypeV2 as EmbeddedFlowComponentType,
  EmbeddedFlowTextVariantV2 as EmbeddedFlowTextVariant,
  EmbeddedFlowEventTypeV2 as EmbeddedFlowEventType,
  createPackageComponentLogger,
  resolveFlowTemplateLiterals,
  resolveEmojiUrisInHtml,
  ConsentPurposeDataV2 as ConsentPurposeData,
  ConsentPromptDataV2 as ConsentPromptData,
  ConsentDecisionsV2 as ConsentDecisions,
  ConsentPurposeDecisionV2 as ConsentPurposeDecision,
  ConsentAttributeElementV2 as ConsentAttributeElement,
} from '@thunderid/browser';
import DOMPurify from 'dompurify';
import {cloneElement, CSSProperties, ReactElement} from 'react';
import {OrganizationUnitPicker} from './OrganizationUnitPicker';
import {
  ComponentRenderer,
  ComponentRenderContext,
  ComponentRendererMap,
} from '../../../contexts/ComponentRenderer/ComponentRendererContext';
import {UseTranslation} from '../../../hooks/useTranslation';
import Consent from '../../adapters/Consent';
import {getConsentOptionalKey} from '../../adapters/ConsentCheckboxList';
import FacebookButton from '../../adapters/FacebookButton';
import FlowTimer from '../../adapters/FlowTimer';
import GitHubButton from '../../adapters/GitHubButton';
import GoogleButton from '../../adapters/GoogleButton';
import ImageComponent from '../../adapters/ImageComponent';
import LinkedInButton from '../../adapters/LinkedInButton';
import MicrosoftButton from '../../adapters/MicrosoftButton';
import SignInWithEthereumButton from '../../adapters/SignInWithEthereumButton';
import SmsOtpButton from '../../adapters/SmsOtpButton';
import {createField} from '../../factories/FieldFactory';
import Button from '../../primitives/Button/Button';
import CopyableText from '../../primitives/CopyableText/CopyableText';
import DatePicker from '../../primitives/DatePicker/DatePicker';
import Divider from '../../primitives/Divider/Divider';
import flowIconRegistry from '../../primitives/Icons/flowIconRegistry';
import Select from '../../primitives/Select/Select';
import Typography from '../../primitives/Typography/Typography';
import {TypographyVariant} from '../../primitives/Typography/Typography.styles';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'AuthOptionFactory',
);

/**
 * Replaces `emoji:` URIs embedded in HTML before DOMPurify sanitization.
 *
 * DOMPurify strips unknown URI schemes from attributes (e.g. `src="emoji:🦊"` → `src=""`).
 * This function converts:
 *   - `<img src="emoji:X" alt="Y">` → `<span role="img" aria-label="Y">X</span>`
 *   - Any remaining `emoji:X` text occurrences → `X`
 */

/** Ensures rich-text content (including all inner elements from the server) always word-wraps. */
const richTextClass: string = css`
  overflow-wrap: anywhere;
  & * {
    overflow-wrap: anywhere;
    word-break: break-word;
  }
  & .rich-text-align-left {
    text-align: left;
  }
  & .rich-text-align-center {
    text-align: center;
  }
  & .rich-text-align-right {
    text-align: right;
  }
  & .rich-text-align-justify {
    text-align: justify;
  }
  & a,
  & .rich-text-link {
    text-decoration: underline;
  }
  & span[role='img'] {
    display: inline-block;
  }
`;

export type AuthType = 'signin' | 'signup' | 'recovery';

/**
 * Get the appropriate FieldType for an input component.
 */
const getFieldType = (variant: EmbeddedFlowComponentType): FieldType => {
  switch (variant) {
    case EmbeddedFlowComponentType.EmailInput:
      return FieldType.Email;
    case EmbeddedFlowComponentType.PhoneInput:
      return FieldType.Tel;
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
  const variantMap: Record<EmbeddedFlowTextVariant, TypographyVariant> = {
    BODY_1: 'body1',
    BODY_2: 'body2',
    BUTTON_TEXT: 'button',
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
  };

  return variantMap[variant] || 'h3';
};

/**
 * Check if a button text or action matches a social provider.
 */
const matchesSocialProvider = (
  actionId: string,
  eventType: string,
  buttonText: string,
  provider: string,
  authType: AuthType,
  /* eslint-disable-next-line @typescript-eslint/no-unused-vars */
  _componentVariant?: string,
): boolean => {
  const providerId: any = `${provider}_auth`;
  const providerMatches: any = actionId === providerId || eventType === providerId;

  // Social buttons usually have "Sign in with X" or "Continue with X" text,
  // so also check button text for the provider name to increase chances of correct detection (especially for signup flows where action IDs are less standardized)
  if (buttonText.toLowerCase().includes(provider)) {
    return true;
  }

  // For signup, also check button text
  if (authType === 'signup') {
    return providerMatches || buttonText.toLowerCase().includes(provider);
  }

  return providerMatches;
};

/**
 * Create an auth component from flow component configuration.
 */
const createAuthComponentFromFlow = (
  component: EmbeddedFlowComponent,
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  authType: AuthType,
  options: {
    /** @internal Passed from the host React component so hooks aren't called inside a loop. */
    _customRenderers?: ComponentRendererMap;
    /** @internal Passed from the host React component so hooks aren't called inside a loop. */
    _theme?: any;
    /** Additional data from the flow response, used for different dynamic data */
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    /** Current consent purpose being rendered. Set by CONSENT_PURPOSE block iteration. */
    currentConsentPurpose?: ConsentPurposeData;
    /** Function to fetch child organization units. Used by OU_SELECT component type. */
    fetchOrganizationUnitChildren?: (
      parentId: string,
      limit: number,
      offset: number,
    ) => Promise<OrganizationUnitListResponse>;
    inStack?: boolean;
    inputClassName?: string;
    /** Flag to determine if the step timeline has expired */
    isTimeoutDisabled?: boolean;
    key?: string | number;
    /** Flow metadata for resolving {{meta(...)}} expressions at render time */
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    /** Translation function for resolving {{t(...)}} expressions at render time */
    t?: UseTranslation['t'];
    variant?: any;
  } = {},
): ReactElement | null => {
  const theme: any = options._theme;
  const customRenderers: ComponentRendererMap = options._customRenderers ?? {};

  const key: string | number = options.key || component.id;

  const customRenderer: ComponentRenderer | undefined =
    customRenderers[component.id] ?? customRenderers[component.type];
  if (customRenderer) {
    const renderCtx: ComponentRenderContext = {
      additionalData: options.additionalData,
      authType,
      formErrors,
      formValues,
      isFormValid,
      isLoading,
      meta: options.meta,
      onInputBlur: options.onInputBlur,
      onInputChange,
      onSubmit: options.onSubmit,
      touchedFields,
    };
    return customRenderer(component, renderCtx);
  }

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
    case EmbeddedFlowComponentType.EmailInput:
    case EmbeddedFlowComponentType.PhoneInput: {
      const identifier: string = component.ref!;
      const value: string = formValues[identifier] || '';
      const isTouched: boolean = touchedFields[identifier] || false;
      const error: string = isTouched ? formErrors[identifier] : undefined!;
      const fieldType: string = getFieldType(component.type);

      const field: any = createField({
        className: options.inputClassName,
        error,
        label: resolve(component.label) || '',
        name: identifier,
        onBlur: () => options.onInputBlur?.(identifier),
        onChange: (newValue: string) => onInputChange(identifier, newValue),
        placeholder: resolve(component.placeholder) || '',
        required: component.required || false,
        type: fieldType as FieldType,
        value,
      });

      return cloneElement(field, {key});
    }

    case EmbeddedFlowComponentType.OtpInput: {
      const identifier: string = component.ref!;
      const value: string = formValues[identifier] || '';
      const isTouched: boolean = touchedFields[identifier] || false;
      const error: string = isTouched ? formErrors[identifier] : undefined!;

      const field: any = createField({
        className: options.inputClassName,
        error,
        label: resolve(component.label) || '',
        name: identifier,
        onBlur: () => options.onInputBlur?.(identifier),
        onChange: (newValue: string) => onInputChange(identifier, newValue),
        placeholder: resolve(component.placeholder) || '',
        required: component.required || false,
        type: FieldType.Otp,
        value,
      });

      return cloneElement(field, {key});
    }

    case EmbeddedFlowComponentType.Action: {
      const actionId: string = component.id;
      const eventType: string = component.eventType || '';
      const buttonText: string = resolve(component.label);
      const componentVariant: string = component.variant || '';

      // Only validate on submit type events.
      const shouldSkipValidation: boolean = eventType.toUpperCase() === EmbeddedFlowEventType.Trigger;

      const handleClick = (): any => {
        if (options.onSubmit) {
          const formData: Record<string, any> = {};
          Object.keys(formValues).forEach((field: any) => {
            // Include all values, even empty strings, to ensure proper submission
            formData[field] = formValues[field];
          });

          // For consent actions, build the consent_decisions JSON from the form values.
          // The allow action captures per-attribute choices; the deny action sends
          // all purposes as rejected so the backend ConsentExecutor can fail the flow.
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
                  purposeName: p.purposeName!,
                }),
              ),
            };
            formData['consent_decisions'] = JSON.stringify(decisions);
          }

          options.onSubmit(component, formData, shouldSkipValidation);
        }
      };

      // Render branded social login buttons for known action IDs

      if (matchesSocialProvider(actionId, eventType, buttonText, 'google', authType, componentVariant)) {
        return <GoogleButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'github', authType, componentVariant)) {
        return <GitHubButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'facebook', authType, componentVariant)) {
        return <FacebookButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'microsoft', authType, componentVariant)) {
        return <MicrosoftButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'linkedin', authType, componentVariant)) {
        return <LinkedInButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }
      if (matchesSocialProvider(actionId, eventType, buttonText, 'ethereum', authType, componentVariant)) {
        return <SignInWithEthereumButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }
      if (actionId === 'prompt_mobile' || eventType === 'prompt_mobile') {
        return <SmsOtpButton key={key} onClick={handleClick} className={options.buttonClassName} />;
      }

      const startIconEl: ReactElement | null = component.startIcon ? (
        <img
          src={component.startIcon}
          alt=""
          aria-hidden="true"
          style={{height: '1.25em', objectFit: 'contain', width: '1.25em'}}
        />
      ) : null;

      const endIconEl: ReactElement | null = component.endIcon ? (
        <img
          src={component.endIcon}
          alt=""
          aria-hidden="true"
          style={{height: '1.25em', objectFit: 'contain', width: '1.25em'}}
        />
      ) : null;

      return (
        <Button
          fullWidth
          key={key}
          onClick={handleClick}
          disabled={
            isLoading ||
            (!isFormValid && !shouldSkipValidation) ||
            options.isTimeoutDisabled ||
            (component as any).config?.disabled
          }
          className={options.buttonClassName}
          data-testid="thunderid-signin-submit"
          variant={component.variant?.toLowerCase() === 'primary' ? 'solid' : 'outline'}
          color={component.variant?.toLowerCase() === 'primary' ? 'primary' : 'secondary'}
          startIcon={startIconEl}
          endIcon={endIconEl}
        >
          {buttonText || 'Submit'}
        </Button>
      );
    }

    case EmbeddedFlowComponentType.Text: {
      const variant: any = getTypographyVariant(component.variant!);
      return (
        <Typography
          key={key}
          variant={variant}
          style={{
            marginBottom: 2,
            textAlign:
              typeof component?.align === 'string' ? (component.align as React.CSSProperties['textAlign']) : 'left',
          }}
        >
          {resolve(component.label)}
        </Typography>
      );
    }

    case EmbeddedFlowComponentType.Divider: {
      return <Divider key={key}>{resolve(component.label) || ''}</Divider>;
    }

    case EmbeddedFlowComponentType.Select: {
      const identifier: string = component.ref!;
      const value: string = formValues[identifier] || '';
      const isTouched: boolean = touchedFields[identifier] || false;
      const error: string = isTouched ? formErrors[identifier] : undefined!;

      // Options are pre-sanitized by flowTransformer to {value: string, label: string} format
      const selectOptions: any = (component.options || []).map((opt: any) => ({
        label: typeof opt === 'string' ? opt : String(opt.label ?? opt.value ?? ''),
        value: typeof opt === 'string' ? opt : String(opt.value ?? ''),
      }));

      return (
        <Select
          key={key}
          name={identifier}
          label={resolve(component.label) || ''}
          placeholder={resolve(component.placeholder)}
          required={component.required}
          options={selectOptions}
          value={value}
          error={error}
          onChange={(e: any): void => onInputChange(identifier, e.target.value)}
          onBlur={(): any => options.onInputBlur?.(identifier)}
          className={options.inputClassName}
        />
      );
    }

    case EmbeddedFlowComponentType.DateInput: {
      const identifier: string = component.ref!;
      const value: string = formValues[identifier] || '';
      const isTouched: boolean = touchedFields[identifier] || false;
      const error: string = isTouched ? formErrors[identifier] : undefined!;

      return (
        <DatePicker
          key={key}
          name={identifier}
          label={resolve(component.label) || ''}
          placeholder={resolve(component.placeholder)}
          required={component.required}
          dateFormat={component.dateFormat}
          value={value}
          error={error}
          onChange={(e: any): void => onInputChange(identifier, e.target.value)}
          onBlur={(): any => options.onInputBlur?.(identifier)}
          className={options.inputClassName}
        />
      );
    }

    case EmbeddedFlowComponentType.OuSelect: {
      const identifier: string = component.ref ?? component.id;
      const rootOuId: string | undefined = options.additionalData?.['rootOuId'] as string | undefined;

      if (!rootOuId || !options.fetchOrganizationUnitChildren) {
        logger.warn('OU_SELECT requires additionalData.rootOuId and fetchOrganizationUnitChildren. Skipping render.');
        return null;
      }

      return (
        <OrganizationUnitPicker
          key={key}
          rootOuId={rootOuId}
          selectedOuId={formValues[identifier] || null}
          onSelect={(ouId: string) => onInputChange(identifier, ouId)}
          fetchChildren={options.fetchOrganizationUnitChildren}
        />
      );
    }

    case EmbeddedFlowComponentType.Block: {
      if (component.components && component.components.length > 0) {
        const formStyles: CSSProperties = {
          display: 'flex',
          flexDirection: 'column',
          gap: `calc(${theme?.vars?.spacing?.unit ?? '4px'} * 2)`,
        };

        const blockComponents: any = component.components
          .map((childComponent: any, index: any) =>
            createAuthComponentFromFlow(
              childComponent,
              formValues,
              touchedFields,
              formErrors,
              isLoading,
              isFormValid,
              onInputChange,
              authType,
              {
                ...options,
                key: childComponent.id || `${component.id}_${index}`,
              },
            ),
          )
          .filter(Boolean);

        return (
          <form id={component.id} key={key} style={formStyles}>
            {blockComponents}
          </form>
        );
      }
      return null;
    }

    case EmbeddedFlowComponentType.RichText: {
      return (
        <div
          key={key}
          className={richTextClass}
          // Manually sanitizes with `DOMPurify`.
          // IMPORTANT: DO NOT REMOVE OR MODIFY THIS SANITIZATION STEP.
          dangerouslySetInnerHTML={{__html: DOMPurify.sanitize(resolveEmojiUrisInHtml(resolve(component.label)))}}
        />
      );
    }

    case EmbeddedFlowComponentType.Image: {
      const explicitHeight: string = resolve(component.height?.toString());
      const explicitWidth: string = resolve(component.width?.toString());
      return (
        <ImageComponent
          key={key}
          component={
            {
              config: {
                alt: resolve(component.alt) || resolve(component.label) || 'Image',
                height: explicitHeight || (options.inStack ? '50' : 'auto'),
                src: resolve(component.src),
                width: explicitWidth || (options.inStack ? '50' : '100%'),
              },
            } as any
          }
          formErrors={undefined!}
          formValues={undefined!}
          isFormValid={false}
          isLoading={false}
          onInputChange={(): void => {
            throw new Error('Function not implemented.');
          }}
          touchedFields={undefined!}
        />
      );
    }

    case EmbeddedFlowComponentType.Icon: {
      const iconName: string = component.name || '';
      const IconComponent: any = flowIconRegistry[iconName];
      if (!IconComponent) {
        logger.warn(`Unknown icon name: "${iconName}". Skipping render.`);
        return null;
      }
      return <IconComponent key={key} size={component.size || 24} color={component.color || 'currentColor'} />;
    }

    case EmbeddedFlowComponentType.Stack: {
      const direction: string = (component as any).direction || 'row';
      const gap: number = (component as any).gap ?? 2;
      const align: string = (component as any).align || 'center';
      const justify: string = (component as any).justify || 'flex-start';

      const stackStyle: CSSProperties = {
        alignItems: align,
        display: 'flex',
        flexDirection: direction as CSSProperties['flexDirection'],
        flexWrap: 'wrap',
        gap: `${gap * 0.5}rem`,
        justifyContent: justify,
      };

      const stackChildren: (ReactElement | null)[] = component.components
        ? component.components.map((childComponent: any, index: number) =>
            createAuthComponentFromFlow(
              childComponent,
              formValues,
              touchedFields,
              formErrors,
              isLoading,
              isFormValid,
              onInputChange,
              authType,
              {
                ...options,
                inStack: true,
                key: childComponent.id || `${component.id}_${index}`,
              },
            ),
          )
        : [];

      return (
        <div key={key} style={stackStyle}>
          {stackChildren}
        </div>
      );
    }

    case EmbeddedFlowComponentType.Consent: {
      const consentPromptRawData: ConsentPromptData | string | undefined = options.additionalData?.['consentPrompt'];

      return (
        <Consent
          key={key}
          consentData={consentPromptRawData as any}
          formValues={formValues}
          onInputChange={onInputChange}
        />
      );
    }

    case EmbeddedFlowComponentType.Timer: {
      const textTemplate: string = resolve((component as any).label) || 'Time remaining: {time}';
      const timeoutMs: number = Number(options.additionalData?.['stepTimeout']) || 0;
      const expiresIn: number = timeoutMs > 0 ? Math.max(0, Math.floor((timeoutMs - Date.now()) / 1000)) : 0;

      return <FlowTimer key={key} expiresIn={expiresIn} textTemplate={textTemplate} />;
    }

    case EmbeddedFlowComponentType.CopyableText: {
      const sourceKey: string | undefined = (component as any).source;
      const value: string = sourceKey && options.additionalData ? String(options.additionalData[sourceKey] ?? '') : '';
      const labelText: string | undefined = resolve((component as any).label) || undefined;

      return <CopyableText key={key} label={labelText} value={value} />;
    }

    default:
      // Gracefully handle unsupported component types by returning null
      logger.warn(`Unsupported component type: ${component.type}. Skipping render.`);
      return null;
  }
};

/**
 * Processes an array of components and renders them as React elements for sign-in.
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
    /** @internal */
    _customRenderers?: ComponentRendererMap;
    /** @internal */
    _theme?: any;
    /** Additional data from the flow response */
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inputClassName?: string;
    /** Flag to determine if the step timeline has expired */
    isTimeoutDisabled?: boolean;
    /** Flow metadata for resolving {{meta(...)}} expressions at render time */
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    /** Translation function for resolving {{t(...)}} expressions at render time */
    t?: UseTranslation['t'];
    variant?: any;
  },
): ReactElement[] =>
  components
    .map((component: any, index: any) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        'signin',
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter((x): x is ReactElement => x !== null);

/**
 * Processes an array of components and renders them as React elements for sign-up.
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
    /** @internal */
    _customRenderers?: ComponentRendererMap;
    /** @internal */
    _theme?: any;
    /** Additional data from the flow response */
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inputClassName?: string;
    /** Flow metadata for resolving {{meta(...)}} expressions at render time */
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    /** Translation function for resolving {{t(...)}} expressions at render time */
    t?: UseTranslation['t'];
    variant?: any;
  },
): ReactElement[] =>
  components
    .map((component: any, index: any) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        'signup',
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter((x): x is ReactElement => x !== null);

/**
 * Processes an array of components and renders them as React elements for recovery flow.
 */
export const renderRecoveryComponents = (
  components: EmbeddedFlowComponent[],
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options?: {
    /** @internal */
    _customRenderers?: ComponentRendererMap;
    /** @internal */
    _theme?: any;
    /** Additional data from the flow response */
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    inputClassName?: string;
    /** Flag to determine if the step timeline has expired */
    isTimeoutDisabled?: boolean;
    /** Flow metadata for resolving {{meta(...)}} expressions at render time */
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    /** Translation function for resolving {{t(...)}} expressions at render time */
    t?: UseTranslation['t'];
    variant?: any;
  },
): ReactElement[] =>
  components
    .map((component: any, index: any) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        'recovery',
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter((x): x is ReactElement => x !== null);

/**
 * Processes an array of components and renders them as React elements for invite user.
 * This is used by both InviteUser and AcceptInvite components.
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
    /** @internal */
    _customRenderers?: ComponentRendererMap;
    /** @internal */
    _theme?: any;
    /** Additional data from the flow response */
    additionalData?: Record<string, any>;
    buttonClassName?: string;
    /** Function to fetch child organization units. Used by OU_SELECT component type. */
    fetchOrganizationUnitChildren?: (
      parentId: string,
      limit: number,
      offset: number,
    ) => Promise<OrganizationUnitListResponse>;
    inputClassName?: string;
    /** Flow metadata for resolving {{meta(...)}} expressions at render time */
    meta?: FlowMetadataResponse | null;
    onInputBlur?: (name: string) => void;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>, skipValidation?: boolean) => void;
    size?: 'small' | 'medium' | 'large';
    /** Translation function for resolving {{t(...)}} expressions at render time */
    t?: UseTranslation['t'];
    variant?: any;
  },
): ReactElement[] =>
  components
    .map((component: any, index: any) =>
      createAuthComponentFromFlow(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        'signup',
        {
          ...options,
          key: component.id || index,
        },
      ),
    )
    .filter((x): x is ReactElement => x !== null);
