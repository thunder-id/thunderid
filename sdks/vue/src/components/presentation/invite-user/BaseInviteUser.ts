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

import {EmbeddedFlowType, FlowMetadataResponse, withVendorCSSClassPrefix} from '@thunderid/browser';
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
import {extractErrorMessage, normalizeFlowResponse} from '../../../utils/v2/flowTransformer';
import {renderInviteUserComponents} from '../../auth/sign-in/AuthOptionFactory';
import Alert from '../../primitives/Alert';
import Button from '../../primitives/Button';
import Card from '../../primitives/Card';
import Spinner from '../../primitives/Spinner';
import Typography from '../../primitives/Typography';

/**
 * Flow response from the invite-user backend.
 */
export interface InviteUserFlowResponse {
  data?: {
    additionalData?: Record<string, string>;
    components?: any[];
    meta?: {
      components?: any[];
    };
  };
  failureReason?: string;
  flowId: string;
  flowStatus: 'INCOMPLETE' | 'COMPLETE' | 'ERROR';
  type?: 'VIEW' | 'REDIRECTION';
}

/**
 * Render props passed to the default scoped slot.
 */
export interface BaseInviteUserRenderProps {
  components: any[];
  copyInviteLink: () => Promise<void>;
  error?: Error | null;
  fieldErrors: Record<string, string>;
  flowId?: string;
  handleInputBlur: (name: string) => void;
  handleInputChange: (name: string, value: string) => void;
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;
  inviteLink?: string;
  inviteLinkCopied: boolean;
  isEmailSent: boolean;
  isInviteGenerated: boolean;
  isLoading: boolean;
  isValid: boolean;
  meta: FlowMetadataResponse | null;
  resetFlow: () => void;
  subtitle?: string;
  title?: string;
  touched: Record<string, boolean>;
  values: Record<string, string>;
}

export interface BaseInviteUserProps {
  className?: string;
  isInitialized?: boolean;
  onError?: (error: Error) => void;
  onFlowChange?: (response: InviteUserFlowResponse) => void;
  onInitialize: (payload: Record<string, any>) => Promise<InviteUserFlowResponse>;
  onInviteLinkGenerated?: (inviteLink: string, flowId: string) => void;
  onSubmit: (payload: Record<string, any>) => Promise<InviteUserFlowResponse>;
  showSubtitle?: boolean;
  showTitle?: boolean;
  size?: 'small' | 'medium' | 'large';
  variant?: 'outlined' | 'elevated';
}

/**
 * BaseInviteUser — handles the admin invite-user flow lifecycle.
 *
 * Steps: user type selection → user details → invite link generation.
 */
