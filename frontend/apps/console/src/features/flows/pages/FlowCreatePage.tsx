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
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  CircularProgress,
  IconButton,
  LinearProgress,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronRight, X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateFlow from '../api/useCreateFlow';
import ConfigureFlowName from '../components/create-flow/ConfigureFlowName';
import type {ConfigureFlowNameValue} from '../components/create-flow/ConfigureFlowName';
import SelectFlowTemplate from '../components/create-flow/SelectFlowTemplate';
import SelectFlowType from '../components/create-flow/SelectFlowType';
import type {FlowType} from '../models/flows';
import type {FlowTemplate} from '../models/templates';

const FlowCreateStep = {
  TYPE: 'TYPE',
  TEMPLATE: 'TEMPLATE',
  CONFIGURE: 'CONFIGURE',
} as const;

type FlowCreateStep = (typeof FlowCreateStep)[keyof typeof FlowCreateStep];

const ALL_STEPS = [FlowCreateStep.TYPE, FlowCreateStep.TEMPLATE, FlowCreateStep.CONFIGURE];

export default function FlowCreatePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('FlowCreatePage');
  const createFlow = useCreateFlow();

  const [currentStep, setCurrentStep] = useState<FlowCreateStep>(FlowCreateStep.TYPE);
  const [selectedType, setSelectedType] = useState<FlowType | null>(null);
  const [selectedTemplate, setSelectedTemplate] = useState<FlowTemplate | null>(null);
  const [typeReady, setTypeReady] = useState(false);
  const [nameValue, setNameValue] = useState<ConfigureFlowNameValue>({name: '', handle: ''});
  const [nameReady, setNameReady] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const steps: Record<FlowCreateStep, {label: string; order: number}> = useMemo(
    () => ({
      [FlowCreateStep.TYPE]: {label: t('flows:create.steps.type', 'Flow Type'), order: 1},
      [FlowCreateStep.TEMPLATE]: {label: t('flows:create.steps.template', 'Template'), order: 2},
      [FlowCreateStep.CONFIGURE]: {label: t('flows:create.steps.configure', 'Configure'), order: 3},
    }),
    [t],
  );

  const handleClose = (): void => {
    (async () => {
      await navigate('/flows');
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate to flows page', {error: _error});
    });
  };

  const handleNextStep = (): void => {
    if (currentStep === FlowCreateStep.TYPE) {
      setCurrentStep(FlowCreateStep.TEMPLATE);
      return;
    }
    if (currentStep === FlowCreateStep.TEMPLATE) {
      setCurrentStep(FlowCreateStep.CONFIGURE);
      return;
    }
    if (currentStep === FlowCreateStep.CONFIGURE) {
      if (!selectedType || !selectedTemplate) return;
      const flowRequest = {
        name: nameValue.name,
        handle: nameValue.handle,
        flowType: selectedType,
        nodes: selectedTemplate.config.nodes,
      };
      setError(null);
      createFlow.mutate(flowRequest, {
        onSuccess: (savedFlow) => {
          (async () => {
            const flowTypeRoute =
              selectedType === 'AUTHENTICATION' ? 'signin' : (selectedType?.toLowerCase() ?? 'signin');
            await navigate(`/flows/${flowTypeRoute}/${savedFlow.id}`);
          })().catch((_error: unknown) => {
            logger.error('Failed to navigate to flow builder', {error: _error, flowId: savedFlow.id});
          });
        },
        onError: (err) => {
          setError(err.message ?? t('flows:create.error.createFailed', 'Failed to create flow. Please try again.'));
        },
      });
    }
  };

  const handlePrevStep = (): void => {
    if (currentStep === FlowCreateStep.TEMPLATE) setCurrentStep(FlowCreateStep.TYPE);
    if (currentStep === FlowCreateStep.CONFIGURE) setCurrentStep(FlowCreateStep.TEMPLATE);
  };

  const handleTypeChange = (type: FlowType): void => {
    setSelectedType(type);
    setSelectedTemplate(null);
  };

  const handleTemplateChange = (template: FlowTemplate): void => {
    setSelectedTemplate(template);
  };

  const getStepProgress = (): number => {
    return ((ALL_STEPS.indexOf(currentStep) + 1) / ALL_STEPS.length) * 100;
  };

  const getBreadcrumbSteps = (): FlowCreateStep[] => {
    const currentIndex = ALL_STEPS.indexOf(currentStep);
    return ALL_STEPS.slice(0, currentIndex + 1);
  };

  const isContinueDisabled = (): boolean => {
    if (currentStep === FlowCreateStep.TYPE) return !typeReady;
    if (currentStep === FlowCreateStep.TEMPLATE) return !selectedTemplate;
    if (currentStep === FlowCreateStep.CONFIGURE) return !nameReady || createFlow.isPending;
    return false;
  };

  const isNarrowStep = currentStep === FlowCreateStep.TYPE || currentStep === FlowCreateStep.CONFIGURE;

  const renderStepContent = (): JSX.Element | null => {
    if (currentStep === FlowCreateStep.TYPE) {
      return (
        <SelectFlowType
          selectedType={selectedType}
          onTypeChange={(type) => handleTypeChange(type as FlowType)}
          onReadyChange={setTypeReady}
        />
      );
    }
    if (currentStep === FlowCreateStep.TEMPLATE && selectedType) {
      return (
        <SelectFlowTemplate
          flowType={selectedType}
          selectedTemplate={selectedTemplate}
          onTemplateChange={handleTemplateChange}
        />
      );
    }
    if (currentStep === FlowCreateStep.CONFIGURE) {
      return <ConfigureFlowName value={nameValue} onChange={setNameValue} onReadyChange={setNameReady} />;
    }
    return null;
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={getStepProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        {/* Header */}
        <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Stack direction="row" alignItems="center" spacing={2}>
            <IconButton
              onClick={handleClose}
              sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
            >
              <X size={24} />
            </IconButton>
            <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
              {getBreadcrumbSteps().map((step, index, array) => {
                const isLast = index === array.length - 1;
                return isLast ? (
                  <Typography key={step} variant="h5" color="text.primary">
                    {steps[step].label}
                  </Typography>
                ) : (
                  <Typography key={step} variant="h5" onClick={() => setCurrentStep(step)} sx={{cursor: 'pointer'}}>
                    {steps[step].label}
                  </Typography>
                );
              })}
            </Breadcrumbs>
          </Stack>
        </Box>

        {/* Main content */}
        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            py: 8,
            px: 10,
            width: '100%',
          }}
        >
          {error && (
            <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
              {error}
            </Alert>
          )}

          <Box sx={isNarrowStep ? {maxWidth: 780} : undefined}>
            {renderStepContent()}

            {/* Navigation buttons */}
            <Box
              sx={{
                mt: 4,
                display: 'flex',
                justifyContent: currentStep === FlowCreateStep.TYPE ? 'flex-end' : 'space-between',
                gap: 2,
              }}
            >
              {currentStep !== FlowCreateStep.TYPE && (
                <Button
                  variant="outlined"
                  onClick={handlePrevStep}
                  sx={{minWidth: 100}}
                  disabled={createFlow.isPending}
                >
                  {t('common:actions.back', 'Back')}
                </Button>
              )}
              <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
                {createFlow.isPending && <CircularProgress size={20} />}
                <Button
                  variant="contained"
                  onClick={handleNextStep}
                  sx={{minWidth: 100}}
                  disabled={isContinueDisabled()}
                >
                  {currentStep === FlowCreateStep.CONFIGURE
                    ? t('common:actions.create', 'Create')
                    : t('common:actions.continue', 'Continue')}
                </Button>
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
