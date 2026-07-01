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

import {OrganizationUnitTreePicker, useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  FormControl,
  FormLabel,
  IconButton,
  LinearProgress,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronRight, X} from '@wso2/oxygen-ui-icons-react';
import {useState, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateVerifiablePresentation from '../api/useCreateVerifiablePresentation';
import ClaimsEditor from '../components/ClaimsEditor';
import {claimRowsToRequest, emptyClaimRow, type ClaimRow} from '../models/claims';

type Step = 'DETAILS' | 'CLAIMS';
const STEP_ORDER: Step[] = ['DETAILS', 'CLAIMS'];

export default function VerifiablePresentationCreatePage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('verifiable-presentations');
  const logger = useLogger('VerifiablePresentationCreatePage');
  const createVP = useCreateVerifiablePresentation();

  const {hasMultipleOUs, ouList} = useHasMultipleOUs();

  const [step, setStep] = useState<Step>('DETAILS');
  const [handle, setHandle] = useState<string>('');
  const [ouId, setOuId] = useState<string>('');
  const [displayName, setDisplayName] = useState<string>('');
  const [vct, setVct] = useState<string>('');
  const [format, setFormat] = useState<string>('dc+sd-jwt');
  const [claims, setClaims] = useState<ClaimRow[]>([emptyClaimRow()]);

  const effectiveOuId: string = ouId !== '' ? ouId : !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';

  const stepLabels: Record<Step, string> = {
    DETAILS: t('create.steps.details'),
    CLAIMS: t('create.steps.claims'),
  };

  const stepReady: Record<Step, boolean> = {
    DETAILS: handle.trim() !== '' && vct.trim() !== '' && effectiveOuId !== '',
    CLAIMS: claims.some((c) => c.name.trim() !== ''),
  };

  const stepIndex = STEP_ORDER.indexOf(step);
  const isLastStep = step === 'CLAIMS';
  const progress = ((stepIndex + 1) / STEP_ORDER.length) * 100;

  const close = (): void => {
    void navigate('/verifiable-presentations');
  };

  const handleCreate = (): void => {
    createVP.mutate(
      {
        handle: handle.trim(),
        ouId: effectiveOuId,
        displayName: displayName.trim() || undefined,
        vct: vct.trim(),
        format: format.trim() || undefined,
        ...claimRowsToRequest(claims),
      },
      {
        onSuccess: () => {
          (async () => {
            await navigate('/verifiable-presentations');
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
    setStep(STEP_ORDER[stepIndex + 1]);
  };

  const handleBack = (): void => {
    if (stepIndex > 0) {
      setStep(STEP_ORDER[stepIndex - 1]);
    }
  };

  const textField = (
    id: string,
    label: string,
    value: string,
    setValue: (v: string) => void,
    placeholder?: string,
    required?: boolean,
  ): JSX.Element => (
    <FormControl fullWidth required={required}>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <TextField
        fullWidth
        id={id}
        value={value}
        placeholder={placeholder}
        onChange={(e: ChangeEvent<HTMLInputElement>): void => setValue(e.target.value)}
      />
    </FormControl>
  );

  const renderStep = (): JSX.Element => {
    if (step === 'DETAILS') {
      return (
        <Stack spacing={3}>
          {textField('vp-handle', t('form.handle.label'), handle, setHandle, 'eudi-pid', true)}
          {textField('vp-display-name', t('form.displayName.label'), displayName, setDisplayName, 'EUDI Wallet PID')}
          {textField('vp-vct', t('form.vct.label'), vct, setVct, 'urn:eudi:pid:de:1', true)}
          <FormControl fullWidth>
            <FormLabel htmlFor="vp-format">{t('form.format.label')}</FormLabel>
            <Select id="vp-format" value={format} onChange={(e): void => setFormat(e.target.value)}>
              <MenuItem value="dc+sd-jwt">{t('form.format.sdJwt')}</MenuItem>
            </Select>
          </FormControl>
          {hasMultipleOUs && (
            <FormControl fullWidth required>
              <FormLabel>{t('form.organizationUnit.label')}</FormLabel>
              <OrganizationUnitTreePicker id="vp-ou-picker" value={effectiveOuId} onChange={setOuId} maxHeight={320} />
            </FormControl>
          )}
        </Stack>
      );
    }
    return (
      <Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          {t('create.claims.help')}
        </Typography>
        <ClaimsEditor claims={claims} onChange={setClaims} />
      </Stack>
    );
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={progress} sx={{height: 6}} />

      <Box sx={{p: 4, display: 'flex', alignItems: 'center'}}>
        <Stack direction="row" alignItems="center" spacing={2}>
          <IconButton
            aria-label={t('common:actions.close')}
            onClick={close}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
          >
            <X size={24} />
          </IconButton>
          <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
            {STEP_ORDER.slice(0, stepIndex + 1).map((s, index, array) => {
              const isCurrent = index === array.length - 1;
              return isCurrent ? (
                <Typography key={s} variant="h5" color="text.primary">
                  {stepLabels[s]}
                </Typography>
              ) : (
                <Typography
                  key={s}
                  variant="h5"
                  color="inherit"
                  role="button"
                  tabIndex={0}
                  onClick={(): void => setStep(s)}
                  onKeyDown={(e): void => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      setStep(s);
                    }
                  }}
                  sx={{cursor: 'pointer', '&:hover': {textDecoration: 'underline'}}}
                >
                  {stepLabels[s]}
                </Typography>
              );
            })}
          </Breadcrumbs>
        </Stack>
      </Box>

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column', py: 8, px: 20}}>
        <Box sx={{width: '100%', maxWidth: 800}}>
          <Typography variant="h4" sx={{mb: 1}}>
            {t('create.title')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 4}}>
            {t('create.subtitle')}
          </Typography>

          {createVP.error && (
            <Alert severity="error" sx={{mb: 3}}>
              {createVP.error.message}
            </Alert>
          )}

          {renderStep()}

          <Stack direction="row" justifyContent="flex-end" spacing={2} sx={{mt: 4}}>
            {stepIndex > 0 && (
              <Button variant="text" onClick={handleBack} disabled={createVP.isPending}>
                {t('common:actions.back')}
              </Button>
            )}
            <Button
              variant="contained"
              sx={{minWidth: 140}}
              disabled={!stepReady[step] || createVP.isPending}
              onClick={handleNext}
            >
              {(() => {
                if (!isLastStep) return t('common:actions.continue');
                if (createVP.isPending) return t('common:status.saving');
                return t('common:actions.create');
              })()}
            </Button>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
