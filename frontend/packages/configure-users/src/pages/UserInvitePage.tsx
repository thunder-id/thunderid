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

/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-explicit-any */

import {zodResolver} from '@hookform/resolvers/zod';
import {PageLoadingAnimation} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {CopyableTextAdapter, type FlowComponent} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {
  EmbeddedFlowComponentType,
  EmbeddedFlowEventType,
  InviteUser,
  useThunderID,
  type EmbeddedFlowComponent,
  type InviteUserRenderProps,
} from '@thunderid/react';
import type {ApiError} from '@thunderid/types';
import {
  Box,
  Stack,
  Typography,
  Button,
  Alert,
  AlertTitle,
  TextField,
  IconButton,
  FormControl,
  FormLabel,
  Select,
  MenuItem,
  LinearProgress,
  AppBreadcrumbs,
  CircularProgress,
} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useState, useEffect, useMemo, useCallback, useRef, type JSX} from 'react';
import {useForm, Controller} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import {z} from 'zod';
import CredentialFieldInput from '../components/CredentialFieldInput';

/** Typed shape for flow sub-components */
type FlowSubComponent = EmbeddedFlowComponent & {
  align?: string;
  direction?: string;
  eventType?: string;
  hint?: string;
  justify?: string;
  options?: unknown[];
  placeholder?: string;
  required?: boolean;
  variant?: string;
};

/**
 * Derive the current step label from flow components.
 * The backend sends HEADING_1 text component as step title.
 */
function deriveStepLabel(
  components: EmbeddedFlowComponent[],
  resolve: (key?: string) => string | undefined,
  t: ReturnType<typeof useTranslation>['t'],
): string {
  const heading = components.find(
    (comp) =>
      (String(comp.type) === String(EmbeddedFlowComponentType.Text) || comp.type === 'TEXT') &&
      (comp as FlowSubComponent).variant === 'HEADING_1' &&
      typeof comp.label === 'string',
  );

  if (heading && typeof heading.label === 'string') {
    return t(resolve(heading.label) ?? heading.label);
  }

  return '';
}

const FLOW_NOT_FOUND_ERROR_CODE = 'FLM-1003';

function containsFlowNotFoundText(value: string | undefined): boolean {
  return value?.toLowerCase().includes('flow not found') ?? false;
}

function isMissingOnboardingFlow(error: unknown): boolean {
  if (!error || typeof error !== 'object') {
    return false;
  }

  const flowError = error as Error & {
    code?: string;
    error?: {
      code?: string;
      description?: {defaultValue?: string; key?: string};
      message?: {defaultValue?: string; key?: string};
    };
    response?: {
      data?: ApiError;
      status?: number;
    };
    status?: number;
  };
  const {response} = flowError;
  const apiError = response?.data;

  return (
    apiError?.code === FLOW_NOT_FOUND_ERROR_CODE ||
    flowError.code === FLOW_NOT_FOUND_ERROR_CODE ||
    flowError.error?.code === FLOW_NOT_FOUND_ERROR_CODE ||
    containsFlowNotFoundText(apiError?.message) ||
    containsFlowNotFoundText(apiError?.description) ||
    containsFlowNotFoundText(flowError.message) ||
    containsFlowNotFoundText(flowError.error?.message?.defaultValue) ||
    containsFlowNotFoundText(flowError.error?.description?.defaultValue)
  );
}

const getOptionValue = (option: unknown): string => {
  if (typeof option === 'string') return option;
  if (typeof option === 'object' && option !== null && 'value' in option) {
    const {value} = option as {value: unknown};
    if (typeof value === 'string') return value;
    return JSON.stringify(value ?? option);
  }
  return JSON.stringify(option);
};

/**
 * Returns true if the component tree contains any action or user-input components.
 * Inputs are identified by having a `ref` property, actions by having an `eventType` property.
 */
function hasActionsOrInputs(comps: EmbeddedFlowComponent[]): boolean {
  return comps.some(
    (c) => c.ref != null || c.eventType != null || (Array.isArray(c.components) && hasActionsOrInputs(c.components)),
  );
}

