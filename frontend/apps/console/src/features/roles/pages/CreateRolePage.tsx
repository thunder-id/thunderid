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

import {useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {useLogger} from '@thunderid/logger/react';
import {Box, Stack, Typography, Button, IconButton, LinearProgress, Alert, Snackbar} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useMemo} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateRole from '../api/useCreateRole';
import ConfigureBasicInfo from '../components/create-role/ConfigureBasicInfo';
import ConfigureOrganizationUnit from '../components/create-role/ConfigureOrganizationUnit';
import useRoleCreate from '../contexts/RoleCreate/useRoleCreate';
import type {CreateRoleRequest} from '../models/requests';
import {RoleCreateFlowStep} from '../models/role-create-flow';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

export default function CreateRolePage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('CreateRolePage');
  const createRole = useCreateRole();

  const {currentStep, setCurrentStep, name, setName, ouId, setOuId, error, setError} = useRoleCreate();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [validationError, setValidationError] = useState<string | null>(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);

  const [stepReady, setStepReady] = useState<Record<RoleCreateFlowStep, boolean>>({
    BASIC_INFO: false,
    ORGANIZATION_UNIT: false,
  });

  const activeSteps = useMemo((): RoleCreateFlowStep[] => {
    const base: RoleCreateFlowStep[] = [RoleCreateFlowStep.BASIC_INFO];
    if (hasMultipleOUs) {
      base.push(RoleCreateFlowStep.ORGANIZATION_UNIT);
    }
    return base;
  }, [hasMultipleOUs]);

  const steps: Partial<Record<RoleCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<RoleCreateFlowStep, {label: string}>> = {
      BASIC_INFO: {label: t('roles:createWizard.steps.basicInfo')},
    };
    if (hasMultipleOUs) {
      map.ORGANIZATION_UNIT = {label: t('roles:createWizard.steps.organizationUnit')};
    }
    return map;
  }, [t, hasMultipleOUs]);

  const listUrl = '/roles';

  const handleClose = (): void => {
    if (createRole.isPending) return;
    void navigate(listUrl);
  };

  const handleStepReadyChange = useCallback((step: RoleCreateFlowStep, isReady: boolean): void => {
    setStepReady((prev) => ({...prev, [step]: isReady}));
  }, []);

  const handleBasicInfoStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(RoleCreateFlowStep.BASIC_INFO, isReady);
    },
    [handleStepReadyChange],
  );

  const handleOuStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(RoleCreateFlowStep.ORGANIZATION_UNIT, isReady);
    },
    [handleStepReadyChange],
  );

  const handleSubmit = async (): Promise<void> => {
    setValidationError(null);
    setError(null);

    if (!name.trim()) {
      setValidationError(t('roles:create.form.name.required'));
      setSnackbarOpen(true);
      return;
    }

    const selectedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
    if (!selectedOuId) {
      setValidationError(t('roles:create.form.organizationUnit.required'));
      setSnackbarOpen(true);
      return;
    }

    const requestData: CreateRoleRequest = {
      name: name.trim(),
      ouId: selectedOuId,
    };

    try {
      await createRole.mutateAsync(requestData);
      await navigate(listUrl);
    } catch (submitError) {
      logger.error('Failed to create role or navigate', {error: submitError});
    }
  };

  const handleNextStep = (): void => {
    switch (currentStep) {
      case RoleCreateFlowStep.BASIC_INFO:
        if (isOuLoading) return;
        if (hasMultipleOUs) {
          setCurrentStep(RoleCreateFlowStep.ORGANIZATION_UNIT);
        } else {
          handleSubmit().catch(() => {
            /* noop */
          });
        }
        break;
      case RoleCreateFlowStep.ORGANIZATION_UNIT:
        handleSubmit().catch(() => {
          /* noop */
        });
        break;
      default:
        break;
    }
  };

  const handlePrevStep = (): void => {
    if (currentStep === RoleCreateFlowStep.ORGANIZATION_UNIT) {
      setCurrentStep(RoleCreateFlowStep.BASIC_INFO);
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case RoleCreateFlowStep.BASIC_INFO:
        return <ConfigureBasicInfo name={name} onNameChange={setName} onReadyChange={handleBasicInfoStepReadyChange} />;
      case RoleCreateFlowStep.ORGANIZATION_UNIT:
        return (
          <ConfigureOrganizationUnit
            selectedOuId={ouId}
            onOuIdChange={setOuId}
            onReadyChange={handleOuStepReadyChange}
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

  const getBreadcrumbSteps = (): RoleCreateFlowStep[] => {
    const currentIndex = activeSteps.indexOf(currentStep);
    return activeSteps.slice(0, currentIndex + 1);
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={getStepProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'row'}}>
        <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
          {/* Header */}
          <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Stack direction="row" alignItems="center" spacing={2}>
              <IconButton
                onClick={handleClose}
                aria-label={t('common:actions.close')}
                sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
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
                py: 8,
                px: 20,
                mx: currentStep === RoleCreateFlowStep.BASIC_INFO ? 'auto' : 0,
                alignItems: 'flex-start',
              }}
            >
              <Box sx={{width: '100%', maxWidth: 800, display: 'flex', flexDirection: 'column'}}>
                {error && (
                  <Alert severity="error" sx={{my: 3}} onClose={() => setError(null)}>
                    {error}
                  </Alert>
                )}

                {createRole.error && (
                  <Alert severity="error" sx={{mb: 3}}>
                    <Typography variant="body2" sx={{fontWeight: 'bold', mb: 0.5}}>
                      {createRole.error.message}
                    </Typography>
                  </Alert>
                )}

                {renderStepContent()}

                {/* Navigation buttons */}
                <Box sx={{mt: 4, display: 'flex', justifyContent: 'flex-end', gap: 2}}>
                  {currentStep !== RoleCreateFlowStep.BASIC_INFO && (
                    <Button
                      variant="outlined"
                      onClick={handlePrevStep}
                      sx={{minWidth: 100}}
                      disabled={createRole.isPending}
                    >
                      {t('common:actions.back')}
                    </Button>
                  )}

                  <Button
                    variant="contained"
                    disabled={!stepReady[currentStep] || createRole.isPending || isOuLoading}
                    sx={{minWidth: 100}}
                    onClick={handleNextStep}
                  >
                    {createRole.isPending ? t('common:status.saving') : t('common:actions.continue')}
                  </Button>
                </Box>
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={6000}
        onClose={() => setSnackbarOpen(false)}
        anchorOrigin={{vertical: 'top', horizontal: 'right'}}
      >
        <Alert onClose={() => setSnackbarOpen(false)} severity="error" sx={{width: '100%'}}>
          {validationError}
        </Alert>
      </Snackbar>
    </Box>
  );
}
