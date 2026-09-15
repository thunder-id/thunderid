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
import useCreateVerifiableCredential from '../api/useCreateVerifiableCredential';
import ConfigureName from '../components/create-verifiable-credential/ConfigureName';
import ClaimsEditor from '../components/CredentialClaimsEditor';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';
import {
  claimRowsToRequest,
  credentialToClaimRows,
  findClaimNameErrors,
  type ClaimRow,
} from '../models/credential-claims';

type Step = 'ORGANIZATION_UNIT' | 'NAME' | 'DETAILS' | 'CLAIMS';

export default function VerifiableCredentialCreatePage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('verifiable-credentials');
  const logger = useLogger('VerifiableCredentialCreatePage');
  const routes = useVerifiableCredentialRoutes();
  const createVC = useCreateVerifiableCredential();

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [step, setStep] = useState<Step>('ORGANIZATION_UNIT');
  const [name, setName] = useState<string>('');
  const [handle, setHandle] = useState<string>('');
  const [handleEdited, setHandleEdited] = useState<boolean>(false);
  const [ouId, setOuId] = useState<string>('');
  const [vct, setVct] = useState<string>('');
  const [format, setFormat] = useState<string>('dc+sd-jwt');
  const [claims, setClaims] = useState<ClaimRow[]>(credentialToClaimRows(undefined));

  const effectiveOuId: string = ouId !== '' ? ouId : !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';

  // A create error is stale once the wizard's data changes.
  const clearCreateError = (): void => {
    if (createVC.isError) createVC.reset();
  };

  const handleNameChange = (v: string): void => {
    clearCreateError();
    setName(v);
  };
  const handleHandleChange = (v: string): void => {
    clearCreateError();
    setHandle(v);
  };
  const handleOuIdChange = (v: string): void => {
    clearCreateError();
    setOuId(v);
  };
  const handleVctChange = (v: string): void => {
    clearCreateError();
    setVct(v);
  };
  const handleFormatChange = (v: string): void => {
    clearCreateError();
    setFormat(v);
  };
  const handleClaimsChange = (v: ClaimRow[]): void => {
    clearCreateError();
    setClaims(v);
  };

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

  const claimNameErrors = findClaimNameErrors(claims);

  const stepReady: Record<Step, boolean> = {
    ORGANIZATION_UNIT: effectiveOuId !== '',
    NAME: name.trim() !== '' && handle.trim() !== '',
    DETAILS: vct.trim() !== '' && effectiveOuId !== '',
    CLAIMS: claims.some((c) => c.name.trim() !== '') && Object.keys(claimNameErrors).length === 0,
  };

  const stepIndex = stepOrder.indexOf(effectiveStep);
  const isLastStep = effectiveStep === 'CLAIMS';
  const progress = ((stepIndex + 1) / stepOrder.length) * 100;

  const close = (): void => {
    void navigate(routes.verifiableCredentials.list());
  };

  const buildRequest = () => ({
    handle: handle.trim(),
    ouId: effectiveOuId,
    name: name.trim() || undefined,
    vct: vct.trim(),
    format: format.trim() || undefined,
    claims: claimRowsToRequest(claims),
  });

  const handleCreate = (): void => {
    createVC.mutate(buildRequest(), {
      onSuccess: () => {
        (async () => {
          await navigate(routes.verifiableCredentials.list());
        })().catch((error: unknown) => {
          logger.error('Failed to navigate after create', {error});
        });
      },
    });
  };

  const handleNext = (): void => {
    if (isLastStep) {
      handleCreate();
      return;
    }
    setStep(stepOrder[stepIndex + 1]);
  };

  const handleBack = (): void => {
    if (stepIndex > 0) {
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
        onChange={(e: ChangeEvent<HTMLInputElement>): void => setValue(e.target.value)}
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
          onNameChange={handleNameChange}
          onHandleChange={handleHandleChange}
          onHandleEditedChange={setHandleEdited}
          hasMultipleOUs={hasMultipleOUs}
          organizationUnitName={resolvedOrganizationUnit?.name}
          organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
          isOrganizationUnitLoading={isResolvedOuLoading}
          onChangeOu={() => setStep('ORGANIZATION_UNIT')}
        />
      );
    }
    if (effectiveStep === 'DETAILS') {
      return (
        <Stack spacing={3}>
          {textField(
            'vc-vct',
            t('form.vct.label'),
            vct,
            handleVctChange,
            'urn:eudi:pid:de:1',
            true,
            t('form.vct.hint'),
          )}
          <FormControl fullWidth>
            <FormLabel htmlFor="vc-format">{t('form.format.label')}</FormLabel>
            <Select id="vc-format" value={format} onChange={(e): void => handleFormatChange(e.target.value)}>
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
        <ClaimsEditor claims={claims} onChange={handleClaimsChange} nameErrors={claimNameErrors} />
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
        title={t('create.organizationUnit.title', 'Where should this verifiable credential belong?')}
        subtitle={t(
          'create.organizationUnit.subtitle',
          "Choose the organization unit that will own this verifiable credential. You can't change this once created.",
        )}
        value={effectiveOuId}
        onChange={handleOuIdChange}
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
        onClick: index < array.length - 1 ? () => setStep(s) : undefined,
      }))}
      footer={
        <Stack direction="row" justifyContent={stepIndex > 0 ? 'space-between' : 'flex-end'} spacing={2}>
          {stepIndex > 0 && (
            <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}} disabled={createVC.isPending}>
              {t('common:actions.back')}
            </Button>
          )}
          <Button
            variant="contained"
            sx={{minWidth: 140}}
            disabled={!stepReady[effectiveStep] || createVC.isPending}
            onClick={handleNext}
          >
            {(() => {
              if (!isLastStep) return t('common:actions.continue');
              if (createVC.isPending) return t('common:status.saving');
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

      {createVC.error && (
        <Alert severity="error" sx={{mb: 3}}>
          {getErrorMessage(createVC.error, t, 'create.error', 'Failed to create credential template')}
        </Alert>
      )}

      {renderStep()}
    </FullScreenCreationWizardLayout>
  );
}