const ONBOARDING_MODE_INVITE_ACTION_ID = 'action_invite_user';

/** Recursively finds an ACTION component by id within a flow component tree. */
function findActionComponentById(
  comps: EmbeddedFlowComponent[] | undefined,
  actionId: string,
): EmbeddedFlowComponent | undefined {
  if (!comps) return undefined;
  for (const comp of comps) {
    if (comp.id === actionId) return comp;
    if (Array.isArray(comp.components)) {
      const found = findActionComponentById(comp.components, actionId);
      if (found) return found;
    }
  }
  return undefined;
}

const getOptionLabel = (option: unknown): string => {
  if (typeof option === 'string') return option;
  if (typeof option === 'object' && option !== null && 'label' in option) {
    const {label} = option as {label: unknown};
    if (typeof label === 'string') return label;
    return JSON.stringify(label ?? option);
  }
  return JSON.stringify(option);
};

/**
 * Inner content component that renders the current flow step's form fields.
 */
function InviteUserStepContent({
  renderProps,
  flowError,
  handleClose,
  onResetLocalState,
}: {
  renderProps: InviteUserRenderProps;
  flowError: string | null;
  handleClose: () => void;
  onResetLocalState: () => void;
}): JSX.Element {
  const {
    additionalData,
    values,
    error,
    isLoading,
    components,
    handleInputChange,
    handleSubmit,
    resetFlow,
    isValid: propsIsValid,
  } = renderProps;
  const {resolveFlowTemplateLiterals: rawResolve} = useThunderID();
  const resolve = useCallback((text?: string) => (text ? rawResolve(text) : undefined), [rawResolve]);
  const {t} = useTranslation();
  const [activeActionId, setActiveActionId] = useState<string | null>(null);

  const buildFormSchema = useMemo(
    () =>
      (comps: EmbeddedFlowComponent[]): z.ZodObject<Record<string, z.ZodTypeAny>> => {
        const shape: Record<string, z.ZodTypeAny> = {};

        const processComponents = (compList: EmbeddedFlowComponent[]) => {
          compList.forEach((comp) => {
            if (
              (String(comp.type) === String(EmbeddedFlowComponentType.Block) || comp.type === 'BLOCK') &&
              comp.components
            ) {
              processComponents(comp.components);
            } else if (
              (String(comp.type) === String(EmbeddedFlowComponentType.TextInput) ||
                comp.type === 'TEXT_INPUT' ||
                comp.type === 'EMAIL_INPUT' ||
                comp.type === 'PHONE_INPUT' ||
                comp.type === 'PASSWORD_INPUT' ||
                comp.type === 'SELECT' ||
                comp.type === 'OU_SELECT') &&
              comp.ref
            ) {
              let fieldSchema: z.ZodTypeAny = z.string();

              if (comp.type === 'EMAIL_INPUT') {
                fieldSchema = z.string().email('Please enter a valid email address');
              } else if (comp.type === 'PHONE_INPUT') {
                fieldSchema = z.string().regex(/^\+?[0-9\s\-().]{7,20}$/, 'Please enter a valid phone number');
              } else if (comp.type === 'PASSWORD_INPUT') {
                fieldSchema = z.string();
              }

              const labelText = typeof comp.label === 'string' ? comp.label : comp.ref;
              if (comp.required) {
                fieldSchema = (fieldSchema as z.ZodString).min(
                  1,
                  `${t(resolve(labelText) ?? labelText) ?? comp.ref} is required`,
                );
              } else {
                fieldSchema = (fieldSchema as z.ZodString).optional();
              }

              shape[comp.ref] = fieldSchema;
            }
          });
        };

        processComponents(comps);
        return z.object(shape);
      },
    [t, resolve],
  );

  const formSchema = useMemo(() => {
    if (!components?.length) return z.object({}) as z.ZodObject<Record<string, z.ZodString>>;
    return buildFormSchema(components as EmbeddedFlowComponent[]);
  }, [components, buildFormSchema]);

  const renderFormField = (
    component: FlowSubComponent,
    index: number,
    formControl: ReturnType<typeof useForm>['control'],
    formErrors: ReturnType<typeof useForm>['formState']['errors'],
    isFormLoading: boolean,
    handleInputChangeFn: (field: string, value: string) => void,
  ) => {
    const {type, ref, label, placeholder, required, options, hint} = component;
    if (!ref) return null;

    const labelText = typeof label === 'string' ? label : '';
    const placeholderText = typeof placeholder === 'string' ? placeholder : '';

    if (String(type) === String(EmbeddedFlowComponentType.TextInput) || type === 'TEXT_INPUT') {
      return (
        <FormControl key={component.id ?? index} required={required}>
          <FormLabel htmlFor={ref}>{resolve(labelText) ?? labelText}</FormLabel>
          <Controller
            name={ref}
            control={formControl}
            rules={{required: required ? `${resolve(labelText) ?? labelText} is required` : false}}
            render={({field}) => (
              <TextField
                {...field}
                fullWidth
                size="small"
                id={ref}
                type="text"
                placeholder={resolve(placeholderText) ?? placeholderText}
                autoComplete="off"
                required={required}
                variant="outlined"
                disabled={isFormLoading}
                error={!!formErrors[ref]}
                helperText={formErrors[ref]?.message as string}
                color={formErrors[ref] ? 'error' : 'primary'}
                onChange={(e) => {
                  field.onChange(e);
                  handleInputChangeFn(ref, e.target.value);
                }}
              />
            )}
          />
        </FormControl>
      );
    }

    if (type === 'EMAIL_INPUT') {
      return (
        <FormControl key={component.id ?? index} required={required}>
          <FormLabel htmlFor={ref}>{resolve(labelText) ?? labelText}</FormLabel>
          <Controller
            name={ref}
            control={formControl}
            rules={{
              required: required ? `${resolve(labelText) ?? labelText} is required` : false,
              pattern: {value: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: 'Please enter a valid email address'},
            }}
            render={({field}) => (
              <TextField
                {...field}
                fullWidth
                size="small"
                id={ref}
                type="email"
                placeholder={resolve(placeholderText) ?? placeholderText}
                autoComplete="email"
                required={required}
                variant="outlined"
                disabled={isFormLoading}
                error={!!formErrors[ref]}
                helperText={formErrors[ref]?.message as string}
                color={formErrors[ref] ? 'error' : 'primary'}
                onChange={(e) => {
                  field.onChange(e);
                  handleInputChangeFn(ref, e.target.value);
                }}
              />
            )}
          />
        </FormControl>
      );
    }

    if (type === 'PHONE_INPUT') {
      return (
        <FormControl key={component.id ?? index} required={required}>
          <FormLabel htmlFor={ref}>{resolve(labelText) ?? labelText}</FormLabel>
          <Controller
            name={ref}
            control={formControl}
            rules={{required: required ? `${resolve(labelText) ?? labelText} is required` : false}}
            render={({field}) => (
              <TextField
                {...field}
                fullWidth
                size="small"
                id={ref}
                type="tel"
                placeholder={resolve(placeholderText) ?? placeholderText}
                autoComplete="tel"
                required={required}
                variant="outlined"
                disabled={isFormLoading}
                error={!!formErrors[ref]}
                helperText={formErrors[ref]?.message as string}
                color={formErrors[ref] ? 'error' : 'primary'}
                onChange={(e) => {
                  field.onChange(e);
                  handleInputChangeFn(ref, e.target.value);
                }}
              />
            )}
          />
        </FormControl>
      );
    }

    if (type === 'PASSWORD_INPUT') {
      return (
        <FormControl key={component.id ?? index} required={required}>
          <FormLabel htmlFor={ref}>{resolve(labelText) ?? labelText}</FormLabel>
          <Controller
            name={ref}
            control={formControl}
            rules={{required: required ? `${resolve(labelText) ?? labelText} is required` : false}}
            render={({field}) => (
              <CredentialFieldInput
                id={ref}
                name={field.name}
                value={(field.value as string) ?? ''}
                placeholder={resolve(placeholderText) ?? placeholderText}
                required={required ?? false}
                error={!!formErrors[ref]}
                helperText={formErrors[ref]?.message as string}
                color={formErrors[ref] ? 'error' : 'primary'}
                ariaLabel={resolve(labelText) ?? labelText}
                onChange={(e) => {
                  field.onChange(e);
                  handleInputChangeFn(ref, e.target.value);
                }}
                onBlur={field.onBlur}
                inputRef={field.ref}
              />
            )}
          />
        </FormControl>
      );
    }

    if (type === 'OU_SELECT') {
      return (
        <FormControl key={component.id ?? index} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{resolve(labelText) ?? labelText}</FormLabel>
          <Controller
            name={ref}
            control={formControl}
            rules={{required: required ? `${resolve(labelText) ?? labelText} is required` : false}}
            render={({field}) => (
              <OrganizationUnitTreePicker
                value={(field.value as string) ?? ''}
                onChange={(ouId: string) => {
                  field.onChange(ouId);
                  handleInputChangeFn(ref, ouId);
                }}
                rootOuId={additionalData?.['rootOuId'] as string | undefined}
              />
            )}
          />
          {formErrors[ref] && (
            <Typography variant="caption" color="error">
              {formErrors[ref]?.message as string}
            </Typography>
          )}
        </FormControl>
      );
    }

    if (type === 'SELECT' && options) {
      return (
        <FormControl key={component.id ?? index} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{resolve(labelText) ?? labelText}</FormLabel>
          <Controller
            name={ref}
            control={formControl}
            rules={{required: required ? `${resolve(labelText) ?? labelText} is required` : false}}
            render={({field}) => (
              <>
                <Select
                  {...field}
                  value={(field.value as string | undefined) ?? ''}
                  displayEmpty
                  size="small"
                  id={ref}
                  required={required}
                  fullWidth
                  disabled={isFormLoading}
                  error={!!formErrors[ref]}
                  onChange={(e) => {
                    field.onChange(e);
                    handleInputChangeFn(ref, String(e.target.value));
                  }}
                  renderValue={(selected) => {
                    if (!selected || selected === '') {
                      return (
                        <Typography sx={{color: 'text.secondary'}}>
                          {resolve(placeholderText) ?? 'Select an option'}
                        </Typography>
                      );
                    }
                    const selectedOption = options.find((opt: unknown) => getOptionValue(opt) === selected);
                    return selectedOption ? getOptionLabel(selectedOption) : String(selected);
                  }}
                >
                  <MenuItem value="" disabled>
                    {resolve(placeholderText) ?? 'Select an option'}
                  </MenuItem>
                  {options.map((option: unknown) => (
                    <MenuItem key={getOptionValue(option)} value={getOptionValue(option)}>
                      {getOptionLabel(option)}
                    </MenuItem>
                  ))}
                </Select>
                {formErrors[ref] && (
                  <Typography variant="caption" color="error.main" sx={{mt: 0.5}}>
                    {formErrors[ref]?.message as string}
                  </Typography>
                )}
                {hint && (
                  <Typography variant="caption" color="text.secondary">
                    {hint}
                  </Typography>
                )}
              </>
            )}
          />
        </FormControl>
      );
    }

    return null;
  };

  const {
    control,
    formState: {errors, isValid},
    reset,
    setValue,
  } = useForm({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: values ?? {},
  });

  useEffect(() => {
    if (!components?.length && Object.keys(values ?? {}).length === 0) {
      reset({});
    }
  }, [components, values, reset]);

  // Pre-select the root OU (user type's OU) when the OU_SELECT step renders.
  useEffect(() => {
    // Key matches BE constant AdditionalDataKeyRootOUID = "rootOuId"
    const rootOuId = additionalData?.['rootOuId'] as string | undefined;
    if (!rootOuId || !components?.length) return;

    const findOuSelectRef = (comps: EmbeddedFlowComponent[]): string | null => {
      for (const comp of comps) {
        if (comp.type === 'OU_SELECT' && comp.ref) return comp.ref;
        if (comp.components) {
          const found = findOuSelectRef(comp.components);
          if (found) return found;
        }
      }
      return null;
    };

    const ouRef = findOuSelectRef(components as EmbeddedFlowComponent[]);
    if (ouRef && !values?.[ouRef]) {
      setValue(ouRef, rootOuId, {shouldValidate: true});
      handleInputChange(ouRef, rootOuId);
    }
  }, [additionalData, components, values, setValue, handleInputChange]);

  // Loading
  if (isLoading && !components?.length) {
    return <PageLoadingAnimation />;
  }

  // Error without components
  if (error && !components?.length) {
    return (
      <Box>
        <Alert severity="error" sx={{mb: 2}}>
          <AlertTitle>{t('users:errors.failed.title', 'Error')}</AlertTitle>
          {error.message ?? t('users:errors.failed.description', 'An error occurred.')}
        </Alert>
        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button variant="outlined" onClick={handleClose}>
            {t('common:actions.close', 'Close')}
          </Button>
        </Box>
      </Box>
    );
  }

  // Loading components
  if (!components?.length) {
    return <PageLoadingAnimation />;
  }

  const hasInteractiveComponents = hasActionsOrInputs(components as EmbeddedFlowComponent[]);

  return (
    <>
      {(flowError ?? error) && (
        <Alert severity="error" sx={{mb: 2}}>
          <AlertTitle>{t('users:errors.failed.title', 'Error')}</AlertTitle>
          {flowError ?? error?.message ?? t('users:errors.failed.description', 'An error occurred.')}
        </Alert>
      )}
      <Stack direction="column" spacing={4}>
        {components.map((component: EmbeddedFlowComponent, index: number) => {
          // TEXT - render headings to match user creation wizard design
          if (String(component.type) === String(EmbeddedFlowComponentType.Text) || component.type === 'TEXT') {
            const variant = typeof component.variant === 'string' ? component.variant : undefined;
            const label = typeof component.label === 'string' ? component.label : '';
            const align =
              typeof (component as FlowSubComponent).align === 'string'
                ? ((component as FlowSubComponent).align as 'left' | 'center' | 'right')
                : undefined;

            if (variant === 'HEADING_1') {
              return (
                <Typography key={component.id ?? index} variant="h1" gutterBottom textAlign={align}>
                  {resolve(label) ?? label}
                </Typography>
              );
            }

            // Subtitles and body text
            return (
              <Typography
                key={component.id ?? index}
                variant={variant === 'HEADING_2' ? 'h2' : 'body1'}
                color="text.secondary"
                textAlign={align}
              >
                {resolve(label) ?? label}
              </Typography>
            );
          }

          // COPYABLE_TEXT - display text value with copy-to-clipboard button
          if (component.type === 'COPYABLE_TEXT') {
            return (
              <CopyableTextAdapter
                key={component.id ?? index}
                component={component as FlowComponent}
                resolve={resolve}
                additionalData={additionalData as Record<string, unknown> | undefined}
              />
            );
          }

          if (String(component.type) === String(EmbeddedFlowComponentType.Block) || component.type === 'BLOCK') {
            const blockComponents = (component.components ?? []) as FlowSubComponent[];

            const isAction = (c: FlowSubComponent) =>
              (String(c.type) === String(EmbeddedFlowComponentType.Action) || c.type === 'ACTION') &&
              (String(c.eventType) === String(EmbeddedFlowEventType.Submit) || c.eventType === 'SUBMIT');

            const submitActions = blockComponents.filter(isAction);
            // Also collect actions nested inside STACK children
            const nestedActions = blockComponents.flatMap((c) =>
              c.type === 'STACK' ? ((c.components ?? []) as FlowSubComponent[]).filter(isAction) : [],
            );
            const primaryAction = submitActions[0] ?? nestedActions[0];

            if (!primaryAction) return null;

            const isButtonDisabled = isLoading || !isValid || (propsIsValid !== undefined && !propsIsValid);

            return (
              <Box
                key={component.id ?? index}
                component="form"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (!isButtonDisabled) {
                    handleSubmit(primaryAction, values).catch(() => undefined);
                  }
                }}
                noValidate
                sx={{display: 'flex', flexDirection: 'column', width: '100%', gap: 2}}
              >
                {blockComponents.map((subComponent, compIndex) => {
                  const field = renderFormField(subComponent, compIndex, control, errors, isLoading, handleInputChange);
                  if (field) return field;

                  // STACK — render action children side by side
                  if (subComponent.type === 'STACK') {
                    const stackActions = (subComponent.components ?? []).filter(isAction) as FlowSubComponent[];
                    const direction = (subComponent.direction ?? 'row') as
                      | 'row'
                      | 'row-reverse'
                      | 'column'
                      | 'column-reverse';
                    const justify = subComponent.justify ?? 'flex-start';
                    return (
                      <Stack
                        key={subComponent.id ?? compIndex}
                        direction={direction}
                        spacing={2}
                        justifyContent={justify}
                        flexWrap="wrap"
                        sx={{mt: 2}}
                      >
                        {stackActions.map((action, actionIndex) => {
                          const actionKey = action.id ?? String(actionIndex);
                          const actionLabel = typeof action.label === 'string' ? action.label : '';
                          const isThisActionLoading = isLoading && activeActionId === actionKey;
                          return (
                            <Button
                              key={actionKey}
                              type="button"
                              variant={action.variant === 'PRIMARY' ? 'contained' : 'outlined'}
                              disabled={isButtonDisabled}
                              sx={{px: 4, py: 1.5}}
                              onClick={() => {
                                if (!isButtonDisabled) {
                                  setActiveActionId(actionKey);
                                  handleSubmit(action, values).catch(() => undefined);
                                }
                              }}
                            >
                              {isThisActionLoading ? (
                                <CircularProgress size={16} color="inherit" />
                              ) : (
                                (resolve(actionLabel) ?? actionLabel)
                              )}
                            </Button>
                          );
                        })}
                      </Stack>
                    );
                  }

                  if (!isAction(subComponent)) return null;

                  const subLabel = typeof subComponent.label === 'string' ? subComponent.label : '';

                  // Standard single-submit layout — right-aligned
                  return (
                    <Stack
                      key={subComponent.id ?? compIndex}
                      direction="row"
                      spacing={2}
                      justifyContent="flex-end"
                      sx={{mt: 4}}
                    >
                      <Button
                        type="button"
                        variant={subComponent.variant === 'PRIMARY' ? 'contained' : 'outlined'}
                        disabled={isButtonDisabled}
                        sx={{minWidth: 140}}
                        onClick={() => {
                          if (!isButtonDisabled) {
                            handleSubmit(subComponent, values).catch(() => undefined);
                          }
                        }}
                      >
                        {isLoading ? <CircularProgress size={20} color="inherit" /> : (resolve(subLabel) ?? subLabel)}
                      </Button>
                    </Stack>
                  );
                })}
              </Box>
            );
          }

          return null;
        })}
      </Stack>
      {!hasInteractiveComponents && (
        <Stack direction="row" spacing={2} justifyContent="center" sx={{mt: 4}}>
          <Button variant="outlined" onClick={handleClose}>
            {t('common:actions.close', 'Close')}
          </Button>
          <Button
            variant="contained"
            onClick={() => {
              resetFlow();
              onResetLocalState();
            }}
          >
            {t('users:addAnother', 'Add Another User')}
          </Button>
        </Stack>
      )}
    </>
  );
}

