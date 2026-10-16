// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery} from '@tanstack/react-query';
import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {useFlowTextResolver} from '@thunderid/configure-users';
import {useConfig, useToast} from '@thunderid/contexts';
import {FlowComponentRenderer, getResourceSelectAdapter, type RenderFlowField} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {
  BaseInviteUser,
  toUserSelectOption,
  useThunderID,
  type BaseInviteUserRenderProps,
  type EmbeddedFlowComponent,
  type InviteUserFlowResponse,
  type ManagedUser,
} from '@thunderid/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, AlertTitle, Box, Button, CircularProgress, FormControl, FormLabel, Stack} from '@wso2/oxygen-ui';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import AgentQueryKeys from '@/constants/agent-query-keys';
import useAgentRoutes from '@/hooks/useAgentRoutes';
import resolveAgentOnboardingFlow, {AgentOnboardingFlowProblem} from '@/utils/resolveAgentOnboardingFlow';

/** The flow's identifier for the owner input, matching ownerKey backend side. */
const ownerInputRef = 'owner';

/** A flow component with the optional fields the page itself reads. The adapters read the rest. */
type FlowSubComponent = EmbeddedFlowComponent & {
  variant?: string;
  required?: boolean;
  components?: FlowSubComponent[];
};

/**
 * Refs of the inputs a step insists on. A boolean is left out: unchecked is an answer, so a
 * required checkbox is satisfied the moment the step renders.
 */
const requiredRefs = (components: FlowSubComponent[]): string[] =>
  components.flatMap((component) =>
    component.components
      ? requiredRefs(component.components)
      : component.ref && component.required && component.type !== 'BOOLEAN_INPUT'
        ? [component.ref]
        : [],
  );

/** Whether a step carries anything to fill in or submit, as opposed to only displaying a result. */
const hasActionsOrInputs = (components: FlowSubComponent[]): boolean =>
  components.some(
    (component) =>
      component.ref != null ||
      component.eventType != null ||
      (Array.isArray(component.components) && hasActionsOrInputs(component.components)),
  );

/**
 * A step's title, taken from its first HEADING_1. The flow names each screen that way, so the
 * trail reads the same as the headings the user just moved through.
 *
 * Duplicated from UserAddPage for now; it belongs beside the other shared flow helpers once the
 * agents feature moves into its own package.
 */
const deriveStepLabel = (
  components: FlowSubComponent[],
  resolve: (text?: string) => string | undefined,
  t: ReturnType<typeof useTranslation>['t'],
): string => {
  const heading = components.find(
    (component) =>
      component.type === 'TEXT' && component.variant === 'HEADING_1' && typeof component.label === 'string',
  );

  return heading && typeof heading.label === 'string' ? t(resolve(heading.label) ?? heading.label) : '';
};

/**
 * Renders one step of the onboarding flow with the shared flow renderer, so every component type the renderer
 * supports is available to the flow. Submitting is held back until the step's required inputs are filled.
 */
