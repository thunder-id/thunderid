// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {
  OrganizationUnitPickerScreen,
  useGetOrganizationUnit,
  useHasMultipleOUs,
} from '@thunderid/configure-organization-units';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Button, CircularProgress, Alert} from '@wso2/oxygen-ui';
import {Home} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useEffect, useMemo} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateGroup from '../api/useCreateGroup';
import ConfigureName from '../components/create-group/ConfigureName';
import GroupConstraints from '../constants/group-constraints';
import useGroupCreate from '../contexts/GroupCreate/useGroupCreate';
import {GroupCreateFlowStep} from '../models/group-create-flow';
import type {CreateGroupRequest} from '../models/requests';

export default function CreateGroupPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('groups');
  const logger = useLogger('CreateGroupPage');
  const createGroup = useCreateGroup();

  const {currentStep, setCurrentStep, name, setName, ouId, setOuId, error, setError} = useGroupCreate();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, skip straight past it.
  useEffect(() => {
    if (!isOuLoading && !hasMultipleOUs && currentStep === GroupCreateFlowStep.ORGANIZATION_UNIT) {
      setCurrentStep(GroupCreateFlowStep.NAME);
    }
  }, [isOuLoading, hasMultipleOUs, currentStep, setCurrentStep]);

  // The organization unit whose name is shown in the Details step's summary chip.
  const resolvedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    resolvedOuId,
    Boolean(resolvedOuId),
  );

  const [stepReady, setStepReady] = useState<Record<GroupCreateFlowStep, boolean>>({
    ORGANIZATION_UNIT: false,
    NAME: false,
  });

  const activeSteps = useMemo((): GroupCreateFlowStep[] => {
    const base: GroupCreateFlowStep[] = [];
    if (hasMultipleOUs) base.push(GroupCreateFlowStep.ORGANIZATION_UNIT);
    base.push(GroupCreateFlowStep.NAME);
    return base;
  }, [hasMultipleOUs]);

  const steps: Partial<Record<GroupCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<GroupCreateFlowStep, {label: string}>> = {};
    if (hasMultipleOUs) {
      map.ORGANIZATION_UNIT = {label: t('groups:createWizard.steps.organizationUnit', 'Organization Unit')};
    }
    map.NAME = {label: t('groups:createWizard.steps.name', 'Details')};
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

  const handleSubmit = async (): Promise<void> => {
    setError(null);

    const trimmedName = name.trim();

    if (!trimmedName) {
      setError(t('create.form.name.required', 'Group name is required'));
      return;
    }

    if (trimmedName.length > GroupConstraints.NAME_MAX_LENGTH) {
      setError(
        t('create.form.name.maxLength', {
          max: GroupConstraints.NAME_MAX_LENGTH,
          defaultValue: `Group name cannot exceed ${GroupConstraints.NAME_MAX_LENGTH} characters`,
        }),
      );
      return;
    }

    // If only one OU, use it directly
    if (!resolvedOuId) {
      setError(t('create.form.organizationUnit.required', 'Organization unit is required'));
      return;
    }

    const requestData: CreateGroupRequest = {
      name: trimmedName,
      ouId: resolvedOuId,
    };

    try {
      await createGroup.mutateAsync(requestData);
      await navigate(listUrl);
    } catch (submitError) {
      logger.error('Failed to create group or navigate', {error: submitError});
      setError(getErrorMessage(submitError as Error, t, 'create.error', 'Failed to create group. Please try again.'));
    }
  };

  const handleNextStep = (): void => {
    switch (currentStep) {
      case GroupCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(GroupCreateFlowStep.NAME);
        break;
      case GroupCreateFlowStep.NAME:
        if (isOuLoading) return;
        handleSubmit().catch(() => {
          // Error handled in handleSubmit
        });
        break;
      default:
        break;
    }
  };

  const handlePrevStep = (): void => {
    if (currentStep === GroupCreateFlowStep.NAME && hasMultipleOUs) {
      setCurrentStep(GroupCreateFlowStep.ORGANIZATION_UNIT);
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case GroupCreateFlowStep.NAME:
        return (
          <ConfigureName
            name={name}
            onNameChange={setName}
            onReadyChange={handleNameStepReadyChange}
            hasMultipleOUs={hasMultipleOUs}
            organizationUnitName={resolvedOrganizationUnit?.name}
            organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
            isOrganizationUnitLoading={isResolvedOuLoading}
            onChangeOu={() => setCurrentStep(GroupCreateFlowStep.ORGANIZATION_UNIT)}
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

  if (currentStep === GroupCreateFlowStep.ORGANIZATION_UNIT) {
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
        title={t('groups:createWizard.organizationUnit.title', 'Where should this group belong?')}
        subtitle={t(
          'groups:createWizard.organizationUnit.subtitle',
          "Choose the organization unit that will own this group. You can't change this once created.",
        )}
        value={ouId}
        onChange={setOuId}
        onBack={handleClose}
        onContinue={handleNextStep}
        backLabel={t('common:actions.back', 'Back')}
        continueLabel={t('common:actions.continue', 'Continue')}
      />
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
        <Box
          sx={{
            display: 'flex',
            justifyContent: activeSteps.indexOf(currentStep) > 0 ? 'space-between' : 'flex-end',
            gap: 2,
          }}
        >
          {activeSteps.indexOf(currentStep) > 0 && (
            <Button variant="outlined" onClick={handlePrevStep} sx={{minWidth: 100}} disabled={createGroup.isPending}>
              {t('common:actions.back', 'Back')}
            </Button>
          )}

          <Button
            variant="contained"
            disabled={!stepReady[currentStep] || createGroup.isPending || isOuLoading}
            sx={{minWidth: 100}}
            onClick={handleNextStep}
          >
            {(() => {
              if (createGroup.isPending) return t('common:status.saving', 'Saving...');
              return t('common:actions.continue', 'Continue');
            })()}
          </Button>
        </Box>
      }
    >
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {renderStepContent()}
    </FullScreenCreationWizardLayout>
  );
}
