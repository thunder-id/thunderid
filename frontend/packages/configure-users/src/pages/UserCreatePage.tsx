// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {useLogger} from '@thunderid/logger/react';
import {
  EmbeddedFlowComponentType,
  InviteUser,
  useThunderID,
  type EmbeddedFlowComponent,
  type InviteUserRenderProps,
} from '@thunderid/react';
import {
  Alert,
  AlertTitle,
  Stack,
  Typography,
  Button,
  TextField,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormLabel,
  Select,
  MenuItem,
} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState, useCallback, useEffect, useRef} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useUserRoutes from '../hooks/useUserRoutes';
import getUserErrorMessage from '../utils/getUserErrorMessage';

const ONBOARDING_MODE_CREATE_ACTION_ID = 'action_create_user_now';

/** Typed shape for flow sub-components */
type FlowSubComponent = EmbeddedFlowComponent & {
  components?: EmbeddedFlowComponent[];
  hint?: string;
  options?: unknown[];
  placeholder?: string;
  required?: boolean;
  variant?: string;
};

/** Collects the refs of every boolean field in a component tree. */
function collectBooleanRefs(comps: EmbeddedFlowComponent[]): string[] {
  const refs: string[] = [];
  comps.forEach((comp) => {
    if (comp.type === 'BOOLEAN_INPUT' && typeof comp.ref === 'string') {
      refs.push(comp.ref);
    }
    const nested = (comp as FlowSubComponent).components;
    if (nested) refs.push(...collectBooleanRefs(nested));
  });
  return refs;
}

