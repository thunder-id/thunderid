// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {
  OrganizationUnitPickerScreen,
  useGetOrganizationUnit,
  useHasMultipleOUs,
} from '@thunderid/configure-organization-units';
import {UserTypeConstraints} from '@thunderid/configure-user-types';
import {useResolveDisplayName} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {getContactTypesMissingRegex, getErrorMessage} from '@thunderid/utils';
import {Box, Stack, Button, CircularProgress, Alert} from '@wso2/oxygen-ui';
import {Home} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useEffect, useMemo} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateAgentType from '../api/useCreateAgentType';
import useGetAllAgentTypes from '../api/useGetAllAgentTypes';
import ConfigureName from '../components/create-agent-type/ConfigureName';
import ConfigureProperties from '../components/create-agent-type/ConfigureProperties';
import useAgentTypeCreate from '../contexts/AgentTypeCreate/useAgentTypeCreate';
import useAgentTypeRoutes from '../hooks/useAgentTypeRoutes';
import {AgentTypeCreateFlowStep} from '../models/agent-type-create-flow';
import type {SchemaPropertyInput} from '../models/property-definition';
import type {CreateAgentTypeRequest} from '../models/requests';
import convertPropertiesToSchema from '../utils/convertPropertiesToSchema';

/**
 * Full-screen wizard that creates an agent type: its organization unit (when there is more than
 * one), its name and handle, then its schema properties and display attribute.
 *
 * @public
 */
