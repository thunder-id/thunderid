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
import useCreateGroup from '../api/useCreateGroup';
import ConfigureName from '../components/create-group/ConfigureName';
import ConfigureOrganizationUnit from '../components/create-group/ConfigureOrganizationUnit';
import useGroupCreate from '../contexts/GroupCreate/useGroupCreate';
import {GroupCreateFlowStep} from '../models/group-create-flow';
import type {CreateGroupRequest} from '../models/requests';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

export default function CreateGroupPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('CreateGroupPage');
  const createGroup = useCreateGroup();

  const {currentStep, setCurrentStep, name, setName, ouId, setOuId, error, setError} = useGroupCreate();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [validationError, setValidationError] = useState<string | null>(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);

  const [stepReady, setStepReady] = useState<Record<GroupCreateFlowStep, boolean>>({
    NAME: false,
    ORGANIZATION_UNIT: false,
  });

  const activeSteps = useMemo((): GroupCreateFlowStep[] => {
    const base: GroupCreateFlowStep[] = [GroupCreateFlowStep.NAME];
    if (hasMultipleOUs) {
      base.push(GroupCreateFlowStep.ORGANIZATION_UNIT);
    }
    return base;
  }, [hasMultipleOUs]);

  const steps: Partial<Record<GroupCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<GroupCreateFlowStep, {label: string}>> = {
      NAME: {label: t('groups:createWizard.steps.name')},
    };
    if (hasMultipleOUs) {
      map.ORGANIZATION_UNIT = {label: t('groups:createWizard.steps.organizationUnit')};
    }
    return map;
  }, [t, hasMultipleOUs]);

  const listUrl = '/groups';

  const handleClose = (): void => {
    if (createGroup.isPending) return;
    void navigate(listUrl);
  };

  const handleStepReadyChange = useCallback((step: GroupCreateFlowStep, isReady: boolean): void => {
    setStepReady((prev) => ({
      ...prev,
      [step]: isReady,
    }));
  }, []);

  const handleNameStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(GroupCreateFlowStep.NAME, isReady);
    },
    [handleStepReadyChange],
  );

  const handleOuStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(GroupCreateFlowStep.ORGANIZATION_UNIT, isReady);
    },
    [handleStepReadyChange],
  );

  const handleSubmit = async (): Promise<void> => {
    setValidationError(null);
    setError(null);

    if (!name.trim()) {
      setValidationError(t('groups:create.form.name.required'));
      setSnackbarOpen(true);
      return;
    }

    // If only one OU, use it directly
    const selectedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
    if (!selectedOuId) {
      setValidationError(t('groups:create.form.organizationUnit.required'));
      setSnackbarOpen(true);
      return;
    }

    const requestData: CreateGroupRequest = {
      name: name.trim(),
      ouId: selectedOuId,
    };

    try {
      await createGroup.mutateAsync(requestData);
      await navigate(listUrl);
    } catch (submitError) {
      logger.error('Failed to create group or navigate', {error: submitError});
    }
  };

  const handleNextStep = (): void => {
    switch (currentStep) {
      case GroupCreateFlowStep.NAME:
        if (isOuLoading) return;
        if (hasMultipleOUs) {
          setCurrentStep(GroupCreateFlowStep.ORGANIZATION_UNIT);
        } else {
          // Only one OU — skip the OU step and submit directly
          handleSubmit().catch(() => {
            // Error handled in handleSubmit
          });
        }
        break;
      case GroupCreateFlowStep.ORGANIZATION_UNIT:
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
      case GroupCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(GroupCreateFlowStep.NAME);
        break;
      default:
        break;
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case GroupCreateFlowStep.NAME:
        return <ConfigureName name={name} onNameChange={setName} onReadyChange={handleNameStepReadyChange} />;
      case GroupCreateFlowStep.ORGANIZATION_UNIT:
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

  const getBreadcrumbSteps = (): GroupCreateFlowStep[] => {
    const currentIndex = activeSteps.indexOf(currentStep);
    return activeSteps.slice(0, currentIndex + 1);
  };

  const handleCloseSnackbar = () => {
    setSnackbarOpen(false);
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={getStepProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'row'}}>
        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {/* Header with close button and breadcrumb */}
          <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Stack direction="row" alignItems="center" spacing={2}>
              <IconButton
                onClick={handleClose}
                aria-label={t('common:actions.close')}
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
                py: 8,
                px: 20,
                mx: currentStep === GroupCreateFlowStep.NAME ? 'auto' : 0,
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

                {createGroup.error && (
                  <Alert severity="error" sx={{mb: 3}}>
                    <Typography variant="body2" sx={{fontWeight: 'bold', mb: 0.5}}>
                      {createGroup.error.message}
                    </Typography>
                  </Alert>
                )}

                {renderStepContent()}

                {/* Navigation buttons */}
                <Box
                  sx={{
                    mt: 4,
                    display: 'flex',
                    justifyContent: 'flex-end',
                    gap: 2,
                  }}
                >
                  {currentStep !== GroupCreateFlowStep.NAME && (
                    <Button
                      variant="outlined"
                      onClick={handlePrevStep}
                      sx={{minWidth: 100}}
                      disabled={createGroup.isPending}
                    >
                      {t('common:actions.back')}
                    </Button>
                  )}

                  <Button
                    variant="contained"
                    disabled={!stepReady[currentStep] || createGroup.isPending || isOuLoading}
                    sx={{minWidth: 100}}
                    onClick={handleNextStep}
                  >
                    {(() => {
                      if (createGroup.isPending) return t('common:status.saving');
                      return t('common:actions.continue');
                    })()}
                  </Button>
                </Box>
              </Box>
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
