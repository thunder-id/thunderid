// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateFlow from '../api/useCreateFlow';
import ConfigureFlowName from '../components/create-flow/ConfigureFlowName';
import type {ConfigureFlowNameValue} from '../components/create-flow/ConfigureFlowName';
import SelectFlowTemplate from '../components/create-flow/SelectFlowTemplate';
import SelectFlowType from '../components/create-flow/SelectFlowType';
import useFlowRoutes from '../hooks/useFlowRoutes';
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
  const flowRoutes = useFlowRoutes();
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
      [FlowCreateStep.CONFIGURE]: {label: t('flows:create.steps.configure', 'Details'), order: 3},
    }),
    [t],
  );

  const handleClose = (): void => {
    void navigate(flowRoutes.flows.list());
  };

  // Resolves an error through the `flows` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `flows:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `flows:${key}`, options),
    [t],
  );

  // A create failure is stale once the user edits any field feeding the create request, so every
  // field-change path below clears both the validation error and the mutation's own error before
  // applying the change. Only reset the mutation once it has actually failed: resetting while it's
  // still pending would flip isPending back to false and re-enable the submit button before the
  // in-flight request settles, letting the user fire a second concurrent create.
  const clearCreateError = useCallback((): void => {
    setError(null);
    if (createFlow.isError) {
      createFlow.reset();
    }
  }, [createFlow]);

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
            await navigate(flowRoutes.flows.detail(savedFlow.id));
          })().catch((_error: unknown) => {
            logger.error('Failed to navigate to flow builder', {error: _error, flowId: savedFlow.id});
          });
        },
        onError: (err) => {
          setError(
            getErrorMessage(err, tForErrors, 'create.error.createFailed', 'Failed to create flow. Please try again.'),
          );
        },
      });
    }
  };

  const handlePrevStep = (): void => {
    if (currentStep === FlowCreateStep.TEMPLATE) setCurrentStep(FlowCreateStep.TYPE);
    if (currentStep === FlowCreateStep.CONFIGURE) setCurrentStep(FlowCreateStep.TEMPLATE);
  };

  const handleTypeChange = (type: FlowType): void => {
    clearCreateError();
    setSelectedType(type);
    setSelectedTemplate(null);
  };

  const handleTemplateChange = (template: FlowTemplate): void => {
    clearCreateError();
    setSelectedTemplate(template);
  };

  const handleNameValueChange = (value: ConfigureFlowNameValue): void => {
    clearCreateError();
    setNameValue(value);
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
      return <ConfigureFlowName value={nameValue} onChange={handleNameValueChange} onReadyChange={setNameReady} />;
    }
    return null;
  };

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={getStepProgress()}
      contentMaxWidth={currentStep === FlowCreateStep.TEMPLATE ? false : 800}
      breadcrumbItems={getBreadcrumbSteps().map((step, index, array) => ({
        key: step,
        label: steps[step].label,
        onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
      }))}
      footer={
        <Box
          sx={{
            display: 'flex',
            justifyContent: currentStep === FlowCreateStep.TYPE ? 'flex-end' : 'space-between',
            gap: 2,
          }}
        >
          {currentStep !== FlowCreateStep.TYPE && (
            <Button variant="outlined" onClick={handlePrevStep} sx={{minWidth: 100}} disabled={createFlow.isPending}>
              {t('common:actions.back', 'Back')}
            </Button>
          )}
          <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
            {createFlow.isPending && <CircularProgress size={20} />}
            <Button variant="contained" onClick={handleNextStep} sx={{minWidth: 100}} disabled={isContinueDisabled()}>
              {currentStep === FlowCreateStep.CONFIGURE
                ? t('common:actions.create', 'Create')
                : t('common:actions.continue', 'Continue')}
            </Button>
          </Box>
        </Box>
      }
    >
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={clearCreateError}>
          {error}
        </Alert>
      )}

      {renderStepContent()}
    </FullScreenCreationWizardLayout>
  );
}
