// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ReadOnlyFieldset, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {OrganizationUnitTreePicker, useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
  type ReactNode,
  type SyntheticEvent,
} from 'react';
import {useTranslation} from 'react-i18next';
import ClaimsEditor from './PresentationClaimsEditor';
import PresentationSignInCard from './PresentationSignInCard';
import TrustedIssuerCertificates from './TrustedIssuerCertificates';
import {
  claimRowsToRequest,
  definitionToClaimRows,
  findDuplicateClaimNames,
  isSignInValid,
  type ClaimRow,
} from '../models/presentation-claims';
import type {CreateVerifiablePresentationRequest, TrustedAuthorityInput} from '../models/presentation-requests';
import type {VerifiablePresentation} from '../models/vp';

export interface VerifiablePresentationFormProps {
  initial?: VerifiablePresentation;
  /** Name, edited inline in the page header rather than in this form. */
  name: string;
  /** Description, edited inline in the page header rather than in this form. */
  description: string;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
  submitting: boolean;
  submitLabel: string;
  onSubmit: (data: CreateVerifiablePresentationRequest) => void;
  /** When provided (edit mode), renders a Danger Zone delete action in the Advanced tab. */
  onDelete?: () => void;
  /** Whether the form is shown without edit controls. */
  readOnly?: boolean;
  /** Inline error to show above the save/reset actions when the last save attempt failed. */
  error?: string;
  /** Called whenever a field changes, so a stale save error can be cleared. */
  onErrorClear?: () => void;
}

interface TabPanelProps {
  children: ReactNode;
  value: number;
  index: number;
  disabled?: boolean;
}

function TabPanel({children, value, index, disabled = false}: TabPanelProps): JSX.Element {
  return (
    <div role="tabpanel" hidden={value !== index}>
      {value === index && (
        <Box sx={{pb: 3}}>
          <ReadOnlyFieldset readOnly={disabled}>{children}</ReadOnlyFieldset>
        </Box>
      )}
    </div>
  );
}

/**
 * Seeds the trusted issuer certificate entries from a stored definition. Stored entries carry only
 * their name, since the server never returns a certificate.
 */
function toTrustedAuthorityInputs(initial?: VerifiablePresentation): TrustedAuthorityInput[] {
  return (initial?.trustedAuthorities ?? []).map((name: string): TrustedAuthorityInput => ({name}));
}

/**
 * Tabbed create/edit form for an OpenID4VP presentation definition: a General
 * tab (identity + credential type) and a Claims tab (the unified claim editor).
 */
