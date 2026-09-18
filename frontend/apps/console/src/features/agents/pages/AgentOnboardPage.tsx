// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery} from '@tanstack/react-query';
import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {useFlowTextResolver, useGetUsers} from '@thunderid/configure-users';
import {useConfig} from '@thunderid/contexts';
import {CopyableTextAdapter, type FlowComponent} from '@thunderid/design';
import {useCopyToClipboard} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {
  BaseInviteUser,
  useThunderID,
  type BaseInviteUserRenderProps,
  type EmbeddedFlowComponent,
  type InviteUserFlowResponse,
} from '@thunderid/react';
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  FormControl,
  FormControlLabel,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Copy, Eye, EyeOff} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import AgentQueryKeys from '../constants/agent-query-keys';
import resolveAgentOnboardingFlow, {AgentOnboardingFlowProblem} from '../utils/resolveAgentOnboardingFlow';

/** A flow component with the optional fields the renderer below reads. */
type FlowSubComponent = EmbeddedFlowComponent & {
  variant?: string;
  required?: boolean;
  placeholder?: string;
  options?: unknown[];
  masked?: boolean;
  source?: string;
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

/** Names a user the way the deployment identifies them, falling back to something readable. */
const userLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attributes = user.attributes ?? {};
  const username = typeof attributes.username === 'string' ? attributes.username : undefined;
  const email = typeof attributes.email === 'string' ? attributes.email : undefined;

  return username ?? email ?? user.id;
};

interface SecretFieldProps {
  id: string;
  label: string;
  value: string;
}

/**
 * Read-only field for a value that must not sit on screen in the clear: masked by default,
 * revealed on request, and copyable without being read. Mirrors ShowClientSecret.
 */
function SecretField({id, label, value}: SecretFieldProps): JSX.Element {
  const {t} = useTranslation();
  const [isRevealed, setIsRevealed] = useState(false);
  const {copy} = useCopyToClipboard({resetDelay: 2000}) as {copy: (text: string) => Promise<void>};

  return (
    <Box>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
        {label}
      </Typography>
      <TextField
        fullWidth
        id={id}
        data-testid={id}
        type={isRevealed ? 'text' : 'password'}
        value={value}
        size="small"
        InputProps={{
          endAdornment: (
            <InputAdornment position="end">
              <IconButton
                aria-label={isRevealed ? t('common:actions.hide', 'Hide') : t('common:actions.show', 'Show')}
                onClick={() => setIsRevealed(!isRevealed)}
                edge="end"
                size="small"
              >
                {isRevealed ? <EyeOff size={16} /> : <Eye size={16} />}
              </IconButton>
              <IconButton
                aria-label={t('common:actions.copy', 'Copy')}
                onClick={() => {
                  copy(value).catch(() => null);
                }}
                edge="end"
                size="small"
                sx={{ml: 0.5}}
              >
                <Copy size={16} />
              </IconButton>
            </InputAdornment>
          ),
          readOnly: true,
        }}
      />
    </Box>
  );
}

interface UserSelectProps {
  id: string;
  label: string;
  placeholder: string;
  required: boolean;
  value: string;
  onChange: (value: string) => void;
}

/**
 * Picker for a USER_SELECT input. As with OU_SELECT the flow sends no candidates, so the list is
 * fetched here; the row shown is the user's display value while the value submitted is their id.
 */
