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
import {useToast} from '@thunderid/contexts';
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
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateResourceServer from '../api/useCreateResourceServer';
import ConfigureName from '../components/create-resource-server/ConfigureName';
import ConfigureOrgUnit from '../components/create-resource-server/ConfigureOrgUnit';
import ConfigureSeparator from '../components/create-resource-server/ConfigureSeparator';
import ConfigureType from '../components/create-resource-server/ConfigureType';
import {DEFAULT_PERMISSION_DELIMITER} from '../constants/permission-constants';
import type {PermissionDelimiter} from '../models/permissions';
import type {ResourceServerType} from '../models/resource-server';
import {deriveHandle} from '../utils/deriveHandle';

const ResourceServerCreateStep = {
  TYPE: 'TYPE',
  NAME: 'NAME',
  SEPARATOR: 'SEPARATOR',
  ORGANIZATION_UNIT: 'ORGANIZATION_UNIT',
} as const;

type ResourceServerCreateStep = keyof typeof ResourceServerCreateStep;

export default function CreateResourceServerPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('CreateResourceServerPage');
  const createResourceServer = useCreateResourceServer();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [currentStep, setCurrentStep] = useState<ResourceServerCreateStep>(ResourceServerCreateStep.TYPE);
  const [selectedType, setSelectedType] = useState<ResourceServerType | undefined>(undefined);
  const [name, setName] = useState('');
  const [handle, setHandle] = useState('');
  const [delimiter, setDelimiter] = useState<PermissionDelimiter>(DEFAULT_PERMISSION_DELIMITER);
  const [ouId, setOuId] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [handleEdited, setHandleEdited] = useState(false);

  const [stepReady, setStepReady] = useState<Record<ResourceServerCreateStep, boolean>>({
    TYPE: false,
    NAME: false,
    SEPARATOR: false,
    ORGANIZATION_UNIT: false,
  });

  const handleDelimiterChange = useCallback(
    (newDelimiter: PermissionDelimiter): void => {
      setDelimiter(newDelimiter);
      if (!handleEdited && name) {
        setHandle(deriveHandle(name, newDelimiter));
      }
    },
    [handleEdited, name],
  );

  const steps: Record<ResourceServerCreateStep, {label: string; order: number}> = useMemo(
    () => ({
      TYPE: {label: t('resourceServers:create.steps.type', 'Type'), order: 1},
      NAME: {label: t('resourceServers:create.steps.name', 'Name'), order: 2},
      SEPARATOR: {label: t('resourceServers:create.steps.separator', 'Permission Delimiter'), order: 3},
      ORGANIZATION_UNIT: {label: t('resourceServers:create.steps.organizationUnit', 'Organization'), order: 4},
    }),
    [t],
  );

  const effectiveOuId = hasMultipleOUs ? ouId : (ouList[0]?.id ?? '');

  const handleClose = (): void => {
    (async (): Promise<void> => {
      await navigate('/resource-servers');
    })().catch((err: unknown) => {
      logger.error('Failed to navigate to resource servers list', {error: err});
    });
  };

  const handleStepReadyChange = useCallback((step: ResourceServerCreateStep, isReady: boolean): void => {
    setStepReady((prev) => ({...prev, [step]: isReady}));
  }, []);

  const handleNameReadyChange = useCallback(
    (isReady: boolean): void => handleStepReadyChange(ResourceServerCreateStep.NAME, isReady),
    [handleStepReadyChange],
  );

  const handleSeparatorReadyChange = useCallback(
    (isReady: boolean): void => handleStepReadyChange(ResourceServerCreateStep.SEPARATOR, isReady),
    [handleStepReadyChange],
  );

  const handleOuReadyChange = useCallback(
    (isReady: boolean): void => handleStepReadyChange(ResourceServerCreateStep.ORGANIZATION_UNIT, isReady),
    [handleStepReadyChange],
  );

  const handleTypeSelect = useCallback(
    (value: ResourceServerType): void => {
      setSelectedType(value);
      handleStepReadyChange(ResourceServerCreateStep.TYPE, true);
    },
    [handleStepReadyChange],
  );

  const isLastStep =
    currentStep === ResourceServerCreateStep.ORGANIZATION_UNIT ||
    (currentStep === ResourceServerCreateStep.SEPARATOR && !hasMultipleOUs);

  const handleNext = (): void => {
    setError(null);

    if (currentStep === ResourceServerCreateStep.TYPE) {
      setCurrentStep(ResourceServerCreateStep.NAME);
      return;
    }

    if (currentStep === ResourceServerCreateStep.NAME) {
      setCurrentStep(ResourceServerCreateStep.SEPARATOR);
      return;
    }

    if (currentStep === ResourceServerCreateStep.SEPARATOR && !isOuLoading && hasMultipleOUs) {
      setCurrentStep(ResourceServerCreateStep.ORGANIZATION_UNIT);
      return;
    }

    const resolvedOuId = effectiveOuId;
    if (!resolvedOuId) return;

    const payload = {
      name: name.trim(),
      handle: handle.trim() || null,
      ouId: resolvedOuId,
      type: selectedType,
      delimiter,
    };

    createResourceServer.mutate(payload, {
      onSuccess: (created) => {
        showToast(t('resourceServers:create.success', 'Resource server created successfully.'), 'success');
        (async (): Promise<void> => {
          await navigate(`/resource-servers/${created.id}?tab=resources`);
        })().catch((err: unknown) => {
          logger.error('Failed to navigate after create', {error: err});
        });
      },
      onError: (err: Error) => {
        logger.error('Failed to create resource server', {error: err});
        setError(err.message);
      },
    });
  };

  const handleBack = (): void => {
    if (currentStep === ResourceServerCreateStep.ORGANIZATION_UNIT) {
      setCurrentStep(ResourceServerCreateStep.SEPARATOR);
    } else if (currentStep === ResourceServerCreateStep.SEPARATOR) {
      setCurrentStep(ResourceServerCreateStep.NAME);
    } else if (currentStep === ResourceServerCreateStep.NAME) {
      setCurrentStep(ResourceServerCreateStep.TYPE);
    }
  };

  const isNextDisabled = createResourceServer.isPending || !stepReady[currentStep] || (isLastStep && isOuLoading);

  const getProgress = (): number => {
    const totalSteps = hasMultipleOUs ? 4 : 3;
    const currentOrder = steps[currentStep].order;
    return (currentOrder / totalSteps) * 100;
  };

  const getBreadcrumbSteps = (): ResourceServerCreateStep[] => {
    const all: ResourceServerCreateStep[] = [
      ResourceServerCreateStep.TYPE,
      ResourceServerCreateStep.NAME,
      ResourceServerCreateStep.SEPARATOR,
    ];
    if (hasMultipleOUs) all.push(ResourceServerCreateStep.ORGANIZATION_UNIT);
    const idx = all.indexOf(currentStep);
    return all.slice(0, idx + 1);
  };

  const renderStep = (): JSX.Element | null => {
    switch (currentStep) {
      case ResourceServerCreateStep.TYPE:
        return <ConfigureType selectedType={selectedType} onSelect={handleTypeSelect} />;
      case ResourceServerCreateStep.NAME:
        return (
          <ConfigureName
            name={name}
            handle={handle}
            delimiter={delimiter}
            handleEdited={handleEdited}
            onHandleEditedChange={setHandleEdited}
            onNameChange={setName}
            onHandleChange={setHandle}
            onReadyChange={handleNameReadyChange}
          />
        );
      case ResourceServerCreateStep.SEPARATOR:
        return (
          <ConfigureSeparator
            delimiter={delimiter}
            handle={handle}
            onDelimiterChange={handleDelimiterChange}
            onReadyChange={handleSeparatorReadyChange}
          />
        );
      case ResourceServerCreateStep.ORGANIZATION_UNIT:
        return <ConfigureOrgUnit selectedOuId={ouId} onOuIdChange={setOuId} onReadyChange={handleOuReadyChange} />;
      default:
        return null;
    }
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={getProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'row'}}>
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

          {/* Content */}
          <Box sx={{flex: 1, display: 'flex', minHeight: 0}}>
            <Box
              sx={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                py: 8,
                px: 20,
                mx: 'auto',
              }}
            >
              <Box sx={{width: '100%', maxWidth: 800, display: 'flex', flexDirection: 'column'}}>
                {error && (
                  <Alert severity="error" sx={{mb: 4}} onClose={() => setError(null)}>
                    {error}
                  </Alert>
                )}

                {renderStep()}

                {/* Navigation */}
                <Box
                  sx={{
                    mt: 4,
                    display: 'flex',
                    justifyContent: currentStep === ResourceServerCreateStep.TYPE ? 'flex-end' : 'space-between',
                    gap: 2,
                  }}
                >
                  {currentStep !== ResourceServerCreateStep.TYPE && (
                    <Button
                      variant="outlined"
                      onClick={handleBack}
                      sx={{minWidth: 100}}
                      disabled={createResourceServer.isPending}
                    >
                      {t('common:actions.back', 'Back')}
                    </Button>
                  )}

                  <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
                    {createResourceServer.isPending && <CircularProgress size={20} />}
                    <Button variant="contained" disabled={isNextDisabled} sx={{minWidth: 100}} onClick={handleNext}>
                      {isLastStep
                        ? createResourceServer.isPending
                          ? t('resourceServers:create.creating', 'Creating…')
                          : t('resourceServers:create.submit', 'Create resource server')
                        : t('common:actions.continue', 'Continue')}
                    </Button>
                  </Box>
                </Box>
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
