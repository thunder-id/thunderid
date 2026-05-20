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

import {FlowMetadataResponse, withVendorCSSClassPrefix} from '@thunderid/browser';
import {
  type Component,
  type PropType,
  type Ref,
  type SetupContext,
  type VNode,
  defineComponent,
  h,
  ref,
  watch,
} from 'vue';
import useFlowMeta from '../../../composables/useFlowMeta';
import useI18n from '../../../composables/useI18n';
import {useOAuthCallback} from '../../../composables/useOAuthCallback';
import {initiateOAuthRedirect} from '../../../utils/oauth';
import {extractErrorMessage, normalizeFlowResponse} from '../../../utils/v2/flowTransformer';
import {renderInviteUserComponents} from '../../auth/sign-in/AuthOptionFactory';
import Alert from '../../primitives/Alert';
import Button from '../../primitives/Button';
import Card from '../../primitives/Card';
import Spinner from '../../primitives/Spinner';
import Typography from '../../primitives/Typography';

/**
 * Flow response from the accept-invite backend.
 */
export interface AcceptInviteFlowResponse {
  data?: {
    additionalData?: Record<string, string>;
    components?: any[];
    meta?: {
      components?: any[];
    };
    redirectURL?: string;
  };
  failureReason?: string;
  flowId: string;
  flowStatus: 'INCOMPLETE' | 'COMPLETE' | 'ERROR';
  type?: 'VIEW' | 'REDIRECTION';
}

/**
 * Render props passed to the default scoped slot.
 */
export interface BaseAcceptInviteRenderProps {
  completionTitle?: string;
  components: any[];
  error?: Error | null;
  fieldErrors: Record<string, string>;
  flowId?: string;
  goToSignIn?: () => void;
  handleInputBlur: (name: string) => void;
  handleInputChange: (name: string, value: string) => void;
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;
  inviteToken?: string;
  isComplete: boolean;
  isLoading: boolean;
  isTokenInvalid: boolean;
  isValid: boolean;
  isValidatingToken: boolean;
  meta: FlowMetadataResponse | null;
  subtitle?: string;
  title?: string;
  touched: Record<string, boolean>;
  values: Record<string, string>;
}

export interface BaseAcceptInviteProps {
  className?: string;
  flowId?: string;
  inviteToken?: string;
  onComplete?: () => void;
  onError?: (error: Error) => void;
  onFlowChange?: (response: AcceptInviteFlowResponse) => void;
  onGoToSignIn?: () => void;
  onSubmit: (payload: Record<string, any>) => Promise<AcceptInviteFlowResponse>;
  showSubtitle?: boolean;
  showTitle?: boolean;
  size?: 'small' | 'medium' | 'large';
  variant?: 'outlined' | 'elevated';
}

/**
 * BaseAcceptInvite — handles the accept-invite flow lifecycle.
 *
 * Steps: validate invite token → render password form → flow completion.
 */
