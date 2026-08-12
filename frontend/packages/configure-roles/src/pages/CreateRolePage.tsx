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
import useCreateRole from '../api/useCreateRole';
import ConfigureBasicInfo from '../components/create-role/ConfigureBasicInfo';
import ConfigurePermissions from '../components/create-role/ConfigurePermissions';
import RoleConstraints from '../constants/role-constraints';
import useRoleCreate from '../contexts/RoleCreate/useRoleCreate';
import type {CreateRoleRequest} from '../models/requests';
import {RoleCreateFlowStep} from '../models/role-create-flow';

export default function CreateRolePage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('roles');
  const logger = useLogger('CreateRolePage');
  const createRole = useCreateRole();

  const {currentStep, setCurrentStep, name, setName, ouId, setOuId, error, setError, permissions, setPermissions} =
    useRoleCreate();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, skip straight past it.
  useEffect(() => {
    if (!isOuLoading && !hasMultipleOUs && currentStep === RoleCreateFlowStep.ORGANIZATION_UNIT) {
      setCurrentStep(RoleCreateFlowStep.BASIC_INFO);
    }
  }, [isOuLoading, hasMultipleOUs, currentStep, setCurrentStep]);

  // The organization unit whose name is shown in the Details step's summary chip.
  const resolvedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    resolvedOuId,
    Boolean(resolvedOuId),
  );

  const [stepReady, setStepReady] = useState<Record<RoleCreateFlowStep, boolean>>({
    ORGANIZATION_UNIT: false,
    BASIC_INFO: false,
    PERMISSIONS: true,
  });

  const activeSteps = useMemo((): RoleCreateFlowStep[] => {
    const base: RoleCreateFlowStep[] = [];
    if (hasMultipleOUs) base.push(RoleCreateFlowStep.ORGANIZATION_UNIT);
    base.push(RoleCreateFlowStep.BASIC_INFO, RoleCreateFlowStep.PERMISSIONS);
    return base;
  }, [hasMultipleOUs]);

  const steps: Partial<Record<RoleCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<RoleCreateFlowStep, {label: string}>> = {};
    if (hasMultipleOUs) {
      map.ORGANIZATION_UNIT = {label: t('createWizard.steps.organizationUnit', 'Organization Unit')};
    }
    map.BASIC_INFO = {label: t('createWizard.steps.basicInfo', 'Details')};
    map.PERMISSIONS = {label: t('createWizard.steps.permissions', 'Permissions')};
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

  const handleSubmit = async (): Promise<void> => {
    setError(null);

    const trimmedName = name.trim();

    if (!trimmedName) {
      setError(t('create.form.name.required', 'Role name is required'));
      return;
    }

    if (trimmedName.length > RoleConstraints.NAME_MAX_LENGTH) {
      setError(
        t('create.form.name.maxLength', {
          max: RoleConstraints.NAME_MAX_LENGTH,
          defaultValue: `Role name cannot exceed ${RoleConstraints.NAME_MAX_LENGTH} characters`,
        }),
      );
      return;
    }

    const selectedOuId = hasMultipleOUs ? ouId : ouList[0]?.id;
    if (!selectedOuId) {
      setError(t('create.form.organizationUnit.required', 'Organization unit is required'));
      return;
    }

    const requestData: CreateRoleRequest = {
      name: trimmedName,
      ouId: selectedOuId,
      ...(permissions.length > 0 ? {permissions} : {}),
    };

    try {
      await createRole.mutateAsync(requestData);
      await navigate(listUrl);
    } catch (submitError) {
      logger.error('Failed to create role or navigate', {error: submitError});
      setError(getErrorMessage(submitError as Error, t, 'create.error', 'Failed to create role. Please try again.'));
    }
  };

  const handleNextStep = (): void => {
    switch (currentStep) {
      case RoleCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(RoleCreateFlowStep.BASIC_INFO);
        break;
      case RoleCreateFlowStep.BASIC_INFO:
        if (isOuLoading) return;
        setCurrentStep(RoleCreateFlowStep.PERMISSIONS);
        break;
      case RoleCreateFlowStep.PERMISSIONS:
        handleSubmit().catch(() => {
          /* noop */
        });
        break;
      default:
        break;
    }
  };

  const handlePrevStep = (): void => {
    if (currentStep === RoleCreateFlowStep.PERMISSIONS) {
      setCurrentStep(RoleCreateFlowStep.BASIC_INFO);
    } else if (currentStep === RoleCreateFlowStep.BASIC_INFO && hasMultipleOUs) {
      setCurrentStep(RoleCreateFlowStep.ORGANIZATION_UNIT);
    }
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case RoleCreateFlowStep.BASIC_INFO:
        return (
          <ConfigureBasicInfo
            name={name}
            onNameChange={setName}
            onReadyChange={handleBasicInfoStepReadyChange}
            hasMultipleOUs={hasMultipleOUs}
            organizationUnitName={resolvedOrganizationUnit?.name}
            organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
            isOrganizationUnitLoading={isResolvedOuLoading}
            onChangeOu={() => setCurrentStep(RoleCreateFlowStep.ORGANIZATION_UNIT)}
          />
        );
      case RoleCreateFlowStep.PERMISSIONS:
        return <ConfigurePermissions permissions={permissions} onPermissionsChange={setPermissions} />;
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

  if (currentStep === RoleCreateFlowStep.ORGANIZATION_UNIT) {
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
        title={t('createWizard.organizationUnit.title', 'Where should this role belong?')}
        subtitle={t(
          'createWizard.organizationUnit.subtitle',
          "Choose the organization unit that will own this role. You can't change this once created.",
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
            <Button variant="outlined" onClick={handlePrevStep} sx={{minWidth: 100}} disabled={createRole.isPending}>
              {t('common:actions.back', 'Back')}
            </Button>
          )}

          <Button
            variant="contained"
            disabled={!stepReady[currentStep] || createRole.isPending || isOuLoading}
            sx={{minWidth: 100}}
            onClick={handleNextStep}
          >
            {createRole.isPending ? t('common:status.saving', 'Saving...') : t('common:actions.continue', 'Continue')}
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
