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

import {useLogger} from '@thunderid/logger/react';
import {Box, Stack, Button, IconButton, LinearProgress, Typography, Alert, Snackbar} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useMemo} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateUserType from '../api/useCreateUserType';
import ConfigureGeneral from '../components/create-user-type/ConfigureGeneral';
import ConfigureName from '../components/create-user-type/ConfigureName';
import ConfigureProperties from '../components/create-user-type/ConfigureProperties';
import useUserTypeCreate from '../contexts/UserTypeCreate/useUserTypeCreate';
import {UserTypeCreateFlowStep} from '../models/user-type-create-flow';
import type {PropertyDefinition, UserTypeDefinition, CreateUserTypeRequest} from '../types/user-types';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

export default function CreateUserTypePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('CreateUserTypePage');
  const createUserTypeMutation = useCreateUserType();

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

  const steps: Record<UserTypeCreateFlowStep, {label: string; order: number}> = useMemo(
    () => ({
      NAME: {label: t('userTypes:createWizard.steps.name'), order: 1},
      GENERAL: {label: t('userTypes:createWizard.steps.general'), order: 2},
      PROPERTIES: {label: t('userTypes:createWizard.steps.properties'), order: 3},
    }),
    [t],
  );

  const [validationError, setValidationError] = useState<string | null>(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);

  const [stepReady, setStepReady] = useState<Record<UserTypeCreateFlowStep, boolean>>({
    NAME: false,
    GENERAL: false,
    PROPERTIES: false,
  });

  const handleClose = (): void => {
    void navigate('/user-types');
  };

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

  const handleGeneralStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(UserTypeCreateFlowStep.GENERAL, isReady);
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
    setValidationError(null);
    setError(null);

    // Validate
    if (!name.trim()) {
      setValidationError(t('userTypes:validationErrors.nameRequired'));
      setSnackbarOpen(true);
      return;
    }

    const trimmedOuId = ouId.trim();
    if (!trimmedOuId) {
      setValidationError(t('userTypes:validationErrors.ouIdRequired'));
      setSnackbarOpen(true);
      return;
    }

    const validProperties = properties.filter((prop) => prop.name.trim());
    if (validProperties.length === 0) {
      setValidationError(t('userTypes:validationErrors.propertiesRequired'));
      setSnackbarOpen(true);
      return;
    }

    // Check for duplicate property names
    const propertyNames = validProperties.map((prop) => prop.name.trim());
    const duplicates = propertyNames.filter((propName, index) => propertyNames.indexOf(propName) !== index);
    if (duplicates.length > 0) {
      setValidationError(t('userTypes:validationErrors.duplicateProperties', {duplicates: duplicates.join(', ')}));
      setSnackbarOpen(true);
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
      await navigate('/user-types');
    } catch (submitError) {
      logger.error('Failed to create user type or navigate', {error: submitError, userTypeName: name});
    }
  };

  const handleNextStep = (): void => {
    switch (currentStep) {
      case UserTypeCreateFlowStep.NAME:
        setCurrentStep(UserTypeCreateFlowStep.GENERAL);
        break;
      case UserTypeCreateFlowStep.GENERAL:
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
      case UserTypeCreateFlowStep.GENERAL:
        setCurrentStep(UserTypeCreateFlowStep.NAME);
        break;
      case UserTypeCreateFlowStep.PROPERTIES:
        setCurrentStep(UserTypeCreateFlowStep.GENERAL);
        break;
      default:
        break;
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case UserTypeCreateFlowStep.NAME:
        return <ConfigureName name={name} onNameChange={setName} onReadyChange={handleNameStepReadyChange} />;
      case UserTypeCreateFlowStep.GENERAL:
        return (
          <ConfigureGeneral
            ouId={ouId}
            onOuIdChange={setOuId}
            allowSelfRegistration={allowSelfRegistration}
            onAllowSelfRegistrationChange={setAllowSelfRegistration}
            onReadyChange={handleGeneralStepReadyChange}
          />
        );
      case UserTypeCreateFlowStep.PROPERTIES:
        return (
          <ConfigureProperties
            properties={properties}
            onPropertiesChange={setProperties}
            enumInput={enumInput}
            onEnumInputChange={setEnumInput}
            displayAttribute={displayAttribute}
            onDisplayAttributeChange={setDisplayAttribute}
            onReadyChange={handlePropertiesStepReadyChange}
            userTypeName={name.trim()}
          />
        );
      default:
        return null;
    }
  };

  const getStepProgress = (): number => {
    const stepNames = Object.keys(steps) as UserTypeCreateFlowStep[];
    return ((stepNames.indexOf(currentStep) + 1) / stepNames.length) * 100;
  };

  const getBreadcrumbSteps = (): UserTypeCreateFlowStep[] => {
    const allSteps: UserTypeCreateFlowStep[] = [
      UserTypeCreateFlowStep.NAME,
      UserTypeCreateFlowStep.GENERAL,
      UserTypeCreateFlowStep.PROPERTIES,
    ];

    const currentIndex = allSteps.indexOf(currentStep);
    return allSteps.slice(0, currentIndex + 1);
  };

  const handleCloseSnackbar = () => {
    setSnackbarOpen(false);
  };

  const isLastStep = currentStep === UserTypeCreateFlowStep.PROPERTIES;

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
                label: steps[step].label,
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
              py: 8,
              px: 20,
              mx: currentStep === UserTypeCreateFlowStep.NAME ? 'auto' : 0,
              alignItems: 'flex-start',
            }}
          >
            <Box
              sx={{
                width: '100%',
                maxWidth: 800,
                display: 'flex',
                flexDirection: 'column',
              }}
            >
              {/* Error Alerts */}
              {error && (
                <Alert severity="error" sx={{my: 3}} onClose={() => setError(null)}>
                  {error}
                </Alert>
              )}

              {createUserTypeMutation.error && (
                <Alert severity="error" sx={{mb: 3}}>
                  <Typography variant="body2" sx={{fontWeight: 'bold', mb: 0.5}}>
                    {createUserTypeMutation.error.message}
                  </Typography>
                </Alert>
              )}

              {renderStepContent()}

              {/* Navigation buttons */}
              <Stack direction="row" justifyContent="flex-end" alignItems="center" spacing={2} sx={{mt: 4}}>
                {currentStep !== UserTypeCreateFlowStep.NAME && (
                  <Button variant="text" onClick={handlePrevStep} disabled={createUserTypeMutation.isPending}>
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

      {/* Validation Error Snackbar */}
      <Snackbar
        open={snackbarOpen}
        autoHideDuration={6000}
        onClose={handleCloseSnackbar}
        anchorOrigin={{vertical: 'top', horizontal: 'right'}}
      >
        <Alert onClose={handleCloseSnackbar} severity="error" sx={{width: '100%'}}>
          {validationError}
        </Alert>
      </Snackbar>
    </Box>
  );
}