/** Inner component that bridges InviteUser render props with parent state via useEffect */
function InviteUserFlowBridge({
  renderProps,
  flowError,
  handleClose,
  onStepLabelChange,
  onInviteComplete,
  onOuStepDetected,
  onResetLocalState,
}: {
  renderProps: InviteUserRenderProps;
  flowError: string | null;
  handleClose: () => void;
  onStepLabelChange: (label: string) => void;
  onInviteComplete: () => void;
  onOuStepDetected: () => void;
  onResetLocalState: () => void;
}): JSX.Element {
  const {resolveFlowTemplateLiterals: rawResolve} = useThunderID();
  const resolve = useCallback((text?: string) => (text ? rawResolve(text) : undefined), [rawResolve]);
  const {t} = useTranslation();
  const components = renderProps.components as EmbeddedFlowComponent[] | undefined;

  // This page is only reached via the dedicated "invite" route (the create-vs-invite choice
  // now happens on AddUserPage), so auto-select the invite path and skip straight past the
  // onboarding-mode prompt if the flow still starts with it.
  const autoInviteTriggeredRef = useRef(false);
  const inviteAction = findActionComponentById(components, ONBOARDING_MODE_INVITE_ACTION_ID);

  useEffect(() => {
    if (inviteAction && !autoInviteTriggeredRef.current) {
      autoInviteTriggeredRef.current = true;
      renderProps.handleSubmit(inviteAction, renderProps.values).catch(() => undefined);
    }
  }, [inviteAction, renderProps]);

  // Clear the auto-invite guard on reset so the restarted prompt is auto-submitted again.
  const handleReset = useCallback(() => {
    autoInviteTriggeredRef.current = false;
    onResetLocalState();
  }, [onResetLocalState]);

  // Derive current step label from the HEADING_1 component
  const currentStepLabel = components?.length ? deriveStepLabel(components, resolve, t) : '';

  const isDisplayOnly = !!components?.length && !hasActionsOrInputs(components);

  // Detect OU step presence to adjust progress calculation
  const currentHasOu =
    components?.some((c) => c.type === 'OU_SELECT' || c.components?.some((sub) => sub.type === 'OU_SELECT')) ?? false;

  // Update breadcrumb trail and OU detection via useEffect to avoid render-time state updates
  useEffect(() => {
    if (currentHasOu) {
      onOuStepDetected();
    }
  }, [currentHasOu, onOuStepDetected]);

  useEffect(() => {
    if (currentStepLabel && !inviteAction) {
      onStepLabelChange(currentStepLabel);
    }
  }, [currentStepLabel, inviteAction, onStepLabelChange]);

  useEffect(() => {
    if (isDisplayOnly) {
      onInviteComplete();
    }
  }, [isDisplayOnly, onInviteComplete]);

  if (inviteAction) {
    return <PageLoadingAnimation />;
  }

  return (
    <InviteUserStepContent
      renderProps={renderProps}
      flowError={flowError}
      handleClose={handleClose}
      onResetLocalState={handleReset}
    />
  );
}