function AgentOnboardStep({
  renderProps,
  onClose,
  onStepLabelChange,
  onResetFlowAvailable,
}: {
  renderProps: BaseInviteUserRenderProps;
  onClose: () => void;
  onStepLabelChange: (label: string) => void;
  onResetFlowAvailable: (reset: () => void) => void;
}): JSX.Element {
  const {t} = useTranslation();
  const resolve = useFlowTextResolver();
  const currentUser = useThunderID().user as {id?: string} | null | undefined;
  const currentUserId = currentUser?.id ?? null;
  const currentUserLabel = currentUserId ? toUserSelectOption(currentUser as unknown as ManagedUser)?.label : undefined;
  const {showToast} = useToast();
  const {
    additionalData,
    components,
    fieldErrors,
    handleInputBlur,
    handleInputChange,
    handleSubmit,
    isLoading,
    resetFlow,
    touched,
    values,
  } = renderProps;

  const stepLabel = deriveStepLabel(components as FlowSubComponent[], resolve, t);

  // Reported from an effect rather than during render, which would set parent state mid-render.
  useEffect(() => {
    if (stepLabel) {
      onStepLabelChange(stepLabel);
    }
  }, [stepLabel, onStepLabelChange]);

  // The breadcrumb's first crumb restarts the flow, and only this scope holds resetFlow.
  useEffect(() => {
    if (resetFlow) {
      onResetFlowAvailable(resetFlow);
    }
  }, [resetFlow, onResetFlowAvailable]);

  // A picker has nowhere on the page to show a failed load, so it is reported as a toast.
  const handleOptionsError = useCallback(
    (error: Error): void => {
      showToast(
        getErrorMessage(error, t, 'agents:onboarding.errors.optionsFailed', 'Could not load the list. Try again.'),
        'error',
      );
    },
    [showToast, t],
  );

  // A step cannot be submitted while it is missing something it asked for. Without this the
  // action posts, the engine refuses it, and the screen simply does not move.
  const isIncomplete = requiredRefs(components as FlowSubComponent[]).some((ref) => !values?.[ref]?.trim());

  // Fields the shared adapters do not draw: the organization unit picker and the resource pickers, which load
  // their options with the signed-in user's token.
  const renderField: RenderFlowField = (component, fieldProps) => {
    const ResourceSelect = getResourceSelectAdapter(component.type);

    if (ResourceSelect) {
      const isOwner = component.ref === ownerInputRef;

      // The server would resolve an unset owner to the signed-in user, so preselecting them only makes
      // that visible before submitting.
      return (
        <ResourceSelect
          {...fieldProps}
          defaultValue={isOwner ? (currentUserId ?? undefined) : undefined}
          defaultLabel={isOwner ? currentUserLabel : undefined}
          onError={handleOptionsError}
        />
      );
    }

    if (component.type === 'OU_SELECT') {
      const {ref} = component;

      if (!ref || typeof ref !== 'string') return null;

      return (
        <FormControl fullWidth required={component.required ?? false}>
          <FormLabel htmlFor={ref}>{t(resolve(component.label) ?? '')}</FormLabel>
          <OrganizationUnitTreePicker
            value={values?.[ref] ?? ''}
            onChange={(ouId: string) => handleInputChange(ref, ouId)}
            rootOuId={additionalData?.['rootOuId'] as string | undefined}
          />
        </FormControl>
      );
    }

    return undefined;
  };

  const steps = components as FlowSubComponent[];

  return (
    <Stack spacing={2}>
      {steps.map((component, index) => (
        <FlowComponentRenderer
          key={component.id ?? index}
          component={component}
          index={index}
          values={values}
          touched={touched}
          fieldErrors={fieldErrors}
          isLoading={isLoading}
          isSubmitDisabled={isIncomplete}
          resolve={resolve}
          additionalData={additionalData}
          onInputChange={handleInputChange}
          onBlur={handleInputBlur}
          onSubmit={(action, inputs) => {
            void handleSubmit(action, inputs);
          }}
          renderField={renderField}
        />
      ))}
      {/* A step with nothing to submit is the end of the run, so the way out is offered here
          rather than leaving the close control in the header as the only exit. */}
      {steps.length > 0 && !hasActionsOrInputs(steps) && (
        <Stack direction="row" spacing={2} justifyContent="flex-start" sx={{mt: 4}}>
          <Button variant="outlined" onClick={onClose}>
            {t('common:actions.close', 'Close')}
          </Button>
          <Button variant="contained" onClick={() => resetFlow()}>
            {t('agents:onboarding.addAnother', 'Add Another Agent')}
          </Button>
        </Stack>
      )}
    </Stack>
  );
}

/**
 * Creates an agent by running the configured agent onboarding flow.
 *
 * The screens, their order and the fields they collect all come from the flow, so this page holds
 * no knowledge of what an agent needs. There is no non-flow fallback: the flow's provisioning node
 * is what creates the agent, so an unresolved flow is reported rather than worked around.
 */