export default function VerifiablePresentationForm({
  initial = undefined,
  name,
  description,
  onNameChange,
  onDescriptionChange,
  submitting,
  submitLabel,
  onSubmit,
  onDelete = undefined,
  readOnly = false,
  error = undefined,
  onErrorClear = undefined,
}: VerifiablePresentationFormProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');
  const {can} = useCapabilitiesContext();

  const {hasMultipleOUs, ouList} = useHasMultipleOUs();

  const [tab, setTab] = useState<number>(0);
  const [handle, setHandle] = useState<string>(initial?.handle ?? '');
  const [ouId, setOuId] = useState<string>(initial?.ouId ?? '');
  const [vct, setVct] = useState<string>(initial?.vct ?? '');
  const [format, setFormat] = useState<string>(initial?.format ?? 'dc+sd-jwt');
  const [claims, setClaims] = useState<ClaimRow[]>(definitionToClaimRows(initial));
  const [trustedAuthorities, setTrustedAuthorities] = useState<TrustedAuthorityInput[]>(
    toTrustedAuthorityInputs(initial),
  );
  const [subjectClaim, setSubjectClaim] = useState<string>(initial?.subjectClaim ?? '');
  const [signInTurnedOn, setSignInTurnedOn] = useState<boolean>(false);
  const allowSignIn = signInTurnedOn || subjectClaim !== '';

  // Once a save succeeds the new certificates are stored on the server, so keep only their names: a
  // name-only entry keeps the stored certificate, and it matches the refreshed definition so the form
  // is no longer dirty (state-during-render, not an effect).
  const [wasSubmitting, setWasSubmitting] = useState<boolean>(submitting);
  if (submitting !== wasSubmitting) {
    setWasSubmitting(submitting);
    if (!submitting && !error) {
      setTrustedAuthorities((prev: TrustedAuthorityInput[]) =>
        prev.map(({name: certificateName}: TrustedAuthorityInput): TrustedAuthorityInput => ({name: certificateName})),
      );
    }
  }

  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    },
    [],
  );
  const handleCopy = useCallback(async (value: string, field: string): Promise<void> => {
    await navigator.clipboard.writeText(value);
    setCopiedField(field);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => setCopiedField(null), 2000);
  }, []);

  const definitionId = initial?.id;

  // In a single-OU deployment the sole OU is used implicitly; otherwise the picker drives ouId.
  const effectiveOuId = ouId !== '' ? ouId : !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';

  const duplicateClaimNames = findDuplicateClaimNames(claims);

  const valid =
    handle.trim() !== '' &&
    vct.trim() !== '' &&
    effectiveOuId !== '' &&
    Object.keys(duplicateClaimNames).length === 0 &&
    isSignInValid(allowSignIn, subjectClaim, claims) &&
    trustedAuthorities.length > 0;

  const snapshot = (
    h: string,
    o: string,
    n: string,
    d: string,
    v: string,
    f: string,
    cl: ClaimRow[],
    authorities: TrustedAuthorityInput[],
    subject: string,
  ): string =>
    JSON.stringify({
      handle: h.trim(),
      ouId: o,
      name: n.trim(),
      description: d.trim(),
      vct: v.trim(),
      format: f.trim(),
      ...claimRowsToRequest(cl),
      trustedAuthorities: authorities,
      subjectClaim: subject,
    });
  const initialSnapshot = useMemo(
    () =>
      snapshot(
        initial?.handle ?? '',
        initial?.ouId ?? '',
        initial?.name ?? '',
        initial?.description ?? '',
        initial?.vct ?? '',
        initial?.format ?? 'dc+sd-jwt',
        definitionToClaimRows(initial),
        toTrustedAuthorityInputs(initial),
        initial?.subjectClaim ?? '',
      ),
    [initial],
  );
  const dirty =
    snapshot(handle, effectiveOuId, name, description, vct, format, claims, trustedAuthorities, subjectClaim) !==
    initialSnapshot;

  const handleSubmit = (): void => {
    onSubmit({
      handle: handle.trim(),
      ouId: effectiveOuId,
      name: name.trim() || undefined,
      description: description.trim() || undefined,
      vct: vct.trim(),
      format: format.trim() || undefined,
      ...claimRowsToRequest(claims),
      trustedAuthorities,
      subjectClaim: subjectClaim || undefined,
    });
  };

  const handleReset = (): void => {
    setHandle(initial?.handle ?? '');
    setOuId(initial?.ouId ?? '');
    onNameChange(initial?.name ?? '');
    onDescriptionChange(initial?.description ?? '');
    setVct(initial?.vct ?? '');
    setFormat(initial?.format ?? 'dc+sd-jwt');
    setClaims(definitionToClaimRows(initial));
    setTrustedAuthorities(toTrustedAuthorityInputs(initial));
    setSubjectClaim(initial?.subjectClaim ?? '');
    setSignInTurnedOn(false);
    onErrorClear?.();
  };

  const handleAddCertificate = (entry: TrustedAuthorityInput): void => {
    onErrorClear?.();
    setTrustedAuthorities((prev: TrustedAuthorityInput[]) => [...prev, entry]);
  };

  const handleRemoveCertificate = (certificateName: string): void => {
    onErrorClear?.();
    setTrustedAuthorities((prev: TrustedAuthorityInput[]) =>
      prev.filter((entry: TrustedAuthorityInput) => entry.name !== certificateName),
    );
  };

  const text = (
    id: string,
    label: string,
    value: string,
    setValue: (v: string) => void,
    placeholder?: string,
    required?: boolean,
    helperText?: string,
  ): JSX.Element => (
    <TextField
      labelPlacement="top"
      label={label}
      required={required}
      fullWidth
      id={id}
      value={value}
      placeholder={placeholder}
      helperText={helperText}
      onChange={(e: ChangeEvent<HTMLInputElement>): void => {
        onErrorClear?.();
        setValue(e.target.value);
      }}
    />
  );

  return (
    <Stack spacing={3}>
      <Tabs
        value={tab}
        onChange={(_e: SyntheticEvent, v: number): void => setTab(v)}
        aria-label="presentation definition"
      >
        <Tab label={t('form.tabs.general')} />
        <Tab label={t('form.tabs.protocolSettings')} />
        <Tab label={t('form.tabs.claims')} />
        <Tab label={t('form.tabs.issuerTrust')} />
        {definitionId && onDelete && can('vc', 'delete') && <Tab label={t('form.tabs.advanced', 'Advanced')} />}
      </Tabs>

      <TabPanel value={tab} index={0}>
        <Stack spacing={3}>
          {definitionId && (
            <SettingsCard title={t('form.quickCopy.title')} description={t('form.quickCopy.description')}>
              <Stack spacing={3}>
                <TextField
                  labelPlacement="top"
                  label={t('form.id.label')}
                  fullWidth
                  id="vp-id"
                  value={definitionId}
                  InputProps={{
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <Tooltip title={copiedField === 'id' ? t('common:actions.copied') : t('form.copyId')}>
                          <IconButton
                            aria-label={t('form.copyId')}
                            edge="end"
                            onClick={(): void => {
                              handleCopy(definitionId, 'id').catch(() => null);
                            }}
                          >
                            {copiedField === 'id' ? <Check size={16} /> : <Copy size={16} />}
                          </IconButton>
                        </Tooltip>
                      </InputAdornment>
                    ),
                  }}
                  helperText={t('form.quickCopy.idHint')}
                  sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
                <TextField
                  labelPlacement="top"
                  label={t('form.handle.label')}
                  fullWidth
                  id="vp-handle"
                  value={handle}
                  InputProps={{
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <Tooltip
                          title={copiedField === 'handle' ? t('common:actions.copied') : t('common:actions.copy')}
                        >
                          <IconButton
                            aria-label={t('common:actions.copy')}
                            edge="end"
                            onClick={(): void => {
                              handleCopy(handle, 'handle').catch(() => null);
                            }}
                          >
                            {copiedField === 'handle' ? <Check size={16} /> : <Copy size={16} />}
                          </IconButton>
                        </Tooltip>
                      </InputAdornment>
                    ),
                  }}
                  helperText={t('form.handle.hint')}
                  sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
              </Stack>
            </SettingsCard>
          )}

          <SettingsCard title={t('form.organizationUnit.title')} description={t('form.organizationUnit.description')}>
            <Stack spacing={3}>
              {!definitionId && hasMultipleOUs && (
                <FormControl fullWidth required>
                  <FormLabel>{t('form.organizationUnit.label')}</FormLabel>
                  <OrganizationUnitTreePicker
                    id="vp-ou-picker"
                    value={effectiveOuId}
                    onChange={(value: string): void => {
                      onErrorClear?.();
                      setOuId(value);
                    }}
                    maxHeight={320}
                    helperText={t('form.organizationUnit.pickerHint')}
                  />
                </FormControl>
              )}
              {definitionId && (
                <>
                  <TextField
                    labelPlacement="top"
                    label={t('form.organizationUnit.handleLabel')}
                    id="vp-ou-handle"
                    fullWidth
                    size="small"
                    value={initial?.ouHandle ?? '-'}
                    InputProps={{
                      readOnly: true,
                      endAdornment: initial?.ouHandle ? (
                        <InputAdornment position="end">
                          <Tooltip
                            title={copiedField === 'ouHandle' ? t('common:actions.copied') : t('common:actions.copy')}
                          >
                            <IconButton
                              aria-label={t('common:actions.copy')}
                              edge="end"
                              onClick={(): void => {
                                handleCopy(initial.ouHandle!, 'ouHandle').catch(() => null);
                              }}
                            >
                              {copiedField === 'ouHandle' ? <Check size={16} /> : <Copy size={16} />}
                            </IconButton>
                          </Tooltip>
                        </InputAdornment>
                      ) : undefined,
                    }}
                    helperText={t('form.organizationUnit.handleHint')}
                    sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                  />
                  <TextField
                    labelPlacement="top"
                    label={t('form.organizationUnit.idLabel')}
                    id="vp-ou-id"
                    fullWidth
                    size="small"
                    value={initial?.ouId ?? ''}
                    InputProps={{
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <Tooltip
                            title={copiedField === 'ouId' ? t('common:actions.copied') : t('common:actions.copy')}
                          >
                            <IconButton
                              aria-label={t('common:actions.copy')}
                              edge="end"
                              onClick={(): void => {
                                handleCopy(initial?.ouId ?? '', 'ouId').catch(() => null);
                              }}
                            >
                              {copiedField === 'ouId' ? <Check size={16} /> : <Copy size={16} />}
                            </IconButton>
                          </Tooltip>
                        </InputAdornment>
                      ),
                    }}
                    helperText={t('form.organizationUnit.idHint')}
                    sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                  />
                </>
              )}
            </Stack>
          </SettingsCard>
        </Stack>
      </TabPanel>

      <TabPanel value={tab} index={1} disabled={readOnly}>
        <SettingsCard title={t('form.protocol.title')} description={t('form.protocol.description')}>
          <Stack spacing={3}>
            {text('vp-vct', t('form.vct.label'), vct, setVct, 'urn:eudi:pid:de:1', true, t('form.vct.hint'))}
            <FormControl fullWidth>
              <FormLabel htmlFor="vp-format">{t('form.format.label')}</FormLabel>
              <Select
                id="vp-format"
                value={format}
                disabled={readOnly}
                onChange={(e): void => {
                  onErrorClear?.();
                  setFormat(e.target.value);
                }}
              >
                <MenuItem value="dc+sd-jwt">{t('form.format.sdJwt')}</MenuItem>
              </Select>
              <FormHelperText>{t('form.format.hint')}</FormHelperText>
            </FormControl>
          </Stack>
        </SettingsCard>
      </TabPanel>

      <TabPanel value={tab} index={2} disabled={readOnly}>
        <Stack spacing={3}>
          <ClaimsEditor
            readOnly={readOnly}
            duplicateNames={duplicateClaimNames}
            claims={claims}
            onChange={(rows: ClaimRow[]): void => {
              onErrorClear?.();
              setClaims(rows);
            }}
          />
          <PresentationSignInCard
            claims={claims}
            allowSignIn={allowSignIn}
            subjectClaim={subjectClaim}
            onAllowSignInChange={(on: boolean): void => {
              onErrorClear?.();
              setSignInTurnedOn(on);
              if (!on) setSubjectClaim('');
            }}
            onSubjectClaimChange={(claim: string): void => {
              onErrorClear?.();
              setSubjectClaim(claim);
            }}
            storedSubjectClaim={initial?.subjectClaim}
          />
        </Stack>
      </TabPanel>

      <TabPanel value={tab} index={3} disabled={readOnly}>
        <SettingsCard title={t('form.issuerTrust.title')} description={t('form.issuerTrust.description')}>
          <Stack spacing={2}>
            <Box>
              <FormLabel sx={{fontWeight: 600, fontSize: (theme) => theme.typography.body1.fontSize}}>
                {t('form.issuerTrust.certificates.title', 'Trusted issuer certificates')}
              </FormLabel>
              <Typography variant="body2" color="text.secondary">
                {t(
                  'form.issuerTrust.certificates.hint',
                  'Certificates of the issuers, or of the CAs that sign them, that this presentation definition trusts.',
                )}
              </Typography>
            </Box>
            <TrustedIssuerCertificates
              certificates={trustedAuthorities}
              onAdd={handleAddCertificate}
              onRemove={handleRemoveCertificate}
              markUnsaved
            />
          </Stack>
        </SettingsCard>
      </TabPanel>

      {definitionId && onDelete && can('vc', 'delete') && (
        <TabPanel value={tab} index={4}>
          <SettingsCard title={t('form.dangerZone.title')} description={t('form.dangerZone.description')}>
            <Typography variant="h6" gutterBottom color="error">
              {t('form.dangerZone.delete')}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
              {t('form.dangerZone.deleteDescription')}
            </Typography>
            <Button variant="contained" color="error" onClick={onDelete}>
              {t('common:actions.delete')}
            </Button>
          </SettingsCard>
        </TabPanel>
      )}

      {dirty && !readOnly && (
        <UnsavedChangesBar
          message={t('form.unsavedChanges')}
          resetLabel={t('common:actions.reset')}
          saveLabel={submitLabel}
          savingLabel={t('common:status.saving')}
          isSaving={submitting}
          saveDisabled={!valid || !can('vc', 'update')}
          error={error}
          onReset={handleReset}
          onSave={handleSubmit}
        />
      )}
    </Stack>
  );
}
