// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {OrganizationUnitPickerScreen, useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
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
import {ChevronRight, Home, X} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateResourceServer from '../api/useCreateResourceServer';
import useGetDefaultResourceServer from '../api/useGetDefaultResourceServer';
import useSetDefaultResourceServer from '../api/useSetDefaultResourceServer';
import ConfigureName from '../components/create-resource-server/ConfigureName';
import ConfigureSeparator from '../components/create-resource-server/ConfigureSeparator';
import ConfigureType from '../components/create-resource-server/ConfigureType';
import {DEFAULT_PERMISSION_DELIMITER} from '../constants/permission-constants';
import useResourceServerRoutes from '../hooks/useResourceServerRoutes';
import type {PermissionDelimiter} from '../models/permissions';
import {isDefaultEligibleType, type ResourceServerType} from '../models/resource-server';

const ResourceServerCreateStep = {
  TYPE: 'TYPE',
  NAME: 'NAME',
  SEPARATOR: 'SEPARATOR',
  ORGANIZATION_UNIT: 'ORGANIZATION_UNIT',
} as const;

type ResourceServerCreateStep = keyof typeof ResourceServerCreateStep;

export default function CreateResourceServerPage(): JSX.Element {
  const navigate = useNavigate();
  const routes = useResourceServerRoutes();
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('CreateResourceServerPage');
  const createResourceServer = useCreateResourceServer();
  const setDefaultResourceServer = useSetDefaultResourceServer();
  const {data: defaultConfig, isLoading: isDefaultLoading, error: defaultError} = useGetDefaultResourceServer();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  // Only trust the config once it has resolved, so the checkbox does not flicker from unticked to
  // ticked. A declarative (read-only) default is locked: the backend rejects any write to it.
  const isDefaultReady = !isDefaultLoading && !defaultError;
  const isDefaultLocked = Boolean(defaultConfig?.readOnly?.resourceServerId);
  const canSetDefault = isDefaultReady && !isDefaultLocked;
  const hasExistingDefault = Boolean(defaultConfig?.merged?.resourceServerId);

  const [currentStep, setCurrentStep] = useState<ResourceServerCreateStep>(ResourceServerCreateStep.ORGANIZATION_UNIT);
  const [selectedType, setSelectedType] = useState<ResourceServerType | undefined>(undefined);
  const [name, setName] = useState('');
  const [identifier, setIdentifier] = useState('');
  const [delimiter, setDelimiter] = useState<PermissionDelimiter>(DEFAULT_PERMISSION_DELIMITER);
  const [ouId, setOuId] = useState('');
  const [error, setError] = useState<string | null>(null);
  // Only the user's explicit choice is stored. Until they make one it derives from the config, which
  // resolves after the first render: offer to make this the default only when the deployment has
  // none. Deriving avoids seeding state from an effect once the config arrives.
  const [makeDefaultOverride, setMakeDefaultOverride] = useState<boolean | undefined>(undefined);
  const makeDefault = makeDefaultOverride ?? !hasExistingDefault;

  const [stepReady, setStepReady] = useState<Record<ResourceServerCreateStep, boolean>>({
    TYPE: false,
    NAME: false,
    SEPARATOR: false,
    ORGANIZATION_UNIT: false,
  });

  // Resolves an error through the `resourceServers` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `resourceServers:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `resourceServers:${key}`, options),
    [t],
  );

  // A create failure is stale once the user edits any field, so every field-change path below
  // clears both the error state and the mutation's own error before applying the change. Only
  // reset the mutation once it has actually failed: resetting while it's still pending would flip
  // isPending back to false and re-enable the submit button before the in-flight request settles,
  // letting the user fire a second concurrent create.
  const clearCreateError = useCallback((): void => {
    setError(null);
    if (createResourceServer.isError) {
      createResourceServer.reset();
    }
  }, [createResourceServer]);

  const handleNameChange = useCallback(
    (newName: string): void => {
      clearCreateError();
      setName(newName);
    },
    [clearCreateError],
  );

  const handleIdentifierChange = useCallback(
    (newIdentifier: string): void => {
      clearCreateError();
      setIdentifier(newIdentifier);
    },
    [clearCreateError],
  );

  const handleOuIdChange = useCallback(
    (newOuId: string): void => {
      clearCreateError();
      setOuId(newOuId);
    },
    [clearCreateError],
  );

  const handleDelimiterChange = useCallback(
    (newDelimiter: PermissionDelimiter): void => {
      clearCreateError();
      setDelimiter(newDelimiter);
    },
    [clearCreateError],
  );

  const handleMakeDefaultChange = useCallback(
    (newMakeDefault: boolean): void => {
      clearCreateError();
      setMakeDefaultOverride(newMakeDefault);
    },
    [clearCreateError],
  );

  const steps: Record<ResourceServerCreateStep, {label: string; order: number}> = useMemo(() => {
    const ouOffset = hasMultipleOUs ? 1 : 0;
    return {
      ORGANIZATION_UNIT: {label: t('resourceServers:create.steps.organizationUnit', 'Organization'), order: 1},
      TYPE: {label: t('resourceServers:create.steps.type', 'Type'), order: 1 + ouOffset},
      NAME: {label: t('resourceServers:create.steps.name', 'Details'), order: 2 + ouOffset},
      SEPARATOR: {label: t('resourceServers:create.steps.separator', 'Permission delimiter'), order: 3 + ouOffset},
    };
  }, [t, hasMultipleOUs]);

  const effectiveOuId = hasMultipleOUs ? ouId : (ouList[0]?.id ?? '');

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, skip straight past it. Adjusted during render
  // (not an effect) so the skip happens before the OU step ever paints.
  if (!isOuLoading && !hasMultipleOUs && currentStep === ResourceServerCreateStep.ORGANIZATION_UNIT) {
    setCurrentStep(ResourceServerCreateStep.TYPE);
  }

  const handleClose = (): void => {
    (async (): Promise<void> => {
      await navigate(routes.list());
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

  const handleTypeSelect = useCallback(
    (value: ResourceServerType): void => {
      clearCreateError();
      setSelectedType(value);
      handleStepReadyChange(ResourceServerCreateStep.TYPE, true);
    },
    [clearCreateError, handleStepReadyChange],
  );

  const isLastStep = currentStep === ResourceServerCreateStep.SEPARATOR;

  const handleNext = (): void => {
    setError(null);

    if (currentStep === ResourceServerCreateStep.ORGANIZATION_UNIT) {
      setCurrentStep(ResourceServerCreateStep.TYPE);
      return;
    }

    if (currentStep === ResourceServerCreateStep.TYPE) {
      setCurrentStep(ResourceServerCreateStep.NAME);
      return;
    }

    if (currentStep === ResourceServerCreateStep.NAME) {
      setCurrentStep(ResourceServerCreateStep.SEPARATOR);
      return;
    }

    const resolvedOuId = effectiveOuId;
    if (!resolvedOuId) return;

    const payload = {
      name: name.trim(),
      identifier: identifier.trim(),
      ouId: resolvedOuId,
      type: selectedType,
      delimiter,
    };

    // Guard on the same conditions the checkbox renders under, so a type change after ticking it
    // cannot carry a stale choice into the request.
    const shouldMakeDefault = makeDefault && canSetDefault && isDefaultEligibleType(selectedType);

    createResourceServer.mutate(payload, {
      onSuccess: (created) => {
        showToast(
          selectedType === 'MCP'
            ? t('resourceServers:create.successMcp', 'MCP server created successfully.')
            : t('resourceServers:create.success', 'Resource server created successfully.'),
          'success',
        );

        const goToCreated = (): void => {
          (async (): Promise<void> => {
            await navigate(`${routes.detail(created.id)}?tab=resources`);
          })().catch((err: unknown) => {
            logger.error('Failed to navigate after create', {error: err});
          });
        };

        if (!shouldMakeDefault) {
          goToCreated();
          return;
        }

        // The server already exists by now, so a failure here must not read as a create failure.
        // Warn and continue to the created server; the default can be set from its page.
        setDefaultResourceServer.mutate(
          {resourceServerId: created.id},
          {
            onSuccess: () => goToCreated(),
            onError: (err: Error) => {
              logger.error('Failed to set the new resource server as default', {error: err});
              showToast(
                t(
                  'resourceServers:create.setDefaultError',
                  'Resource server created, but it could not be made the default.',
                ),
                'warning',
              );
              goToCreated();
            },
          },
        );
      },
      onError: (err: Error) => {
        logger.error('Failed to create resource server', {error: err});
        setError(
          getErrorMessage(err, tForErrors, 'create.error', 'Failed to create resource server. Please try again.'),
        );
      },
    });
  };

  const handleBack = (): void => {
    if (currentStep === ResourceServerCreateStep.SEPARATOR) {
      setCurrentStep(ResourceServerCreateStep.NAME);
    } else if (currentStep === ResourceServerCreateStep.NAME) {
      setCurrentStep(ResourceServerCreateStep.TYPE);
    } else if (currentStep === ResourceServerCreateStep.TYPE && hasMultipleOUs) {
      setCurrentStep(ResourceServerCreateStep.ORGANIZATION_UNIT);
    }
  };

  // The wizard is spent once the create succeeds: the follow-up default request and the navigation
  // away both still have to run, and re-enabling in between would let the user create a duplicate.
  // Keyed off isSuccess rather than the two pending flags so it never depends on the two mutations'
  // state changes landing in the same render.
  const isSubmitting =
    createResourceServer.isPending || createResourceServer.isSuccess || setDefaultResourceServer.isPending;

  const isNextDisabled = isSubmitting || !stepReady[currentStep] || (isLastStep && isOuLoading);

  const getProgress = (): number => {
    const totalSteps = hasMultipleOUs ? 4 : 3;
    const currentOrder = steps[currentStep].order;
    return (currentOrder / totalSteps) * 100;
  };

  const getBreadcrumbSteps = (): ResourceServerCreateStep[] => {
    const all: ResourceServerCreateStep[] = [];
    if (hasMultipleOUs) all.push(ResourceServerCreateStep.ORGANIZATION_UNIT);
    all.push(ResourceServerCreateStep.TYPE, ResourceServerCreateStep.NAME, ResourceServerCreateStep.SEPARATOR);
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
            identifier={identifier}
            selectedType={selectedType}
            canSetDefault={canSetDefault}
            makeDefault={makeDefault}
            onNameChange={handleNameChange}
            onIdentifierChange={handleIdentifierChange}
            onReadyChange={handleNameReadyChange}
            onMakeDefaultChange={handleMakeDefaultChange}
          />
        );
      case ResourceServerCreateStep.SEPARATOR:
        return (
          <ConfigureSeparator
            delimiter={delimiter}
            onDelimiterChange={handleDelimiterChange}
            onReadyChange={handleSeparatorReadyChange}
          />
        );
      default:
        return null;
    }
  };

  const pageTitle = t('resourceServers:listing.addResourceServer', 'Add resource server');

  if (currentStep === ResourceServerCreateStep.ORGANIZATION_UNIT) {
    if (isOuLoading || !hasMultipleOUs) {
      return (
        <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <DocumentTitle title={pageTitle} />
          <CircularProgress />
        </Box>
      );
    }

    return (
      <>
        <DocumentTitle title={pageTitle} />
        <OrganizationUnitPickerScreen
          icon={<Home size={26} />}
          title={t('resourceServers:create.orgUnit.title', 'Choose an organization unit')}
          subtitle={t(
            'resourceServers:create.orgUnit.subtitle',
            'Select which organization unit this resource server belongs to.',
          )}
          value={ouId}
          onChange={handleOuIdChange}
          onBack={handleClose}
          onContinue={handleNext}
          backLabel={t('common:actions.back', 'Back')}
          continueLabel={t('common:actions.continue', 'Continue')}
        />
      </>
    );
  }

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <DocumentTitle title={pageTitle} />
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
                py: 4,
                px: {xs: 4, md: 10},
              }}
            >
              <Box sx={{width: '100%', maxWidth: 800, display: 'flex', flexDirection: 'column'}}>
                {error && (
                  <Alert severity="error" sx={{mb: 4}} onClose={clearCreateError}>
                    {error}
                  </Alert>
                )}

                {renderStep()}

                {/* Navigation */}
                <Box
                  sx={{
                    mt: 4,
                    display: 'flex',
                    justifyContent:
                      currentStep === ResourceServerCreateStep.TYPE && !hasMultipleOUs ? 'flex-end' : 'space-between',
                    gap: 2,
                  }}
                >
                  {!(currentStep === ResourceServerCreateStep.TYPE && !hasMultipleOUs) && (
                    <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}} disabled={isSubmitting}>
                      {t('common:actions.back', 'Back')}
                    </Button>
                  )}

                  <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
                    {isSubmitting && <CircularProgress size={20} />}
                    <Button variant="contained" disabled={isNextDisabled} sx={{minWidth: 100}} onClick={handleNext}>
                      {isLastStep
                        ? isSubmitting
                          ? t('resourceServers:create.creating', 'Creating…')
                          : selectedType === 'MCP'
                            ? t('resourceServers:create.submitMcp', 'Create MCP server')
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