export default function AgentOnboardPage(): JSX.Element {
  const routes = useAgentRoutes();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const logger = useLogger('AgentOnboardPage');

  const isWelcomeFlow = pathname.startsWith('/welcome');

  // The flow decides how many screens there are, so there is no total to divide by. The bar
  // advances asymptotically with each step and reaches the end only once the flow reports
  // completion.
  const [stepsSeen, setStepsSeen] = useState(0);
  // The trail of step headings visited so far, seeded with the page itself.
  const [breadcrumbs, setBreadcrumbs] = useState<string[]>([t('agents:addAgent', 'Add Agent')]);
  const prevStepLabelRef = useRef<string>('');
  const resetFlowRef = useRef<(() => void) | null>(null);
  const [isFlowComplete, setIsFlowComplete] = useState(false);
  const [flowError, setFlowError] = useState<string | null>(null);
  const progress = isFlowComplete ? 100 : (1 - 1 / (stepsSeen + 1)) * 100;

  const {
    data: resolution,
    isLoading,
    error: resolutionError,
  } = useQuery({
    queryKey: [AgentQueryKeys.AGENTS, 'onboardingFlow'],
    queryFn: () => resolveAgentOnboardingFlow(http as never, getServerUrl()),
  });

  const handleClose = useCallback((): void => {
    void navigate(isWelcomeFlow ? routes.welcome.getStarted() : routes.agents.list());
  }, [navigate, isWelcomeFlow, routes]);

  const handleStepLabelChange = useCallback((label: string): void => {
    if (label === prevStepLabelRef.current) return;
    prevStepLabelRef.current = label;
    setBreadcrumbs((previous) => {
      // Revisiting a step trims the trail back to it rather than repeating it.
      const seenAt = previous.indexOf(label);

      return seenAt >= 0 ? previous.slice(0, seenAt + 1) : [...previous, label];
    });
  }, []);

  const handleResetFlowAvailable = useCallback((reset: () => void): void => {
    resetFlowRef.current = reset;
  }, []);

  const handleRestart = useCallback((): void => {
    resetFlowRef.current?.();
    prevStepLabelRef.current = '';
    setStepsSeen(0);
    setIsFlowComplete(false);
    setBreadcrumbs([t('agents:addAgent', 'Add Agent')]);
  }, [t]);

  const executeFlow = useCallback(
    async (payload: Record<string, unknown>): Promise<InviteUserFlowResponse> => {
      const response: {data: InviteUserFlowResponse} = await http.request({
        url: `${getServerUrl()}/flow/execute`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: payload,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    [http, getServerUrl],
  );

  // The flow is addressed by id because the console resolved it from a configured handle. The SDK's
  // InviteUser wrapper instead hardcodes a flow type, which is why this page drives BaseInviteUser.
  const handleInitialize = useCallback(
    (payload: Record<string, unknown>): Promise<InviteUserFlowResponse> =>
      executeFlow({...payload, flowId: resolution?.flowId, verbose: true}),
    [executeFlow, resolution?.flowId],
  );

  const renderProblem = (title: string, description: string): JSX.Element => (
    <Box sx={{p: 3}}>
      <Alert severity="error">
        <AlertTitle>{title}</AlertTitle>
        {description}
      </Alert>
    </Box>
  );

  const renderBody = (): JSX.Element => {
    if (isLoading) {
      return (
        <Box sx={{display: 'flex', justifyContent: 'center', p: 6}}>
          <CircularProgress />
        </Box>
      );
    }

    if (resolutionError) {
      logger.error('Failed to resolve the agent onboarding flow', {error: resolutionError});

      return renderProblem(
        t('agents:onboarding.errors.unavailable.title'),
        t('agents:onboarding.errors.unavailable.description'),
      );
    }

    if (resolution?.problem === AgentOnboardingFlowProblem.NotConfigured) {
      return renderProblem(
        t('agents:onboarding.errors.notConfigured.title'),
        t('agents:onboarding.errors.notConfigured.description'),
      );
    }

    if (resolution?.problem === AgentOnboardingFlowProblem.FlowMissing) {
      return renderProblem(
        t('agents:onboarding.errors.flowMissing.title'),
        t('agents:onboarding.errors.flowMissing.description', {handle: resolution.handle}),
      );
    }

    return (
      <Stack spacing={2}>
        {/* A flow-level failure leaves the step with no components to render, so it is surfaced
            here rather than only logged. */}
        {flowError !== null && (
          <Alert severity="error" onClose={() => setFlowError(null)}>
            {flowError}
          </Alert>
        )}
        <BaseInviteUser
          onInitialize={handleInitialize}
          onSubmit={executeFlow}
          onError={(err: Error) => {
            logger.error('Agent onboarding flow error', {error: err});
            setFlowError(err.message || t('agents:onboarding.errors.stepFailed'));
          }}
          onFlowChange={(response: InviteUserFlowResponse) => {
            const failure = (response as {error?: {message?: {defaultValue?: string}}}).error;
            if (failure) {
              setFlowError(failure.message?.defaultValue ?? t('agents:onboarding.errors.stepFailed'));
              return;
            }
            setFlowError(null);
            if (response?.flowStatus === 'COMPLETE') {
              setIsFlowComplete(true);
              return;
            }
            if (response?.flowStatus === 'INCOMPLETE') {
              setStepsSeen((seen) => seen + 1);
            }
          }}
        >
          {(renderProps: BaseInviteUserRenderProps) => (
            <AgentOnboardStep
              renderProps={renderProps}
              onClose={handleClose}
              onStepLabelChange={handleStepLabelChange}
              onResetFlowAvailable={handleResetFlowAvailable}
            />
          )}
        </BaseInviteUser>
      </Stack>
    );
  };

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={progress}
      breadcrumbItems={breadcrumbs.map((label, index) => ({
        key: `breadcrumb-${index}`,
        label,
        onClick: index === 0 ? handleRestart : undefined,
        disabled: index !== 0,
      }))}
      footer={null}
    >
      {renderBody()}
    </FullScreenCreationWizardLayout>
  );
}
