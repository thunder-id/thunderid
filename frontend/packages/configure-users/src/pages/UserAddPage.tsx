// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {FullScreenCreationWizardLayout, PageLoadingAnimation} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {CopyableTextAdapter, type FlowComponent, mapEmbeddedFlowTextColor} from '@thunderid/design';
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
import {EMAIL_REGEX} from '@thunderid/utils';
import {
  Box,
  Stack,
  Typography,
  Button,
  Alert,
  AlertTitle,
  TextField,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormLabel,
  Select,
  MenuItem,
  CircularProgress,
  Card,
  CardActionArea,
  CardContent,
} from '@wso2/oxygen-ui';
import {UserPlus, Send} from '@wso2/oxygen-ui-icons-react';
import {useState, useEffect, useMemo, useCallback, useRef, type JSX} from 'react';
import {useForm, Controller} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import {z} from 'zod';
import CredentialFieldInput from '../components/CredentialFieldInput';
import useUserRoutes from '../hooks/useUserRoutes';
import getUserErrorMessage from '../utils/getUserErrorMessage';

/** Typed shape for flow sub-components */
type FlowSubComponent = EmbeddedFlowComponent & {
  align?: string;
  color?: string;
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
  onClearFlowError,
}: {
  renderProps: InviteUserRenderProps;
  flowError: string | null;
  handleClose: () => void;
  onResetLocalState: () => void;
  onClearFlowError: () => void;
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

  // A submit failure is stale once the user edits a field. Only user-driven edits clear it: the
  // automatic organization unit prefill below keeps the raw handler so it cannot wipe a fresh error.
  const handleUserInputChange = useCallback(
    (field: string, value: string): void => {
      onClearFlowError();
      handleInputChange(field, value);
    },
    [handleInputChange, onClearFlowError],
  );

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
                comp.type === 'BOOLEAN_INPUT' ||
                comp.type === 'NUMBER_INPUT' ||
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
              if (comp.type === 'BOOLEAN_INPUT') {
                // A required boolean is satisfied by either answer, so it carries no min-length rule.
                shape[comp.ref] = fieldSchema;
                return;
              }
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

    if (
      String(type) === String(EmbeddedFlowComponentType.TextInput) ||
      type === 'TEXT_INPUT' ||
      type === 'NUMBER_INPUT'
    ) {
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
                type={type === 'NUMBER_INPUT' ? 'number' : 'text'}
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
              pattern: {
                value: EMAIL_REGEX,
                message: t('validations:field.email.invalid', 'Please enter a valid email address.'),
              },
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

    if (type === 'BOOLEAN_INPUT') {
      return (
        <FormControl key={component.id ?? index} required={required}>
          <Controller
            name={ref}
            control={formControl}
            render={({field}) => (
              <FormControlLabel
                control={
                  <Checkbox
                    id={ref}
                    size="small"
                    disabled={isFormLoading}
                    checked={field.value === 'true'}
                    onChange={(e) => {
                      const next = String(e.target.checked);
                      field.onChange(next);
                      handleInputChangeFn(ref, next);
                    }}
                  />
                }
                label={resolve(labelText) ?? labelText}
              />
            )}
          />
          {hint && (
            <Typography variant="caption" color="text.secondary">
              {hint}
            </Typography>
          )}
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

  // Seed boolean fields with their unchecked value. A boolean answer is meaningful even when the
  // user never touches the field, and without a seeded value a required boolean attribute is
  // absent from the submission and can never be satisfied.
  useEffect(() => {
    if (!components?.length) return;

    const findBooleanRefs = (comps: EmbeddedFlowComponent[]): string[] => {
      const refs: string[] = [];
      for (const comp of comps) {
        if (comp.type === 'BOOLEAN_INPUT' && comp.ref) refs.push(comp.ref);
        if (comp.components) refs.push(...findBooleanRefs(comp.components));
      }
      return refs;
    };

    findBooleanRefs(components as EmbeddedFlowComponent[]).forEach((booleanRef) => {
      if (values?.[booleanRef] === undefined) {
        setValue(booleanRef, 'false', {shouldValidate: true});
        handleInputChange(booleanRef, 'false');
      }
    });
  }, [components, values, setValue, handleInputChange]);

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
          {getUserErrorMessage(
            error,
            (key, options) => t(key.includes(':') ? key : `users:${key}`, options),
            'errors.failed.description',
            'An error occurred. Please try again.',
          )}
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
          {flowError ??
            (error &&
              getUserErrorMessage(
                error,
                (key, options) => t(key.includes(':') ? key : `users:${key}`, options),
                'errors.failed.description',
                'An error occurred. Please try again.',
              ))}
        </Alert>
      )}
      <Stack direction="column" spacing={4}>
        {components.map((component: EmbeddedFlowComponent, index: number) => {
          // TEXT - render headings to match user creation wizard design
          if (String(component.type) === String(EmbeddedFlowComponentType.Text) || component.type === 'TEXT') {
            const variant = typeof component.variant === 'string' ? component.variant : undefined;
            const label = typeof component.label === 'string' ? component.label : '';
            const subComponent = component as FlowSubComponent;
            const align = subComponent.align as 'left' | 'center' | 'right' | undefined;
            const textColor =
              mapEmbeddedFlowTextColor(typeof subComponent.color === 'string' ? subComponent.color : undefined) ??
              (variant === 'HEADING_1' ? undefined : 'textSecondary');

            if (variant === 'HEADING_1') {
              return (
                <Typography key={component.id ?? index} variant="h1" gutterBottom textAlign={align} color={textColor}>
                  {resolve(label) ?? label}
                </Typography>
              );
            }

            // Subtitles and body text
            return (
              <Typography
                key={component.id ?? index}
                variant={variant === 'HEADING_2' ? 'h2' : 'body1'}
                color={textColor}
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
                  const field = renderFormField(
                    subComponent,
                    compIndex,
                    control,
                    errors,
                    isLoading,
                    handleUserInputChange,
                  );
                  if (field) return field;

                  // STACK — render action children side by side
                  if (subComponent.type === 'STACK') {
                    const stackActions = (subComponent.components ?? []).filter(isAction) as FlowSubComponent[];
                    const direction = (subComponent.direction ?? 'row') as
                      | 'row'
                      | 'row-reverse'
                      | 'column'
                      | 'column-reverse';
                    const justify = subComponent.justify ?? 'center';
                    const isOnboardingModeStack = subComponent.id === 'stack_onboarding_mode_actions';

                    // Render as cards only for onboarding mode stack
                    if (isOnboardingModeStack) {
                      const getActionMetadata = (id: string | undefined) => {
                        if (id === 'action_create_user_now') {
                          return {
                            icon: <UserPlus size={28} />,
                            descriptionKey: 'onboarding:forms.onboarding_mode.actions.create.description',
                            descriptionDefault: 'Create user account immediately with all details',
                          };
                        }
                        if (id === 'action_invite_user') {
                          return {
                            icon: <Send size={28} />,
                            descriptionKey: 'onboarding:forms.onboarding_mode.actions.invite.description',
                            descriptionDefault: 'Send invitation for user to complete their profile',
                          };
                        }
                        return {icon: null, descriptionKey: '', descriptionDefault: ''};
                      };

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
                            const metadata = getActionMetadata(action.id);

                            return (
                              <Card key={actionKey} variant="outlined" sx={{flex: 1, minWidth: 200}}>
                                <CardActionArea
                                  disabled={isButtonDisabled}
                                  onClick={() => {
                                    if (!isButtonDisabled) {
                                      setActiveActionId(actionKey);
                                      handleSubmit(action, values).catch(() => undefined);
                                    }
                                  }}
                                  sx={{
                                    height: '100%',
                                    p: 2,
                                    transition: 'all 0.2s ease-in-out',
                                    '&:hover:not([aria-disabled="true"])': {
                                      borderColor: 'primary.main',
                                      bgcolor: 'action.hover',
                                    },
                                  }}
                                >
                                  <CardContent sx={{p: 0, '&:last-child': {pb: 0}}}>
                                    <Stack direction="column" spacing={1.5} alignItems="flex-start">
                                      {metadata.icon && <Box sx={{color: 'text.secondary'}}>{metadata.icon}</Box>}
                                      <Stack direction="column" spacing={0.5}>
                                        <Typography variant="subtitle1" sx={{fontWeight: 500}}>
                                          {isThisActionLoading ? (
                                            <CircularProgress size={16} color="inherit" />
                                          ) : (
                                            (resolve(actionLabel) ?? actionLabel)
                                          )}
                                        </Typography>
                                        {metadata.descriptionKey && (
                                          <Typography variant="body2" color="text.secondary">
                                            {t(metadata.descriptionKey, metadata.descriptionDefault)}
                                          </Typography>
                                        )}
                                      </Stack>
                                    </Stack>
                                  </CardContent>
                                </CardActionArea>
                              </Card>
                            );
                          })}
                        </Stack>
                      );
                    }

                    // Fallback: render as generic buttons for other stacks
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
        <Stack direction="row" spacing={2} justifyContent="flex-start" sx={{mt: 4}}>
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
  onClearFlowError,
  onResetFlowAvailable = undefined,
}: {
  renderProps: InviteUserRenderProps;
  flowError: string | null;
  handleClose: () => void;
  onStepLabelChange: (label: string) => void;
  onInviteComplete: () => void;
  onOuStepDetected: () => void;
  onResetLocalState: () => void;
  onClearFlowError: () => void;
  onResetFlowAvailable?: (resetFn: () => void) => void;
}): JSX.Element {
  const {resolveFlowTemplateLiterals: rawResolve} = useThunderID();
  const resolve = useCallback((text?: string) => (text ? rawResolve(text) : undefined), [rawResolve]);
  const {t} = useTranslation();
  const components = renderProps.components as EmbeddedFlowComponent[] | undefined;

  // Derive current step label from the HEADING_1 component
  const currentStepLabel = components?.length ? deriveStepLabel(components, resolve, t) : '';

  const isDisplayOnly = !!components?.length && !hasActionsOrInputs(components);

  // Detect OU step presence to adjust progress calculation
  const currentHasOu =
    components?.some((c) => c.type === 'OU_SELECT' || c.components?.some((sub) => sub.type === 'OU_SELECT')) ?? false;

  // Expose resetFlow to parent
  useEffect(() => {
    if (onResetFlowAvailable && renderProps.resetFlow) {
      onResetFlowAvailable(renderProps.resetFlow);
    }
  }, [onResetFlowAvailable, renderProps.resetFlow]);

  // Update breadcrumb trail and OU detection via useEffect to avoid render-time state updates
  useEffect(() => {
    if (currentHasOu) {
      onOuStepDetected();
    }
  }, [currentHasOu, onOuStepDetected]);

  useEffect(() => {
    if (currentStepLabel) {
      onStepLabelChange(currentStepLabel);
    }
  }, [currentStepLabel, onStepLabelChange]);

  useEffect(() => {
    if (isDisplayOnly) {
      onInviteComplete();
    }
  }, [isDisplayOnly, onInviteComplete]);

  return (
    <InviteUserStepContent
      renderProps={renderProps}
      flowError={flowError}
      handleClose={handleClose}
      onResetLocalState={onResetLocalState}
      onClearFlowError={onClearFlowError}
    />
  );
}

export default function UserAddPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('UserAddPage');
  const routes = useUserRoutes();
  const [flowError, setFlowError] = useState<string | null>(null);
  const resetFlowRef = useRef<(() => void) | null>(null);

  // Track breadcrumb trail of visited step labels, starting with "Add User"
  const [breadcrumbs, setBreadcrumbs] = useState<string[]>([t('users:addUser', 'Add User')]);
  const prevStepLabelRef = useRef<string>('');
  const [hasOuStep, setHasOuStep] = useState(false);

  const handleClose = useCallback(() => {
    Promise.resolve(navigate(-1)).catch((error: unknown) => {
      logger.error('Failed to navigate back', {error});
    });
  }, [navigate, logger]);

  const handleManualCreateFallback = useCallback(() => {
    logger.info('Falling back to manual user creation because the onboarding flow is unavailable');
    (async () => {
      await navigate(routes.addCreate());
    })().catch((err: unknown) => {
      logger.error('Failed to navigate to fallback user creation page', {error: err});
    });
  }, [navigate, routes, logger]);

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
    }
  }, [setBreadcrumbs, t]);

  const handleOuStepDetected = useCallback(() => {
    setHasOuStep(true);
  }, []);

  const handleClearFlowError = useCallback(() => {
    setFlowError(null);
  }, []);

  const handleResetLocalState = useCallback(() => {
    setBreadcrumbs([t('users:addUser', 'Add User')]);
    prevStepLabelRef.current = '';
    setHasOuStep(false);
    setFlowError(null);
  }, [t]);

  // Compute progress from breadcrumb trail.
  // Without OU step: user type, onboarding choice, email/details, completion path.
  // With OU step: add one extra OU selection step.
  const totalSteps = hasOuStep ? 5 : 4;
  const progress = Math.min((breadcrumbs.length / totalSteps) * 100, 100);

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={progress}
      breadcrumbItems={breadcrumbs.map((label, index) => {
        const isFirstItem = index === 0;

        return {
          key: `breadcrumb-${index}`,
          label,
          onClick: isFirstItem
            ? () => {
                if (resetFlowRef.current) {
                  resetFlowRef.current();
                  handleResetLocalState();
                }
              }
            : undefined,
          disabled: !isFirstItem,
        };
      })}
      footer={null}
    >
      <InviteUser
        onError={(err: Error) => {
          if (isMissingOnboardingFlow(err)) {
            handleManualCreateFallback();
            return;
          }
          logger.error('User onboarding error', {error: err});
          // onFlowChange runs first and sees the full error envelope, including the code. The SDK
          // flattens that envelope into a plain Error before onError, so only fill in here when
          // onFlowChange had nothing to report (for example, a thrown network failure).
          setFlowError(
            (current) =>
              current ??
              getUserErrorMessage(
                err,
                (key, options) => t(key.includes(':') ? key : `users:${key}`, options),
                'errors.failed.description',
                'An error occurred. Please try again.',
              ),
          );
        }}
        onFlowChange={(response) => {
          if (isMissingOnboardingFlow(response)) {
            handleManualCreateFallback();
            return;
          }
          if (!response?.error) {
            setFlowError(null);
            return;
          }
          setFlowError(
            getUserErrorMessage(
              response as unknown as Error,
              (key, options) => t(key.includes(':') ? key : `users:${key}`, options),
              'errors.failed.description',
              'An error occurred. Please try again.',
            ),
          );
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
            onClearFlowError={handleClearFlowError}
            onResetFlowAvailable={(resetFn) => {
              resetFlowRef.current = resetFn;
            }}
          />
        )}
      </InviteUser>
    </FullScreenCreationWizardLayout>
  );
}