const BaseInviteUser: Component = defineComponent({
  name: 'BaseInviteUser',
  props: {
    className: {default: '', type: String},
    isInitialized: {default: true, type: Boolean},
    onError: {default: undefined, type: Function as PropType<(error: Error) => void>},
    onFlowChange: {
      default: undefined,
      type: Function as PropType<(response: InviteUserFlowResponse) => void>,
    },
    onInitialize: {
      required: true,
      type: Function as PropType<(payload: Record<string, any>) => Promise<InviteUserFlowResponse>>,
    },
    onInviteLinkGenerated: {
      default: undefined,
      type: Function as PropType<(inviteLink: string, flowId: string) => void>,
    },
    onSubmit: {
      required: true,
      type: Function as PropType<(payload: Record<string, any>) => Promise<InviteUserFlowResponse>>,
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
    const isFlowInitialized: Ref<boolean> = ref(false);
    const currentFlow: Ref<InviteUserFlowResponse | null> = ref(null);
    const apiError: Ref<Error | null> = ref(null);

    // Form state
    const formValues: Ref<Record<string, string>> = ref({});
    const formErrors: Ref<Record<string, string>> = ref({});
    const touchedFields: Ref<Record<string, boolean>> = ref({});
    const isFormValid: Ref<boolean> = ref(true);

    // Invite state
    const inviteLink: Ref<string | undefined> = ref(undefined);
    const inviteLinkCopied: Ref<boolean> = ref(false);
    const emailSent: Ref<boolean> = ref(false);

    let initializationAttempted = false;

    // ── Helpers ──

    const handleError = (error: any): void => {
      const errorMessage: string =
        error?.failureReason || extractErrorMessage(error, t, 'components.inviteUser.errors.generic');
      apiError.value = error instanceof Error ? error : new Error(errorMessage);
      props.onError?.(apiError.value);
    };

    const normalizeFlowResponseLocal = (response: InviteUserFlowResponse): InviteUserFlowResponse => {
      if (!response?.data?.meta?.components) return response;
      try {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {defaultErrorKey: 'components.inviteUser.errors.generic', resolveTranslations: false},
          (metaRef as Ref<FlowMetadataResponse | null>).value,
        );
        return {...response, data: {...response.data, components: components as any}};
      } catch {
        return response;
      }
    };

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
            (comp.type === 'TEXT_INPUT' || comp.type === 'EMAIL_INPUT' || comp.type === 'SELECT') &&
            comp.required &&
            comp.ref
          ) {
            const value: string = formValues.value[comp.ref] || '';
            if (!value || value.trim() === '') {
              errors[comp.ref] = `${comp.label || comp.ref} is required`;
            }
            if (comp.type === 'EMAIL_INPUT' && value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
              errors[comp.ref] = 'Please enter a valid email address';
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

        const rawResponse: InviteUserFlowResponse = await props.onSubmit(payload);
        const response: InviteUserFlowResponse = normalizeFlowResponseLocal(rawResponse);
        props.onFlowChange?.(response);

        // Check for invite link
        if (response.data?.additionalData?.['inviteLink']) {
          const linkValue: string = response.data.additionalData['inviteLink'];
          inviteLink.value = linkValue;
          props.onInviteLinkGenerated?.(linkValue, response.flowId);
        }

        if (response.data?.additionalData?.['emailSent'] === 'true') {
          emailSent.value = true;
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

    // ── Copy invite link ──

    const copyInviteLink = async (): Promise<void> => {
      if (!inviteLink.value) return;
      try {
        await navigator.clipboard.writeText(inviteLink.value);
        inviteLinkCopied.value = true;
        setTimeout(() => {
          inviteLinkCopied.value = false;
        }, 3000);
      } catch {
        const textArea: HTMLTextAreaElement = document.createElement('textarea');
        textArea.value = inviteLink.value;
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
        inviteLinkCopied.value = true;
        setTimeout(() => {
          inviteLinkCopied.value = false;
        }, 3000);
      }
    };

    // ── Reset flow ──

    const resetFlow = (): void => {
      isFlowInitialized.value = false;
      currentFlow.value = null;
      apiError.value = null;
      formValues.value = {};
      formErrors.value = {};
      touchedFields.value = {};
      inviteLink.value = undefined;
      inviteLinkCopied.value = false;
      emailSent.value = false;
      initializationAttempted = false;
    };

    // ── Flow initialization ──

    watch(
      () => [props.isInitialized, isFlowInitialized.value] as [boolean, boolean],
      ([initialized, flowInit]: [boolean, boolean]) => {
        if (initialized && !flowInit && !initializationAttempted) {
          initializationAttempted = true;

          (async (): Promise<void> => {
            isLoading.value = true;
            apiError.value = null;

            try {
              const payload: any = {flowType: EmbeddedFlowType.UserOnboarding, verbose: true};
              const rawResponse: InviteUserFlowResponse = await props.onInitialize(payload);
              const response: InviteUserFlowResponse = normalizeFlowResponseLocal(rawResponse);
              currentFlow.value = response;
              isFlowInitialized.value = true;
              props.onFlowChange?.(response);

              if (response.flowStatus === 'ERROR') {
                handleError(response);
              }
            } catch (err) {
              handleError(err);
            } finally {
              isLoading.value = false;
            }
          })();
        }
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
      const containerClass: string = [withVendorCSSClassPrefix('invite-user'), props.className]
        .filter(Boolean)
        .join(' ');

      const components: any[] = currentFlow.value?.data?.components || currentFlow.value?.data?.meta?.components || [];
      const {title, subtitle} = extractHeadings(components);
      const componentsWithoutHeadings: any[] = filterHeadings(components);
      const isInviteGenerated = !!inviteLink.value;
      const isEmailSent: boolean = emailSent.value;

      const meta: FlowMetadataResponse | null = (metaRef as Ref<FlowMetadataResponse | null>).value;

      // Scoped slot
      if (slots['default']) {
        const renderProps: BaseInviteUserRenderProps = {
          components,
          copyInviteLink,
          error: apiError.value,
          fieldErrors: formErrors.value,
          flowId: currentFlow.value?.flowId,
          handleInputBlur,
          handleInputChange,
          handleSubmit,
          inviteLink: inviteLink.value,
          inviteLinkCopied: inviteLinkCopied.value,
          isEmailSent,
          isInviteGenerated,
          isLoading: isLoading.value,
          isValid: isFormValid.value,
          meta,
          resetFlow,
          subtitle,
          title,
          touched: touchedFields.value,
          values: formValues.value,
        };
        return h('div', {class: containerClass}, slots['default'](renderProps));
      }

      // Waiting for SDK initialization
      if (!props.isInitialized || (!isFlowInitialized.value && isLoading.value)) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h('div', {style: 'display:flex;justify-content:center;padding:2rem'}, h(Spinner)),
        );
      }

      // Error state during initialization
      if (!currentFlow.value && apiError.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h(Alert, {variant: 'error'}, () => apiError.value.message),
        );
      }

      // Email sent confirmation
      if (isInviteGenerated && isEmailSent) {
        return h(Card, {class: containerClass, variant: props.variant}, () => [
          h('div', {style: 'padding:1rem'}, [
            h(Typography, {variant: 'h5'}, () => 'Invite Email Sent!'),
            h(
              Alert,
              {style: 'margin-top:1rem', variant: 'success'},
              () =>
                'An invitation email has been sent successfully. The user can complete their registration using the link in the email.',
            ),
            h('div', {style: 'display:flex;gap:0.5rem;margin-top:1.5rem'}, [
              h(Button, {onClick: resetFlow, variant: 'outline'}, () => 'Invite Another User'),
            ]),
          ]),
        ]);
      }

      // Invite link generated — show copy link
      if (isInviteGenerated && inviteLink.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () => [
          h('div', {style: 'padding:1rem'}, [
            h(Typography, {variant: 'h5'}, () => 'Invite Link Generated!'),
            h(
              Alert,
              {style: 'margin-top:1rem', variant: 'success'},
              () => 'Share this link with the user to complete their registration.',
            ),
            h('div', {style: 'margin-top:1rem'}, [
              h(Typography, {style: 'margin-bottom:0.5rem', variant: 'body2'}, () => 'Invite Link'),
              h(
                'div',
                {
                  style:
                    'display:flex;align-items:center;gap:0.5rem;padding:0.75rem;background:var(--thunder-color-background-secondary,#f5f5f5);border-radius:4px;word-break:break-all',
                },
                [
                  h(Typography, {style: 'flex:1', variant: 'body2'}, () => inviteLink.value),
                  h(Button, {onClick: copyInviteLink, size: 'small', variant: 'outline'}, () =>
                    inviteLinkCopied.value ? 'Copied!' : 'Copy',
                  ),
                ],
              ),
            ]),
            h('div', {style: 'display:flex;gap:0.5rem;margin-top:1.5rem'}, [
              h(Button, {onClick: resetFlow, variant: 'outline'}, () => 'Invite Another User'),
            ]),
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
      ]);
    };
  },
});

export default BaseInviteUser;