export default function UserInvitePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('UserInvitePage');
  const [flowError, setFlowError] = useState<string | null>(null);

  // Track breadcrumb trail of visited step labels
  const [breadcrumbs, setBreadcrumbs] = useState<string[]>([]);
  const prevStepLabelRef = useRef<string>('');
  const [hasOuStep, setHasOuStep] = useState(false);
  const [isComplete, setIsComplete] = useState(false);

  const handleClose = useCallback(() => {
    (async () => {
      await navigate('/users');
    })().catch((err: unknown) => {
      logger.error('Failed to navigate to users page', {error: err});
    });
  }, [navigate, logger]);

  const handleBreadcrumbHome = useCallback(() => {
    (async () => {
      await navigate('/users/add');
    })().catch((err: unknown) => {
      logger.error('Failed to navigate to add user page', {error: err});
    });
  }, [navigate, logger]);

  const handleManualCreateFallback = useCallback(() => {
    logger.info('Falling back to manual user creation because the onboarding flow is unavailable');
    (async () => {
      await navigate('/users/add/create');
    })().catch((err: unknown) => {
      logger.error('Failed to navigate to fallback user creation page', {error: err});
    });
  }, [navigate, logger]);

  const handleStepLabelChange = useCallback(
    (label: string) => {
      if (label !== prevStepLabelRef.current) {
        prevStepLabelRef.current = label;
        setBreadcrumbs((prev) => {
          const existingIndex = prev.indexOf(label);
          if (existingIndex >= 0) {
            return prev.slice(0, existingIndex + 1);
          }
          return [...prev, label];
        });
      }
    },
    [setBreadcrumbs],
  );

  const handleInviteComplete = useCallback(() => {
    if (prevStepLabelRef.current !== 'complete') {
      prevStepLabelRef.current = 'complete';
      setBreadcrumbs((prev) => [...prev, t('users:invite.steps.complete', 'Complete')]);
      setIsComplete(true);
    }
  }, [setBreadcrumbs, t]);

  const handleOuStepDetected = useCallback(() => {
    setHasOuStep(true);
  }, []);

  const handleResetLocalState = useCallback(() => {
    setBreadcrumbs([]);
    prevStepLabelRef.current = '';
    setHasOuStep(false);
    setFlowError(null);
    setIsComplete(false);
  }, []);

  // Compute progress from breadcrumb trail.
  // Without OU step: user type, onboarding choice, email/details, completion path.
  // With OU step: add one extra OU selection step.
  const totalSteps = hasOuStep ? 5 : 4;
  const progress = Math.min((breadcrumbs.length / totalSteps) * 100, 100);

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      {/* Progress bar */}
      <LinearProgress variant="determinate" value={progress} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        {/* Header with close button and breadcrumb */}
        <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Stack direction="row" alignItems="center" spacing={2}>
            <IconButton
              aria-label={t('common:actions.close', 'Close')}
              onClick={handleClose}
              sx={{
                bgcolor: 'background.paper',
                '&:hover': {bgcolor: 'action.hover'},
                boxShadow: 1,
              }}
            >
              <X size={24} />
            </IconButton>
            <AppBreadcrumbs
              items={[
                {key: 'add-user', label: t('users:addUser', 'Add User'), onClick: handleBreadcrumbHome},
                {key: 'invite-user', label: t('users:invite.title', 'Invite User')},
                ...breadcrumbs.map((label) => ({key: label, label})),
              ]}
            />
          </Stack>
        </Box>

        {/* Main content */}
        <Box sx={{flex: 1, display: 'flex', minHeight: 0}}>
          <Box
            sx={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              pt: isComplete ? 2 : 8,
              pb: 8,
              px: 20,
              mx: 'auto',
              alignItems: isComplete ? 'flex-start' : 'flex-start',
              justifyContent: 'flex-start',
            }}
          >
            <Box
              sx={{
                width: '100%',
                maxWidth: 800,
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
              }}
            >
              <InviteUser
                onError={(err: Error) => {
                  if (isMissingOnboardingFlow(err)) {
                    handleManualCreateFallback();
                    return;
                  }
                  logger.error('User onboarding error', {error: err});
                }}
                onFlowChange={(response: any) => {
                  if (isMissingOnboardingFlow(response)) {
                    handleManualCreateFallback();
                    return;
                  }
                  // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
                  const messageKey: string | undefined = response?.error?.message?.key;
                  if (messageKey) {
                    const translated: string = t(messageKey);
                    if (translated !== messageKey) {
                      setFlowError(translated);

                      return;
                    }
                  }
                  // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
                  const fallback: string | undefined =
                    response?.error?.message?.defaultValue ?? response?.error?.description?.defaultValue;
                  setFlowError(fallback ?? null);
                }}
              >
                {(renderProps: InviteUserRenderProps) => (
                  <InviteUserFlowBridge
                    renderProps={renderProps}
                    flowError={flowError}
                    handleClose={handleClose}
                    onStepLabelChange={handleStepLabelChange}
                    onInviteComplete={handleInviteComplete}
                    onOuStepDetected={handleOuStepDetected}
                    onResetLocalState={handleResetLocalState}
                  />
                )}
              </InviteUser>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
