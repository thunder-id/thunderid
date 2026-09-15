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
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Home} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateVerifiablePresentation from '../api/useCreateVerifiablePresentation';
import ConfigureName from '../components/create-verifiable-presentation/ConfigureName';
import ClaimsEditor from '../components/PresentationClaimsEditor';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';
import {claimRowsToRequest, emptyClaimRow, findDuplicateClaimNames, type ClaimRow} from '../models/presentation-claims';

type Step = 'ORGANIZATION_UNIT' | 'NAME' | 'DETAILS' | 'CLAIMS';

export default function VerifiablePresentationCreatePage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('verifiable-presentations');
  const logger = useLogger('VerifiablePresentationCreatePage');
  const routes = useVerifiableCredentialRoutes();
  const createVP = useCreateVerifiablePresentation();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [step, setStep] = useState<Step>('ORGANIZATION_UNIT');
  const [name, setName] = useState<string>('');
  const [handle, setHandle] = useState<string>('');
  const [handleEdited, setHandleEdited] = useState<boolean>(false);
  const [ouId, setOuId] = useState<string>('');
  const [vct, setVct] = useState<string>('');
  const [format, setFormat] = useState<string>('dc+sd-jwt');
  const [claims, setClaims] = useState<ClaimRow[]>([emptyClaimRow()]);

  const effectiveOuId: string = ouId !== '' ? ouId : !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';

  // The organization unit whose name is shown in the Details step's summary chip.
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    effectiveOuId,
    Boolean(effectiveOuId),
  );

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, skip straight past it.
  const effectiveStep: Step = step === 'ORGANIZATION_UNIT' && !isOuLoading && !hasMultipleOUs ? 'NAME' : step;

  const stepOrder = useMemo((): Step[] => {
    const base: Step[] = [];
    if (hasMultipleOUs) base.push('ORGANIZATION_UNIT');
    base.push('NAME', 'DETAILS', 'CLAIMS');
    return base;
  }, [hasMultipleOUs]);

  const stepLabels: Record<Step, string> = {
    ORGANIZATION_UNIT: t('create.steps.organizationUnit', 'Organization Unit'),
    NAME: t('createWizard.steps.name', 'Details'),
    DETAILS: t('create.steps.details', 'Type'),
    CLAIMS: t('create.steps.claims', 'Claims'),
  };

  const duplicateClaimNames = findDuplicateClaimNames(claims);

  const stepReady: Record<Step, boolean> = {
    ORGANIZATION_UNIT: effectiveOuId !== '',
    NAME: name.trim() !== '' && handle.trim() !== '',
    DETAILS: vct.trim() !== '' && effectiveOuId !== '',
    CLAIMS: claims.some((c) => c.name.trim() !== '') && Object.keys(duplicateClaimNames).length === 0,
  };

  const stepIndex = stepOrder.indexOf(effectiveStep);
  const isLastStep = effectiveStep === 'CLAIMS';
  const progress = ((stepIndex + 1) / stepOrder.length) * 100;

  const close = (): void => {
    void navigate(routes.verifiablePresentations.list());
  };

  // A create error is stale once the user changes a field or moves between steps to fix something.
  const clearCreateError = (): void => {
    if (createVP.isError) createVP.reset();
  };

  const handleCreate = (): void => {
    createVP.mutate(
      {
        handle: handle.trim(),
        ouId: effectiveOuId,
        name: name.trim() || undefined,
        vct: vct.trim(),
        format: format.trim() || undefined,
        ...claimRowsToRequest(claims),
      },
      {
        onSuccess: () => {
          (async () => {
            await navigate(routes.verifiablePresentations.list());
          })().catch((error: unknown) => {
            logger.error('Failed to navigate after create', {error});
          });
        },
      },
    );
  };

  const handleNext = (): void => {
    if (isLastStep) {
      handleCreate();
      return;
    }
    clearCreateError();
    setStep(stepOrder[stepIndex + 1]);
  };

  const handleBack = (): void => {
    if (stepIndex > 0) {
      clearCreateError();
      setStep(stepOrder[stepIndex - 1]);
    }
  };

  const textField = (
    id: string,
    label: string,
    value: string,
    setValue: (v: string) => void,
    placeholder?: string,
    required?: boolean,
    helperText?: string,
  ): JSX.Element => (
    <FormControl fullWidth required={required}>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <TextField
        fullWidth
        id={id}
        value={value}
        placeholder={placeholder}
        helperText={helperText}
        onChange={(e: ChangeEvent<HTMLInputElement>): void => {
          clearCreateError();
          setValue(e.target.value);
        }}
      />
    </FormControl>
  );

  const renderStep = (): JSX.Element => {
    if (effectiveStep === 'NAME') {
      return (
        <ConfigureName
          name={name}
          handle={handle}
          handleEdited={handleEdited}
          onNameChange={(value: string): void => {
            clearCreateError();
            setName(value);
          }}
          onHandleChange={(value: string): void => {
            clearCreateError();
            setHandle(value);
          }}
          onHandleEditedChange={setHandleEdited}
          hasMultipleOUs={hasMultipleOUs}
          organizationUnitName={resolvedOrganizationUnit?.name}
          organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
          isOrganizationUnitLoading={isResolvedOuLoading}
          onChangeOu={() => {
            clearCreateError();
            setStep('ORGANIZATION_UNIT');
          }}
        />
      );
    }
    if (effectiveStep === 'DETAILS') {
      return (
        <Stack spacing={3}>
          {textField('vp-vct', t('form.vct.label'), vct, setVct, 'urn:eudi:pid:de:1', true, t('form.vct.hint'))}
          <FormControl fullWidth>
            <FormLabel htmlFor="vp-format">{t('form.format.label')}</FormLabel>
            <Select
              id="vp-format"
              value={format}
              onChange={(e): void => {
                clearCreateError();
                setFormat(e.target.value);
              }}
            >
              <MenuItem value="dc+sd-jwt">{t('form.format.sdJwt')}</MenuItem>
            </Select>
            <FormHelperText>{t('form.format.hint')}</FormHelperText>
          </FormControl>
        </Stack>
      );
    }
    return (
      <Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          {t('create.claims.help')}
        </Typography>
        <ClaimsEditor
          duplicateNames={duplicateClaimNames}
          claims={claims}
          onChange={(rows: ClaimRow[]): void => {
            clearCreateError();
            setClaims(rows);
          }}
        />
      </Stack>
    );
  };

  if (effectiveStep === 'ORGANIZATION_UNIT') {
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
        title={t('create.organizationUnit.title', 'Where should this verifiable presentation belong?')}
        subtitle={t(
          'create.organizationUnit.subtitle',
          "Choose the organization unit that will own this verifiable presentation. You can't change this once created.",
        )}
        value={effectiveOuId}
        onChange={(value: string): void => {
          clearCreateError();
          setOuId(value);
        }}
        onBack={close}
        onContinue={handleNext}
        backLabel={t('common:actions.back', 'Back')}
        continueLabel={t('common:actions.continue', 'Continue')}
      />
    );
  }

  return (
    <FullScreenCreationWizardLayout
      onClose={close}
      progress={progress}
      breadcrumbItems={stepOrder.slice(0, stepIndex + 1).map((s, index, array) => ({
        key: s,
        label: stepLabels[s],
        onClick:
          index < array.length - 1
            ? () => {
                clearCreateError();
                setStep(s);
              }
            : undefined,
      }))}
      footer={
        <Stack direction="row" justifyContent={stepIndex > 0 ? 'space-between' : 'flex-end'} spacing={2}>
          {stepIndex > 0 && (
            <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}} disabled={createVP.isPending}>
              {t('common:actions.back')}
            </Button>
          )}
          <Button
            variant="contained"
            sx={{minWidth: 140}}
            disabled={!stepReady[effectiveStep] || createVP.isPending}
            onClick={handleNext}
          >
            {(() => {
              if (!isLastStep) return t('common:actions.continue');
              if (createVP.isPending) return t('common:status.saving');
              return t('common:actions.create');
            })()}
          </Button>
        </Stack>
      }
    >
      {effectiveStep !== 'NAME' && (
        <>
          <Typography variant="h4" sx={{mb: 1}}>
            {t('create.title')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 4}}>
            {t('create.subtitle')}
          </Typography>
        </>
      )}

      {createVP.error && (
        <Alert severity="error" sx={{mb: 3}}>
          {getErrorMessage(createVP.error, t, 'create.error', 'Failed to create presentation definition')}
        </Alert>
      )}

      {renderStep()}
    </FullScreenCreationWizardLayout>
  );
}
