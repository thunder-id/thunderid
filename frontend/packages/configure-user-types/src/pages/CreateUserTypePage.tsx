// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  OrganizationUnitPickerScreen,
  useGetOrganizationUnit,
  useHasMultipleOUs,
} from '@thunderid/configure-organization-units';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Stack, Button, CircularProgress, IconButton, LinearProgress, Alert, AppBreadcrumbs} from '@wso2/oxygen-ui';
import {Home, X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useEffect, useMemo} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateUserType from '../api/useCreateUserType';
import ConfigureName from '../components/create-user-type/ConfigureName';
import ConfigureProperties from '../components/create-user-type/ConfigureProperties';
import UserTypeConstraints from '../constants/user-type-constraints';
import useUserTypeCreate from '../contexts/UserTypeCreate/useUserTypeCreate';
import useUserTypeRoutes from '../hooks/useUserTypeRoutes';
import {UserTypeCreateFlowStep} from '../models/user-type-create-flow';
import type {PropertyDefinition, UserTypeDefinition, CreateUserTypeRequest} from '../types/user-types';

export default function CreateUserTypePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('CreateUserTypePage');
  const createUserTypeMutation = useCreateUserType();
  const routes = useUserTypeRoutes();

  const {
    currentStep,
    setCurrentStep,
    name,
    setName,
    ouId,
    setOuId,
    allowSelfRegistration,
    setAllowSelfRegistration,
    properties,
    setProperties,
    enumInput,
    setEnumInput,
    displayAttribute,
    setDisplayAttribute,
    error,
    setError,
  } = useUserTypeCreate();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, resolve it automatically and skip straight
  // past it.
  useEffect(() => {
    if (isOuLoading || hasMultipleOUs || currentStep !== UserTypeCreateFlowStep.ORGANIZATION_UNIT) return;
    setOuId(ouList[0]?.id ?? '');
    setCurrentStep(UserTypeCreateFlowStep.NAME);
  }, [isOuLoading, hasMultipleOUs, ouList, currentStep, setOuId, setCurrentStep]);

  // The organization unit whose name is shown in the Details step's summary chip.
  const resolvedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    resolvedOuId,
    Boolean(resolvedOuId),
  );

  const activeSteps = useMemo((): UserTypeCreateFlowStep[] => {
    const base: UserTypeCreateFlowStep[] = [];
    if (hasMultipleOUs) base.push(UserTypeCreateFlowStep.ORGANIZATION_UNIT);
    base.push(UserTypeCreateFlowStep.NAME, UserTypeCreateFlowStep.PROPERTIES);
    return base;
  }, [hasMultipleOUs]);

  const steps: Partial<Record<UserTypeCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<UserTypeCreateFlowStep, {label: string}>> = {};
    if (hasMultipleOUs) {
      map.ORGANIZATION_UNIT = {label: t('userTypes:createWizard.steps.organizationUnit', 'Organization Unit')};
    }
    map.NAME = {label: t('userTypes:createWizard.steps.name', 'Details')};
    map.PROPERTIES = {label: t('userTypes:createWizard.steps.properties', 'Properties')};
    return map;
  }, [t, hasMultipleOUs]);

  const [stepReady, setStepReady] = useState<Record<UserTypeCreateFlowStep, boolean>>({
    ORGANIZATION_UNIT: false,
    NAME: false,
    PROPERTIES: false,
  });

  const handleClose = (): void => {
    void navigate(routes.list());
  };

  // A create failure is stale once the user edits any field, so every field-change path below
  // clears both the validation error and the mutation's own error before applying the change.
  // Only reset the mutation once it has actually failed: resetting while it's still pending would
  // flip isPending back to false and re-enable the submit button before the in-flight request
  // settles, letting the user fire a second concurrent create.
  const clearCreateError = useCallback((): void => {
    setError(null);
    if (createUserTypeMutation.isError) {
      createUserTypeMutation.reset();
    }
  }, [setError, createUserTypeMutation]);

  const handleNameChange = useCallback(
    (newName: string): void => {
      clearCreateError();
      setName(newName);
    },
    [clearCreateError, setName],
  );

  const handleOuIdChange = useCallback(
    (newOuId: string): void => {
      clearCreateError();
      setOuId(newOuId);
    },
    [clearCreateError, setOuId],
  );

  const handleAllowSelfRegistrationChange = useCallback(
    (allow: boolean): void => {
      clearCreateError();
      setAllowSelfRegistration(allow);
    },
    [clearCreateError, setAllowSelfRegistration],
  );

  const handlePropertiesChange = useCallback(
    (newProperties: typeof properties): void => {
      clearCreateError();
      setProperties(newProperties);
    },
    [clearCreateError, setProperties],
  );

  const handleEnumInputChange = useCallback(
    (newEnumInput: typeof enumInput): void => {
      clearCreateError();
      setEnumInput(newEnumInput);
    },
    [clearCreateError, setEnumInput],
  );

  const handleDisplayAttributeChange = useCallback(
    (newDisplayAttribute: string): void => {
      clearCreateError();
      setDisplayAttribute(newDisplayAttribute);
    },
    [clearCreateError, setDisplayAttribute],
  );

  const handleStepReadyChange = useCallback((step: UserTypeCreateFlowStep, isReady: boolean): void => {
    setStepReady((prev) => ({
      ...prev,
      [step]: isReady,
    }));
  }, []);

  const handleNameStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(UserTypeCreateFlowStep.NAME, isReady);
    },
    [handleStepReadyChange],
  );

  const handlePropertiesStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(UserTypeCreateFlowStep.PROPERTIES, isReady);
    },
    [handleStepReadyChange],
  );

  const handleSubmit = async (): Promise<void> => {
    setError(null);

    // Validate
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError(t('userTypes:validationErrors.nameRequired', 'Please enter a user type name'));
      return;
    }

    if (trimmedName.length > UserTypeConstraints.NAME_MAX_LENGTH) {
      setError(
        t('userTypes:createWizard.name.maxLength', {
          max: UserTypeConstraints.NAME_MAX_LENGTH,
          defaultValue: `User type name cannot exceed ${UserTypeConstraints.NAME_MAX_LENGTH} characters`,
        }),
      );
      return;
    }

    const trimmedOuId = ouId.trim();
    if (!trimmedOuId) {
      setError(t('userTypes:validationErrors.ouIdRequired', 'Please provide an organization unit ID'));
      return;
    }

    const validProperties = properties.filter((prop) => prop.name.trim());
    if (validProperties.length === 0) {
      setError(t('userTypes:validationErrors.propertiesRequired', 'Please add at least one property'));
      return;
    }

    // Check for duplicate property names
    const propertyNames = validProperties.map((prop) => prop.name.trim());
    const duplicates = propertyNames.filter((propName, index) => propertyNames.indexOf(propName) !== index);
    if (duplicates.length > 0) {
      setError(
        t('userTypes:validationErrors.duplicateProperties', {
          duplicates: duplicates.join(', '),
          defaultValue: 'Duplicate property names found: {{duplicates}}',
        }),
      );
      return;
    }

    // Convert properties to schema definition
    const schema: UserTypeDefinition = {};
    validProperties.forEach((prop) => {
      const actualType = prop.type === 'enum' ? 'string' : prop.type;

      const propDef: Partial<PropertyDefinition> = {
        type: actualType,
        required: prop.required,
        ...(prop.displayName.trim() ? {displayName: prop.displayName.trim()} : {}),
      };

      if (actualType === 'string' || actualType === 'number') {
        if (prop.unique) {
          (propDef as {unique?: boolean}).unique = true;
        }
        if (prop.credential) {
          (propDef as {credential?: boolean}).credential = true;
        }
      }

      if (actualType === 'string') {
        if (prop.type === 'enum' || prop.enum.length > 0) {
          (propDef as {enum?: string[]}).enum = prop.enum;
        }
        if (prop.regex.trim()) {
          (propDef as {regex?: string}).regex = prop.regex;
        }
      }

      if (actualType === 'array') {
        (propDef as {items?: {type: string}}).items = {type: 'string'};
      } else if (actualType === 'object') {
        (propDef as {properties?: Record<string, PropertyDefinition>}).properties = {};
      }

      schema[prop.name.trim()] = propDef as PropertyDefinition;
    });

    const requestBody: CreateUserTypeRequest = {
      name: name.trim(),
      ouId: trimmedOuId,
      schema,
    };

    if (allowSelfRegistration) {
      requestBody.allowSelfRegistration = true;
    }

    if (displayAttribute) {
      requestBody.systemAttributes = {display: displayAttribute};
    }

    try {
      await createUserTypeMutation.mutateAsync(requestBody);
      await navigate(routes.list());
    } catch (submitError) {
      logger.error('Failed to create user type or navigate', {error: submitError, userTypeName: name});
    }
  };

  // Resolves an error through the `userTypes` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `userTypes:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = (key: string, options?: Record<string, unknown>): string =>
    t(key.includes(':') ? key : `userTypes:${key}`, options);

  // Precedence: a validation error (raised at submit time) wins over the mutation's own error, so
  // the user always sees the most actionable message for their current attempt.
  const displayError =
    error ??
    (createUserTypeMutation.error
      ? getErrorMessage(
          createUserTypeMutation.error,
          tForErrors,
          'create.error',
          'Failed to create user type. Please try again.',
        )
      : null);

  const handleNextStep = (): void => {
    switch (currentStep) {
      case UserTypeCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(UserTypeCreateFlowStep.NAME);
        break;
      case UserTypeCreateFlowStep.NAME:
        setCurrentStep(UserTypeCreateFlowStep.PROPERTIES);
        break;
      case UserTypeCreateFlowStep.PROPERTIES:
        handleSubmit().catch(() => {
          // Error handled in handleSubmit
        });
        break;
      default:
        break;
    }
  };

  const handlePrevStep = (): void => {
    switch (currentStep) {
      case UserTypeCreateFlowStep.NAME:
        if (hasMultipleOUs) setCurrentStep(UserTypeCreateFlowStep.ORGANIZATION_UNIT);
        break;
      case UserTypeCreateFlowStep.PROPERTIES:
        setCurrentStep(UserTypeCreateFlowStep.NAME);
        break;
      default:
        break;
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case UserTypeCreateFlowStep.NAME:
        return (
          <ConfigureName
            name={name}
            onNameChange={handleNameChange}
            onReadyChange={handleNameStepReadyChange}
            hasMultipleOUs={hasMultipleOUs}
            organizationUnitName={resolvedOrganizationUnit?.name}
            organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
            isOrganizationUnitLoading={isResolvedOuLoading}
            onChangeOu={() => setCurrentStep(UserTypeCreateFlowStep.ORGANIZATION_UNIT)}
            allowSelfRegistration={allowSelfRegistration}
            onAllowSelfRegistrationChange={handleAllowSelfRegistrationChange}
          />
        );
      case UserTypeCreateFlowStep.PROPERTIES:
        return (
          <ConfigureProperties
            properties={properties}
            onPropertiesChange={handlePropertiesChange}
            enumInput={enumInput}
            onEnumInputChange={handleEnumInputChange}
            displayAttribute={displayAttribute}
            onDisplayAttributeChange={handleDisplayAttributeChange}
            onReadyChange={handlePropertiesStepReadyChange}
            userTypeName={name.trim()}
          />
        );
      default:
        return null;
    }
  };

  const getStepProgress = (): number => {
    const currentIndex = activeSteps.indexOf(currentStep);
    return ((currentIndex + 1) / activeSteps.length) * 100;
  };

  const getBreadcrumbSteps = (): UserTypeCreateFlowStep[] => {
    const currentIndex = activeSteps.indexOf(currentStep);
    return activeSteps.slice(0, currentIndex + 1);
  };

  const isLastStep = currentStep === UserTypeCreateFlowStep.PROPERTIES;
  // The Properties step uses a two-panel builder that needs more horizontal room
  // than the single-column Name/General forms.
  const isPropertiesStep = currentStep === UserTypeCreateFlowStep.PROPERTIES;

  if (currentStep === UserTypeCreateFlowStep.ORGANIZATION_UNIT) {
    if (isOuLoading || !hasMultipleOUs) {
      return (
        <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <CircularProgress />
        </Box>
      );
    }

    return (
      <OrganizationUnitPickerScreen
        icon={<Home size={26} />}
        title={t('userTypes:createWizard.organizationUnit.title', 'Where should this user type belong?')}
        subtitle={t(
          'userTypes:createWizard.organizationUnit.subtitle',
          "Choose the organization unit that will own this user type. You can't change this once created.",
        )}
        value={ouId}
        onChange={handleOuIdChange}
        onBack={handleClose}
        onContinue={handleNextStep}
        backLabel={t('common:actions.back', 'Back')}
        continueLabel={t('common:actions.continue', 'Continue')}
      />
    );
  }

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      {/* Progress bar at the very top */}
      <LinearProgress variant="determinate" value={getStepProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        {/* Header with close button and breadcrumb */}
        <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Stack direction="row" alignItems="center" spacing={2}>
            <IconButton
              aria-label={t('common:actions.close')}
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
              items={getBreadcrumbSteps().map((step, index, array) => ({
                key: step,
                label: steps[step]?.label ?? step,
                onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
              }))}
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
              py: 4,
              px: {xs: 4, md: 10},
              alignItems: 'flex-start',
            }}
          >
            <Box
              sx={{
                width: '100%',
                maxWidth: isPropertiesStep ? 1200 : 800,
                display: 'flex',
                flexDirection: 'column',
              }}
            >
              {/* Error Alert — validation error takes precedence over the mutation's own error */}
              {displayError && (
                <Alert
                  severity="error"
                  sx={{my: 3}}
                  onClose={() => {
                    setError(null);
                    createUserTypeMutation.reset();
                  }}
                >
                  {displayError}
                </Alert>
              )}

              {renderStepContent()}

              {/* Navigation buttons */}
              <Stack
                direction="row"
                justifyContent={activeSteps.indexOf(currentStep) > 0 ? 'space-between' : 'flex-end'}
                alignItems="center"
                spacing={2}
                sx={{mt: 4}}
              >
                {activeSteps.indexOf(currentStep) > 0 && (
                  <Button
                    variant="outlined"
                    onClick={handlePrevStep}
                    sx={{minWidth: 100}}
                    disabled={createUserTypeMutation.isPending}
                  >
                    {t('common:actions.back')}
                  </Button>
                )}

                <Button
                  variant="contained"
                  disabled={!stepReady[currentStep] || createUserTypeMutation.isPending}
                  sx={{minWidth: 140}}
                  onClick={handleNextStep}
                >
                  {(() => {
                    if (!isLastStep) return t('common:actions.continue');
                    if (createUserTypeMutation.isPending) return t('common:status.saving');
                    return t('userTypes:createUserType');
                  })()}
                </Button>
              </Stack>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