export default function CreateAgentTypePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('CreateAgentTypePage');
  const createAgentTypeMutation = useCreateAgentType();
  const routes = useAgentTypeRoutes();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const {
    currentStep,
    setCurrentStep,
    name,
    setName,
    handle,
    setHandle,
    handleEdited,
    setHandleEdited,
    ouId,
    setOuId,
    properties,
    setProperties,
    displayAttribute,
    setDisplayAttribute,
    error,
    setError,
  } = useAgentTypeCreate();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, resolve it automatically and skip straight
  // past it.
  useEffect(() => {
    if (isOuLoading || hasMultipleOUs || currentStep !== AgentTypeCreateFlowStep.ORGANIZATION_UNIT) return;
    setOuId(ouList[0]?.id ?? '');
    setCurrentStep(AgentTypeCreateFlowStep.NAME);
  }, [isOuLoading, hasMultipleOUs, ouList, currentStep, setOuId, setCurrentStep]);

  // The organization unit whose name is shown in the Details step's summary chip.
  const resolvedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    resolvedOuId,
    Boolean(resolvedOuId),
  );

  // An organization unit can have one agent type, so an organization unit that already has one is
  // refused on the first step the wizard shows rather than at the final submit. The server still
  // enforces the rule; this list only reports it sooner. With a single organization unit there is no
  // other one to choose, so the message says how to get one instead.
  const {data: agentTypes} = useGetAllAgentTypes();
  const takenByAgentType = resolvedOuId
    ? (agentTypes ?? []).find((agentType) => agentType.ouId === resolvedOuId)
    : undefined;
  const isOuTaken = Boolean(takenByAgentType);
  const takenHandles = useMemo(() => agentTypes?.map((agentType) => agentType.handle), [agentTypes]);
  let ouTakenError: string | null = null;
  // A read-only agent type can't be edited, so its message offers only the other way out.
  if (takenByAgentType) {
    const takenName = resolveDisplayName(takenByAgentType.displayName) || takenByAgentType.handle;
    if (hasMultipleOUs) {
      ouTakenError = takenByAgentType.isReadOnly
        ? t(
            'agentTypes:createWizard.organizationUnitTakenReadOnly',
            'This organization unit already has the agent type {{name}}, which is read-only. Choose another organization unit.',
            {name: takenName},
          )
        : t(
            'agentTypes:createWizard.organizationUnitTaken',
            'This organization unit already has the agent type {{name}}. Choose another organization unit, or edit {{name}} instead.',
            {name: takenName},
          );
    } else {
      ouTakenError = takenByAgentType.isReadOnly
        ? t(
            'agentTypes:createWizard.onlyOrganizationUnitTakenReadOnly',
            'The only organization unit already has the agent type {{name}}, which is read-only. Create another organization unit first.',
            {name: takenName},
          )
        : t(
            'agentTypes:createWizard.onlyOrganizationUnitTaken',
            'The only organization unit already has the agent type {{name}}. Create another organization unit first, or edit {{name}} instead.',
            {name: takenName},
          );
    }
  }
  // Opens the agent type that holds the organization unit, where the admin can edit it or, when it is
  // read-only, see what it holds.
  const ouTakenAction = takenByAgentType ? (
    <Button color="inherit" size="small" onClick={() => void navigate(routes.detail(takenByAgentType.id))}>
      {t('agentTypes:createWizard.openTakenAgentType', 'Open agent type')}
    </Button>
  ) : undefined;

  const activeSteps = useMemo((): AgentTypeCreateFlowStep[] => {
    const base: AgentTypeCreateFlowStep[] = [];
    if (hasMultipleOUs) base.push(AgentTypeCreateFlowStep.ORGANIZATION_UNIT);
    base.push(AgentTypeCreateFlowStep.NAME, AgentTypeCreateFlowStep.PROPERTIES);
    return base;
  }, [hasMultipleOUs]);

  const steps: Partial<Record<AgentTypeCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<AgentTypeCreateFlowStep, {label: string}>> = {};
    if (hasMultipleOUs) {
      map.ORGANIZATION_UNIT = {label: t('agentTypes:createWizard.steps.organizationUnit', 'Organization unit')};
    }
    map.NAME = {label: t('agentTypes:createWizard.steps.name', 'Details')};
    map.PROPERTIES = {label: t('agentTypes:createWizard.steps.properties', 'Properties')};
    return map;
  }, [t, hasMultipleOUs]);

  const [stepReady, setStepReady] = useState<Record<AgentTypeCreateFlowStep, boolean>>({
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
    if (createAgentTypeMutation.isError) {
      createAgentTypeMutation.reset();
    }
  }, [setError, createAgentTypeMutation]);

  const handleNameChange = useCallback(
    (newName: string): void => {
      clearCreateError();
      setName(newName);
    },
    [clearCreateError, setName],
  );

  const handleHandleChange = useCallback(
    (newHandle: string): void => {
      clearCreateError();
      setHandle(newHandle);
    },
    [clearCreateError, setHandle],
  );

  const handleOuIdChange = useCallback(
    (newOuId: string): void => {
      clearCreateError();
      setOuId(newOuId);
    },
    [clearCreateError, setOuId],
  );

  const handlePropertiesChange = useCallback(
    (newProperties: SchemaPropertyInput[]): void => {
      clearCreateError();
      setProperties(newProperties);
    },
    [clearCreateError, setProperties],
  );

  const handleDisplayAttributeChange = useCallback(
    (newDisplayAttribute: string): void => {
      clearCreateError();
      setDisplayAttribute(newDisplayAttribute);
    },
    [clearCreateError, setDisplayAttribute],
  );

  const handleNameStepReadyChange = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, [AgentTypeCreateFlowStep.NAME]: isReady}));
  }, []);

  const handlePropertiesStepReadyChange = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, [AgentTypeCreateFlowStep.PROPERTIES]: isReady}));
  }, []);

  const handleSubmit = async (): Promise<void> => {
    setError(null);

    const trimmedName = name.trim();
    if (!trimmedName) {
      setError(t('agentTypes:validationErrors.nameRequired', 'Please enter an agent type name'));
      return;
    }

    if (trimmedName.length > UserTypeConstraints.NAME_MAX_LENGTH) {
      setError(
        t('agentTypes:createWizard.name.maxLength', {
          max: UserTypeConstraints.NAME_MAX_LENGTH,
          defaultValue: 'Agent type name cannot exceed {{max}} characters',
        }),
      );
      return;
    }

    const trimmedHandle = handle.trim();
    if (!trimmedHandle) {
      setError(t('agentTypes:validationErrors.handleRequired', 'Please enter an agent type handle'));
      return;
    }

    if (
      trimmedHandle.length > UserTypeConstraints.HANDLE_MAX_LENGTH ||
      !UserTypeConstraints.HANDLE_PATTERN.test(trimmedHandle)
    ) {
      setError(
        t(
          'agentTypes:validationErrors.handleInvalid',
          'Handle must start and end with a lowercase letter or number, and may contain only lowercase letters, numbers, hyphens, and underscores',
        ),
      );
      return;
    }

    const trimmedOuId = ouId.trim();
    if (!trimmedOuId) {
      setError(t('agentTypes:validationErrors.ouIdRequired', 'Please provide an organization unit ID'));
      return;
    }

    const validProperties = properties.filter((prop) => prop.name.trim());
    if (validProperties.length === 0) {
      setError(t('agentTypes:validationErrors.propertiesRequired', 'Please add at least one property'));
      return;
    }

    const propertyNames = validProperties.map((prop) => prop.name.trim());
    const duplicates = propertyNames.filter((propName, index) => propertyNames.indexOf(propName) !== index);
    if (duplicates.length > 0) {
      setError(
        t('agentTypes:validationErrors.duplicateProperties', {
          duplicates: [...new Set(duplicates)].join(', '),
          defaultValue: 'Duplicate property names found: {{duplicates}}',
        }),
      );
      return;
    }

    const missingRegexTypes = getContactTypesMissingRegex(validProperties);
    if (missingRegexTypes.length > 0) {
      setError(
        missingRegexTypes
          .map((type) =>
            type === 'email'
              ? t('agentTypes:validationErrors.emailRegexRequired', 'Enter a regular expression for email attributes.')
              : t(
                  'agentTypes:validationErrors.mobileRegexRequired',
                  'Enter a regular expression for mobile attributes.',
                ),
          )
          .join(' '),
      );
      return;
    }

    const requestBody: CreateAgentTypeRequest = {
      handle: trimmedHandle,
      displayName: trimmedName,
      ouId: trimmedOuId,
      ...(displayAttribute ? {systemAttributes: {display: displayAttribute}} : {}),
      schema: convertPropertiesToSchema(validProperties),
    };

    try {
      await createAgentTypeMutation.mutateAsync(requestBody);
      await navigate(routes.list());
    } catch (submitError) {
      logger.error('Failed to create agent type or navigate', {error: submitError, agentTypeHandle: trimmedHandle});
    }
  };

  // Resolves an error through the `agentTypes` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agentTypes:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = (key: string, options?: Record<string, unknown>): string =>
    t(key.includes(':') ? key : `agentTypes:${key}`, options);

  // Precedence: a validation error (raised at submit time) wins over the mutation's own error, so
  // the user always sees the most actionable message for their current attempt.
  const displayError =
    ouTakenError ??
    error ??
    (createAgentTypeMutation.error
      ? getErrorMessage(
          createAgentTypeMutation.error,
          tForErrors,
          'create.error',
          'Failed to create agent type. Please try again.',
        )
      : null);

  const handleNextStep = (): void => {
    if (isOuTaken) return;

    switch (currentStep) {
      case AgentTypeCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(AgentTypeCreateFlowStep.NAME);
        break;
      case AgentTypeCreateFlowStep.NAME:
        setCurrentStep(AgentTypeCreateFlowStep.PROPERTIES);
        break;
      case AgentTypeCreateFlowStep.PROPERTIES:
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
      case AgentTypeCreateFlowStep.NAME:
        if (hasMultipleOUs) setCurrentStep(AgentTypeCreateFlowStep.ORGANIZATION_UNIT);
        break;
      case AgentTypeCreateFlowStep.PROPERTIES:
        setCurrentStep(AgentTypeCreateFlowStep.NAME);
        break;
      default:
        break;
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case AgentTypeCreateFlowStep.NAME:
        return (
          <ConfigureName
            name={name}
            handle={handle}
            handleEdited={handleEdited}
            onNameChange={handleNameChange}
            onHandleChange={handleHandleChange}
            onHandleEditedChange={setHandleEdited}
            onReadyChange={handleNameStepReadyChange}
            hasMultipleOUs={hasMultipleOUs}
            organizationUnitName={resolvedOrganizationUnit?.name}
            organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
            isOrganizationUnitLoading={isResolvedOuLoading}
            onChangeOu={() => setCurrentStep(AgentTypeCreateFlowStep.ORGANIZATION_UNIT)}
            takenHandles={takenHandles}
          />
        );
      case AgentTypeCreateFlowStep.PROPERTIES:
        return (
          <ConfigureProperties
            properties={properties}
            onPropertiesChange={handlePropertiesChange}
            displayAttribute={displayAttribute}
            onDisplayAttributeChange={handleDisplayAttributeChange}
            onReadyChange={handlePropertiesStepReadyChange}
            agentTypeHandle={handle.trim()}
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

  const getBreadcrumbSteps = (): AgentTypeCreateFlowStep[] => {
    const currentIndex = activeSteps.indexOf(currentStep);
    return activeSteps.slice(0, currentIndex + 1);
  };

  const isLastStep = currentStep === AgentTypeCreateFlowStep.PROPERTIES;

  const pageTitle = t('agentTypes:createAgentType', 'Create agent type');

  if (currentStep === AgentTypeCreateFlowStep.ORGANIZATION_UNIT) {
    if (isOuLoading || !hasMultipleOUs) {
      return (
        <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <DocumentTitle title={pageTitle} />
          <CircularProgress aria-label={t('common:status.loading', 'Loading...')} />
        </Box>
      );
    }

    return (
      <>
        <DocumentTitle title={pageTitle} />
        <OrganizationUnitPickerScreen
          icon={<Home size={26} />}
          title={t('agentTypes:createWizard.organizationUnit.title', 'Where should this agent type belong?')}
          subtitle={t(
            'agentTypes:createWizard.organizationUnit.subtitle',
            "Choose the organization unit that will own this agent type. You can't change this once created.",
          )}
          value={ouId}
          onChange={handleOuIdChange}
          onBack={handleClose}
          onContinue={handleNextStep}
          backLabel={t('common:actions.back', 'Back')}
          continueLabel={t('common:actions.continue', 'Continue')}
          error={ouTakenError ?? undefined}
          errorAction={ouTakenAction}
          autoSelectFirst={false}
        />
      </>
    );
  }

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={getStepProgress()}
      breadcrumbItems={getBreadcrumbSteps().map((step, index, array) => ({
        key: step,
        label: steps[step]?.label ?? step,
        onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
      }))}
      footer={
        <Stack
          direction="row"
          justifyContent={activeSteps.indexOf(currentStep) > 0 ? 'space-between' : 'flex-end'}
          alignItems="center"
          spacing={2}
        >
          {activeSteps.indexOf(currentStep) > 0 && (
            <Button
              variant="outlined"
              onClick={handlePrevStep}
              sx={{minWidth: 100}}
              disabled={createAgentTypeMutation.isPending}
            >
              {t('common:actions.back', 'Back')}
            </Button>
          )}

          <Button
            variant="contained"
            disabled={!stepReady[currentStep] || isOuTaken || createAgentTypeMutation.isPending}
            sx={{minWidth: 140}}
            onClick={handleNextStep}
          >
            {(() => {
              if (!isLastStep) return t('common:actions.continue', 'Continue');
              if (createAgentTypeMutation.isPending) return t('common:status.saving', 'Saving...');
              return t('agentTypes:createAgentType', 'Create agent type');
            })()}
          </Button>
        </Stack>
      }
    >
      <DocumentTitle title={pageTitle} />
      {/* A validation error takes precedence over the mutation's own error */}
      {displayError && (
        <Alert
          severity="error"
          sx={{mb: 3}}
          action={ouTakenAction}
          onClose={
            isOuTaken
              ? undefined
              : () => {
                  setError(null);
                  createAgentTypeMutation.reset();
                }
          }
        >
          {displayError}
        </Alert>
      )}

      {renderStepContent()}
    </FullScreenCreationWizardLayout>
  );
}