const BaseAcceptInvite: Component = defineComponent({
  name: 'BaseAcceptInvite',
  props: {
    className: {default: '', type: String},
    flowId: {default: undefined, type: String},
    inviteToken: {default: undefined, type: String},
    onComplete: {default: undefined, type: Function as PropType<() => void>},
    onError: {default: undefined, type: Function as PropType<(error: Error) => void>},
    onFlowChange: {
      default: undefined,
      type: Function as PropType<(response: AcceptInviteFlowResponse) => void>,
    },
    onGoToSignIn: {default: undefined, type: Function as PropType<() => void>},
    onSubmit: {
      required: true,
      type: Function as PropType<(payload: Record<string, any>) => Promise<AcceptInviteFlowResponse>>,
    },
    showSubtitle: {default: true, type: Boolean},
    showTitle: {default: true, type: Boolean},
    size: {default: 'medium', type: String as PropType<'small' | 'medium' | 'large'>},
    variant: {default: 'outlined', type: String as PropType<'outlined' | 'elevated'>},
  },
  setup(props: any, {slots}: SetupContext): () => VNode | null {
    const {meta: metaRef} = useFlowMeta();
    const {t} = useI18n();

    // ── State ──
    const isLoading: Ref<boolean> = ref(false);
    const isValidatingToken: Ref<boolean> = ref(true);
    const isTokenInvalid: Ref<boolean> = ref(false);
    const isComplete: Ref<boolean> = ref(false);
    const currentFlow: Ref<AcceptInviteFlowResponse | null> = ref(null);
    const apiError: Ref<Error | null> = ref(null);
    const completionTitle: Ref<string | undefined> = ref(undefined);

    // Form state
    const formValues: Ref<Record<string, string>> = ref({});
    const formErrors: Ref<Record<string, string>> = ref({});
    const touchedFields: Ref<Record<string, boolean>> = ref({});
    const isFormValid: Ref<boolean> = ref(true);

    let tokenValidationAttempted = false;

    // ── Helpers ──

    const handleError = (error: any): void => {
      const errorMessage: string =
        error?.failureReason || extractErrorMessage(error, t, 'components.acceptInvite.errors.generic');
      apiError.value = error instanceof Error ? error : new Error(errorMessage);
      props.onError?.(apiError.value);
    };

    const normalizeFlowResponseLocal = (response: AcceptInviteFlowResponse): AcceptInviteFlowResponse => {
      if (!response?.data?.meta?.components) return response;
      try {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {defaultErrorKey: 'components.acceptInvite.errors.generic', resolveTranslations: false},
          (metaRef as Ref<FlowMetadataResponse | null>).value,
        );
        return {...response, data: {...response.data, components: components as any}};
      } catch {
        return response;
      }
    };

    // ── OAuth callback ──

    useOAuthCallback({
      currentFlowId: ref(props.flowId ?? null),
      isInitialized: ref(true),
      onComplete: () => {
        isComplete.value = true;
        isValidatingToken.value = false;
        props.onComplete?.();
      },
      onError: (error: any) => {
        isTokenInvalid.value = true;
        isValidatingToken.value = false;
        handleError(error);
      },
      onFlowChange: (response: any) => {
        props.onFlowChange?.(response);
        if (response.flowStatus !== 'COMPLETE') {
          currentFlow.value = response;
          formValues.value = {};
          formErrors.value = {};
          touchedFields.value = {};
        }
      },
      onProcessingStart: () => {
        isValidatingToken.value = true;
      },
      onSubmit: async (payload: any) => {
        const rawResponse: any = await props.onSubmit(payload);
        return normalizeFlowResponseLocal(rawResponse);
      },
      tokenValidationAttemptedFlag: {value: tokenValidationAttempted},
    });

    // ── Input handlers ──

    const handleInputChange = (name: string, value: string): void => {
      formValues.value = {...formValues.value, [name]: value};
      const newErrors: Record<string, string> = {...formErrors.value};
      delete newErrors[name];
      formErrors.value = newErrors;
    };

    const handleInputBlur = (name: string): void => {
      touchedFields.value = {...touchedFields.value, [name]: true};
    };

    // ── Validation ──

    const validateForm = (components: any[]): {errors: Record<string, string>; isValid: boolean} => {
      const errors: Record<string, string> = {};
      const validateComponents = (comps: any[]): void => {
        comps.forEach((comp: any) => {
          if (
            (comp.type === 'PASSWORD_INPUT' || comp.type === 'TEXT_INPUT' || comp.type === 'EMAIL_INPUT') &&
            comp.required &&
            comp.ref
          ) {
            const value: string = formValues.value[comp.ref] || '';
            if (!value || value.trim() === '') {
              errors[comp.ref] = `${comp.label || comp.ref} is required`;
            }
          }
          if (comp.components && Array.isArray(comp.components)) {
            validateComponents(comp.components);
          }
        });
      };
      validateComponents(components);
      return {errors, isValid: Object.keys(errors).length === 0};
    };

    // ── Submit handler ──

    const handleSubmit = async (component: any, data?: Record<string, any>): Promise<void> => {
      if (!currentFlow.value) return;

      const components: any[] = currentFlow.value.data?.components || [];
      const validation: {errors: Record<string, string>; isValid: boolean} = validateForm(components);
      if (!validation.isValid) {
        formErrors.value = validation.errors;
        isFormValid.value = false;
        const touched: Record<string, boolean> = {};
        Object.keys(validation.errors).forEach((key: string) => {
          touched[key] = true;
        });
        touchedFields.value = {...touchedFields.value, ...touched};
        return;
      }

      isLoading.value = true;
      apiError.value = null;
      isFormValid.value = true;

      try {
        const inputs: Record<string, any> = data || formValues.value;
        const payload: Record<string, any> = {
          flowId: currentFlow.value.flowId,
          inputs,
          verbose: true,
        };
        if (component?.id) payload['action'] = component.id;

        const rawResponse: AcceptInviteFlowResponse = await props.onSubmit(payload);
        const response: AcceptInviteFlowResponse = normalizeFlowResponseLocal(rawResponse);
        props.onFlowChange?.(response);

        // Handle OAuth redirect
        if (response.type === 'REDIRECTION') {
          const redirectURL: string | undefined = response.data?.redirectURL || (response as any)?.redirectURL;
          if (redirectURL) {
            initiateOAuthRedirect(redirectURL);
            return;
          }
        }

        // Store heading before completion
        if (currentFlow.value?.data?.components || currentFlow.value?.data?.meta?.components) {
          const currentComponents: any[] =
            currentFlow.value.data?.components || currentFlow.value.data?.meta?.components || [];
          const heading: any = currentComponents.find(
            (comp: any) => comp.type === 'TEXT' && comp.variant === 'HEADING_1',
          );
          if (heading?.label) completionTitle.value = heading.label;
        }

        if (response.flowStatus === 'COMPLETE') {
          isComplete.value = true;
          props.onComplete?.();
          return;
        }

        if (response.flowStatus === 'ERROR') {
          handleError(response);
          return;
        }

        currentFlow.value = response;
        formValues.value = {};
        formErrors.value = {};
        touchedFields.value = {};
      } catch (err) {
        handleError(err);
      } finally {
        isLoading.value = false;
      }
    };

    // ── Token validation on mount ──

    watch(
      () => [props.flowId, props.inviteToken] as [string | undefined, string | undefined],
      ([flowId, inviteToken]: [string | undefined, string | undefined]) => {
        if (tokenValidationAttempted) return;

        if (!flowId || !inviteToken) {
          isValidatingToken.value = false;
          isTokenInvalid.value = true;
          handleError(new Error('Invalid invite link. Missing flowId or inviteToken.'));
          return;
        }

        tokenValidationAttempted = true;

        (async (): Promise<void> => {
          isValidatingToken.value = true;
          apiError.value = null;

          try {
            if (flowId) sessionStorage.setItem('thunderid_flow_id', flowId);

            const payload: any = {flowId, inputs: {inviteToken}, verbose: true};
            const rawResponse: AcceptInviteFlowResponse = await props.onSubmit(payload);
            const response: AcceptInviteFlowResponse = normalizeFlowResponseLocal(rawResponse);
            props.onFlowChange?.(response);

            if (response.flowStatus === 'ERROR') {
              isTokenInvalid.value = true;
              handleError(response);
              return;
            }

            currentFlow.value = response;
          } catch (err) {
            isTokenInvalid.value = true;
            handleError(err);
          } finally {
            isValidatingToken.value = false;
          }
        })();
      },
      {immediate: true},
    );

    // ── Heading extraction ──

    const extractHeadings = (components: any[]): {subtitle?: string; title?: string} => {
      let title: string | undefined;
      let subtitle: string | undefined;
      components.forEach((comp: any) => {
        if (comp.type === 'TEXT') {
          if (comp.variant === 'HEADING_1' && !title) title = comp.label;
          else if ((comp.variant === 'HEADING_2' || comp.variant === 'SUBTITLE_1') && !subtitle) subtitle = comp.label;
        }
      });
      return {subtitle, title};
    };

    const filterHeadings = (components: any[]): any[] =>
      components.filter(
        (comp: any) => !(comp.type === 'TEXT' && (comp.variant === 'HEADING_1' || comp.variant === 'HEADING_2')),
      );

    // ── Render ──

    return (): VNode | null => {
      const containerClass: string = [withVendorCSSClassPrefix('accept-invite'), props.className]
        .filter(Boolean)
        .join(' ');

      const components: any[] = currentFlow.value?.data?.components || currentFlow.value?.data?.meta?.components || [];
      const {title, subtitle} = extractHeadings(components);
      const componentsWithoutHeadings: any[] = filterHeadings(components);

      const meta: FlowMetadataResponse | null = (metaRef as Ref<FlowMetadataResponse | null>).value;

      // Scoped slot
      if (slots['default']) {
        const renderProps: BaseAcceptInviteRenderProps = {
          completionTitle: completionTitle.value,
          components,
          error: apiError.value,
          fieldErrors: formErrors.value,
          flowId: props.flowId,
          goToSignIn: props.onGoToSignIn,
          handleInputBlur,
          handleInputChange,
          handleSubmit,
          inviteToken: props.inviteToken,
          isComplete: isComplete.value,
          isLoading: isLoading.value,
          isTokenInvalid: isTokenInvalid.value,
          isValid: isFormValid.value,
          isValidatingToken: isValidatingToken.value,
          meta,
          subtitle,
          title,
          touched: touchedFields.value,
          values: formValues.value,
        };
        return h('div', {class: containerClass}, slots['default'](renderProps));
      }

      // Loading / validating state
      if (isValidatingToken.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h('div', {style: 'display:flex;flex-direction:column;align-items:center;gap:1rem;padding:2rem'}, [
            h(Spinner),
            h(Typography, {variant: 'body1'}, () => 'Validating your invite link...'),
          ]),
        );
      }

      // Invalid token
      if (isTokenInvalid.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () => [
          h('div', {style: 'padding:1rem'}, [
            h(Typography, {variant: 'h5'}, () => 'Invalid Invite Link'),
            h(
              Alert,
              {style: 'margin-top:1rem', variant: 'error'},
              () =>
                apiError.value?.message ||
                'This invite link is invalid or has expired. Please contact your administrator for a new invite.',
            ),
            props.onGoToSignIn
              ? h('div', {style: 'display:flex;justify-content:center;margin-top:1.5rem'}, [
                  h(Button, {onClick: props.onGoToSignIn, variant: 'outline'}, () => 'Go to Sign In'),
                ])
              : null,
          ]),
        ]);
      }

      // Completion
      if (isComplete.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () => [
          h('div', {style: 'padding:1rem'}, [
            h(Typography, {variant: 'h5'}, () => 'Account Setup Complete!'),
            h(
              Alert,
              {style: 'margin-top:1rem', variant: 'success'},
              () => 'Your account has been successfully set up. You can now sign in with your credentials.',
            ),
            props.onGoToSignIn
              ? h('div', {style: 'display:flex;justify-content:center;margin-top:1.5rem'}, [
                  h(Button, {onClick: props.onGoToSignIn, variant: 'solid'}, () => 'Sign In'),
                ])
              : null,
          ]),
        ]);
      }

      // Render form components
      const renderedComponents: VNode[] =
        componentsWithoutHeadings.length > 0
          ? renderInviteUserComponents(
              componentsWithoutHeadings,
              formValues.value,
              touchedFields.value,
              formErrors.value,
              isLoading.value,
              isFormValid.value,
              handleInputChange,
              {
                meta,
                onInputBlur: handleInputBlur,
                onSubmit: handleSubmit,
                size: props.size,
                t,
                variant: props.variant,
              },
            )
          : [];

      return h(Card, {class: containerClass, variant: props.variant}, () => [
        (props.showTitle || props.showSubtitle) && (title || subtitle)
          ? h('div', {style: 'padding:1rem 1rem 0'}, [
              props.showTitle && title ? h(Typography, {variant: 'h5'}, () => title) : null,
              props.showSubtitle && subtitle
                ? h(Typography, {style: 'margin-top:0.25rem', variant: 'body1'}, () => subtitle)
                : null,
            ])
          : null,
        apiError.value
          ? h(
              'div',
              {style: 'padding:0 1rem;margin-bottom:1rem'},
              h(Alert, {variant: 'error'}, () => apiError.value.message),
            )
          : null,
        h(
          'div',
          {style: 'padding:1rem'},
          ((): (VNode | VNode[] | null)[] => {
            const formContent: (VNode | VNode[] | null)[] = [];

            if (renderedComponents.length > 0) {
              formContent.push(renderedComponents);
            } else if (!isLoading.value) {
              formContent.push(h(Alert, {variant: 'warning'}, () => 'No form components available'));
            }

            if (isLoading.value) {
              formContent.push(h('div', {style: 'display:flex;justify-content:center;padding:1rem'}, h(Spinner)));
            }

            return formContent;
          })(),
        ),
        props.onGoToSignIn
          ? h('div', {style: 'margin-top:1.5rem;text-align:center;padding:0 1rem 1rem'}, [
              h(Typography, {variant: 'body2'}, () => [
                'Already have an account? ',
                h(
                  Button,
                  {onClick: props.onGoToSignIn, style: 'min-width:auto;padding:0', variant: 'text'},
                  () => 'Sign In',
                ),
              ]),
            ])
          : null,
      ]);
    };
  },
});

export default BaseAcceptInvite;