function UserSelect({id, label, placeholder, required, value, onChange}: UserSelectProps): JSX.Element {
  const {data} = useGetUsers({limit: 100, offset: 0});
  const users = data?.users ?? [];

  return (
    <FormControl fullWidth required={required}>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <Select
        id={id}
        value={value}
        size="small"
        displayEmpty
        required={required}
        onChange={(e) => onChange(String(e.target.value))}
      >
        <MenuItem value="">{placeholder}</MenuItem>
        {users.map((user) => (
          <MenuItem key={user.id} value={user.id}>
            {userLabel(user)}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

/**
 * Renders one step of the onboarding flow.
 *
 * Components are matched here rather than delegated to FlowComponentRenderer, as UserCreatePage
 * and UserAddPage also do. That renderer covers only the context-free adapters: TEXT and RICH_TEXT
 * read the DesignProvider the console does not mount, and it has no case for OU_SELECT or
 * USER_SELECT.
 */
function AgentOnboardStep({
  renderProps,
  onClose,
}: {
  renderProps: BaseInviteUserRenderProps;
  onClose: () => void;
}): JSX.Element {
  const {t} = useTranslation();
  const resolve = useFlowTextResolver();
  const {additionalData, components, values, isLoading, handleInputChange, handleSubmit, resetFlow} = renderProps;

  const text = (component: FlowSubComponent): string => {
    const label = typeof component.label === 'string' ? component.label : '';

    return t(resolve(label) ?? label);
  };

  const placeholderOf = (component: FlowSubComponent, fallback: string): string =>
    component.placeholder ? (resolve(component.placeholder) ?? fallback) : fallback;

  // A step cannot be submitted while it is missing something it asked for. Without this the
  // action posts, the engine refuses it, and the screen simply does not move.
  const isIncomplete = requiredRefs(components as FlowSubComponent[]).some(
    (ref) => !((values as Record<string, unknown>)?.[ref] as string),
  );

  const renderComponent = (component: FlowSubComponent, index: number): JSX.Element | null => {
    const key = component.id ?? index;
    const ref = component.ref;
    const required = component.required ?? false;
    const value = ((values as Record<string, unknown>)?.[ref ?? ''] as string) ?? '';

    if (component.type === 'TEXT') {
      return component.variant === 'HEADING_1' ? (
        <Typography key={key} variant="h1" gutterBottom>
          {text(component)}
        </Typography>
      ) : (
        <Typography key={key} variant="body1" color="text.secondary">
          {text(component)}
        </Typography>
      );
    }

    if (
      component.type === 'TEXT_INPUT' ||
      component.type === 'EMAIL_INPUT' ||
      component.type === 'NUMBER_INPUT' ||
      component.type === 'PHONE_INPUT' ||
      component.type === 'PASSWORD_INPUT'
    ) {
      if (!ref) return null;

      const inputTypes: Record<string, string> = {
        EMAIL_INPUT: 'email',
        NUMBER_INPUT: 'number',
        PASSWORD_INPUT: 'password',
        PHONE_INPUT: 'tel',
      };

      return (
        <FormControl key={key} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{text(component)}</FormLabel>
          <TextField
            id={ref}
            type={inputTypes[component.type] ?? 'text'}
            value={value}
            size="small"
            required={required}
            placeholder={placeholderOf(component, '')}
            onChange={(e) => handleInputChange(ref, e.target.value)}
          />
        </FormControl>
      );
    }

    if (component.type === 'SELECT') {
      const options = component.options ?? [];
      if (!ref || !options.length) return null;

      return (
        <FormControl key={key} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{text(component)}</FormLabel>
          <Select
            id={ref}
            value={value}
            size="small"
            displayEmpty
            required={required}
            onChange={(e) => handleInputChange(ref, String(e.target.value))}
          >
            <MenuItem value="">{placeholderOf(component, t('agents:onboarding.selectPlaceholder'))}</MenuItem>
            {options.map((option: unknown) => {
              const isObject = typeof option === 'object' && option !== null;
              const optionValue = String(isObject ? (option as Record<string, unknown>).value : option);
              const optionLabel = isObject ? String((option as Record<string, unknown>).label) : optionValue;

              return (
                <MenuItem key={optionValue} value={optionValue}>
                  {optionLabel}
                </MenuItem>
              );
            })}
          </Select>
        </FormControl>
      );
    }

    if (component.type === 'USER_SELECT') {
      if (!ref) return null;

      return (
        <UserSelect
          key={key}
          id={ref}
          label={text(component)}
          placeholder={placeholderOf(component, t('agents:onboarding.selectPlaceholder'))}
          required={required}
          value={value}
          onChange={(selected) => handleInputChange(ref, selected)}
        />
      );
    }

    if (component.type === 'OU_SELECT') {
      if (!ref) return null;

      return (
        <FormControl key={key} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{text(component)}</FormLabel>
          <OrganizationUnitTreePicker
            value={value}
            onChange={(ouId: string) => handleInputChange(ref, ouId)}
            rootOuId={additionalData?.rootOuId as string | undefined}
          />
        </FormControl>
      );
    }

    if (component.type === 'BOOLEAN_INPUT') {
      if (!ref) return null;

      return (
        <FormControl key={key} required={required}>
          <FormControlLabel
            control={
              <Checkbox
                id={ref}
                size="small"
                checked={value === 'true'}
                onChange={(e) => handleInputChange(ref, String(e.target.checked))}
              />
            }
            label={text(component)}
          />
        </FormControl>
      );
    }

    if (component.type === 'COPYABLE_TEXT') {
      if (component.masked) {
        const source = component.source ?? '';
        const secret = (additionalData as Record<string, string> | undefined)?.[source] ?? '';

        return <SecretField key={key} id={component.id ?? source} label={text(component)} value={secret} />;
      }

      return (
        <CopyableTextAdapter
          key={key}
          component={component as FlowComponent}
          resolve={resolve}
          additionalData={additionalData as Record<string, unknown> | undefined}
        />
      );
    }

    if (component.type === 'ACTION') {
      return (
        <Button
          key={key}
          variant={component.variant === 'OUTLINED' ? 'outlined' : 'contained'}
          disabled={isLoading || isIncomplete}
          fullWidth
          size="large"
          onClick={() => {
            void handleSubmit(component, values);
          }}
        >
          {text(component)}
        </Button>
      );
    }

    if (component.components) {
      return (
        <Stack key={key} direction="column" spacing={2}>
          {component.components.map((nested, nestedIndex) => renderComponent(nested, nestedIndex))}
        </Stack>
      );
    }

    return null;
  };

  const steps = components as FlowSubComponent[];

  return (
    <Stack spacing={2}>
      {steps.map((component, index) => renderComponent(component, index))}
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
    void navigate(isWelcomeFlow ? RouteConfig.welcome.getStarted() : RouteConfig.agents.list());
  }, [navigate, isWelcomeFlow]);

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
            <AgentOnboardStep renderProps={renderProps} onClose={handleClose} />
          )}
        </BaseInviteUser>
      </Stack>
    );
  };

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={progress}
      breadcrumbItems={[{key: 'agents', label: t('agents:listing.title')}]}
      footer={null}
    >
      {renderBody()}
    </FullScreenCreationWizardLayout>
  );
}