function UserCreateStepContent({
  renderProps,
  error,
  onFieldChange,
  onStepLabelChange,
}: {
  renderProps: InviteUserRenderProps;
  error: string | null;
  onFieldChange: () => void;
  onStepLabelChange: (label: string) => void;
}): JSX.Element {
  const {resolveFlowTemplateLiterals: rawResolve} = useThunderID();
  const resolve = useCallback((text?: string) => (text ? rawResolve(text) : undefined), [rawResolve]);
  const {t} = useTranslation();
  const components = renderProps.components as EmbeddedFlowComponent[] | undefined;

  // Auto-select create action if present
  const createAction = components?.find((c) => c.id === ONBOARDING_MODE_CREATE_ACTION_ID);

  const submittedActionIdRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (createAction && submittedActionIdRef.current !== createAction.id) {
      submittedActionIdRef.current = createAction.id;
      renderProps.handleSubmit(createAction, renderProps.values).catch(() => undefined);
    }
  }, [createAction, renderProps]);

  // Derive step label from HEADING_1 component
  const stepLabel = components?.find(
    (c) =>
      (String(c.type) === String(EmbeddedFlowComponentType.Text) || c.type === 'TEXT') &&
      (c as FlowSubComponent).variant === 'HEADING_1',
  );

  useEffect(() => {
    if (stepLabel && typeof stepLabel.label === 'string') {
      onStepLabelChange(t(resolve(stepLabel.label) ?? stepLabel.label));
    }
  }, [stepLabel, resolve, t, onStepLabelChange]);

  const {values, additionalData, handleInputChange: rawHandleInputChange} = renderProps;
  const handleInputChange = useCallback(
    (name: string, value: string) => {
      onFieldChange(); // a create failure is stale once the form changes
      rawHandleInputChange(name, value);
    },
    [rawHandleInputChange, onFieldChange],
  );

  // Seed boolean fields with their unchecked value. A boolean answer is meaningful even when the
  // user never touches the field, and without a seeded value a required boolean attribute is absent
  // from the submission and can never be satisfied. Seeding goes through the raw handler so it does
  // not clear a create error the user has not acted on yet.
  useEffect(() => {
    if (!components?.length) return;
    const current = values as Record<string, unknown> | undefined;
    collectBooleanRefs(components).forEach((ref) => {
      if (current?.[ref] === undefined) {
        rawHandleInputChange(ref, 'false');
      }
    });
  }, [components, values, rawHandleInputChange]);

  const renderComponent = (component: EmbeddedFlowComponent, index: number): JSX.Element | null => {
    // Render text components
    if (String(component.type) === String(EmbeddedFlowComponentType.Text) || component.type === 'TEXT') {
      const label = typeof component.label === 'string' ? component.label : '';
      const variant = (component as FlowSubComponent).variant;

      if (variant === 'HEADING_1') {
        return (
          <Typography key={component.id ?? index} variant="h1" gutterBottom>
            {t(resolve(label) ?? label)}
          </Typography>
        );
      }

      return (
        <Typography key={component.id ?? index} variant="body1" color="text.secondary">
          {t(resolve(label) ?? label)}
        </Typography>
      );
    }

    // Render form fields
    if (
      component.type === 'EMAIL_INPUT' ||
      component.type === 'TEXT_INPUT' ||
      component.type === 'PHONE_INPUT' ||
      component.type === 'NUMBER_INPUT' ||
      component.type === 'PASSWORD_INPUT'
    ) {
      const ref = component.ref;
      const label = typeof component.label === 'string' ? component.label : '';
      const placeholder = (component as FlowSubComponent).placeholder ?? '';
      const required = (component as FlowSubComponent).required ?? false;

      if (!ref) return null;

      const value = ((values as Record<string, unknown>)?.[ref] as string) ?? '';
      let inputType = 'text';

      if (component.type === 'EMAIL_INPUT') inputType = 'email';
      if (component.type === 'PASSWORD_INPUT') inputType = 'password';
      if (component.type === 'PHONE_INPUT') inputType = 'tel';
      if (component.type === 'NUMBER_INPUT') inputType = 'number';

      return (
        <FormControl key={component.id ?? index} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{t(resolve(label) ?? label)}</FormLabel>
          <TextField
            id={ref}
            type={inputType}
            size="small"
            variant="outlined"
            placeholder={resolve(placeholder) ?? placeholder}
            value={value}
            required={required}
            onChange={(e) => handleInputChange(ref, e.target.value)}
          />
        </FormControl>
      );
    }

    // Render SELECT
    if (component.type === 'SELECT') {
      const ref = component.ref;
      const label = typeof component.label === 'string' ? component.label : '';
      const options = (component as FlowSubComponent).options ?? [];
      const required = (component as FlowSubComponent).required ?? false;

      if (!ref || !options.length) return null;

      const value = ((values as Record<string, unknown>)?.[ref] as string) ?? '';

      return (
        <FormControl key={component.id ?? index} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{t(resolve(label) ?? label)}</FormLabel>
          <Select
            id={ref}
            value={value}
            size="small"
            displayEmpty
            required={required}
            onChange={(e) => handleInputChange(ref, String(e.target.value))}
          >
            <MenuItem value="">{t('Select an option')}</MenuItem>
            {options.map((opt: unknown) => {
              const optValue =
                typeof opt === 'object' && opt !== null ? (opt as Record<string, unknown>)['value'] : opt;
              const optLabel =
                typeof opt === 'object' && opt !== null ? (opt as Record<string, unknown>)['label'] : opt;
              const keyValue = String(optValue);
              return (
                <MenuItem key={keyValue} value={keyValue}>
                  {String(optLabel)}
                </MenuItem>
              );
            })}
          </Select>
        </FormControl>
      );
    }

    // Render BOOLEAN_INPUT
    if (component.type === 'BOOLEAN_INPUT') {
      const ref = component.ref;
      const label = typeof component.label === 'string' ? component.label : '';
      const required = (component as FlowSubComponent).required ?? false;

      if (!ref) return null;

      const checked = (values as Record<string, unknown>)?.[ref] === 'true';

      return (
        <FormControl key={component.id ?? index} required={required}>
          <FormControlLabel
            control={
              <Checkbox
                id={ref}
                size="small"
                checked={checked}
                onChange={(e) => handleInputChange(ref, String(e.target.checked))}
              />
            }
            label={t(resolve(label) ?? label)}
          />
        </FormControl>
      );
    }

    // Render OU_SELECT
    if (component.type === 'OU_SELECT') {
      const ref = component.ref;
      const label = typeof component.label === 'string' ? component.label : '';
      const required = (component as FlowSubComponent).required ?? false;

      if (!ref) return null;

      const value = ((values as Record<string, unknown>)?.[ref] as string) ?? '';
      const rootOuId = additionalData?.['rootOuId'] as string | undefined;

      return (
        <FormControl key={component.id ?? index} fullWidth required={required}>
          <FormLabel htmlFor={ref}>{t(resolve(label) ?? label)}</FormLabel>
          <OrganizationUnitTreePicker
            value={value}
            onChange={(ouId: string) => handleInputChange(ref, ouId)}
            rootOuId={rootOuId}
          />
        </FormControl>
      );
    }

    // Render ACTION buttons (submit buttons)
    if (component.type === 'ACTION') {
      const label = typeof component.label === 'string' ? component.label : '';
      const variant = (component as FlowSubComponent).variant;

      return (
        <Button
          key={component.id ?? index}
          variant={variant === 'OUTLINED' ? 'outlined' : 'contained'}
          onClick={() => {
            renderProps.handleSubmit(component, renderProps.values).catch(() => undefined);
          }}
          fullWidth
          size="large"
        >
          {t(resolve(label) ?? label)}
        </Button>
      );
    }

    // Render nested components (BLOCK, STACK)
    if ((component as FlowSubComponent).components) {
      const nested = (component as FlowSubComponent).components!;
      return (
        <Stack key={component.id ?? index} direction="column" spacing={2}>
          {nested.map((c, idx) => renderComponent(c, idx))}
        </Stack>
      );
    }

    return null;
  };

  return (
    <Stack direction="column" spacing={4}>
      {error && (
        <Alert severity="error">
          <AlertTitle>{t('users:errors.failed.title', 'Error')}</AlertTitle>
          {error}
        </Alert>
      )}
      {components?.map((component: EmbeddedFlowComponent, index: number) => renderComponent(component, index))}
    </Stack>
  );
}

export default function UserCreatePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useUserRoutes();
  const logger = useLogger('UserCreatePage');
  const [breadcrumbs, setBreadcrumbs] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  const handleClose = useCallback(() => {
    Promise.resolve(navigate(routes.list())).catch((err: unknown) => {
      logger.error('Failed to navigate to users page', {error: err});
    });
  }, [navigate, routes, logger]);

  const handleStepLabelChange = useCallback(
    (label: string) => {
      setBreadcrumbs([t('users:addUser', 'Add User'), label]);
    },
    [t],
  );

  const handleError = useCallback(
    (err: Error) => {
      logger.error('Failed to create user', {error: err});
      setError(
        getUserErrorMessage(
          err,
          (key, options) => t(key.includes(':') ? key : `users:${key}`, options),
          'errors.failed.description',
          'An error occurred. Please try again.',
        ),
      );
    },
    [logger, t],
  );

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={0}
      breadcrumbItems={breadcrumbs.map((label, idx) => ({
        key: `breadcrumb-${idx}`,
        label,
      }))}
      footer={null}
    >
      <InviteUser onError={handleError}>
        {(renderProps: InviteUserRenderProps) => (
          <UserCreateStepContent
            renderProps={renderProps}
            error={error}
            onFieldChange={() => setError(null)}
            onStepLabelChange={handleStepLabelChange}
          />
        )}
      </InviteUser>
    </FullScreenCreationWizardLayout>
  );
}
