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

import {SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {OrganizationUnitTreePicker, useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  IconButton,
  InputAdornment,
  ListItemText,
  MenuItem,
  Select,
  Stack,
  Switch,
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
import ClaimsEditor from './ClaimsEditor';
import useGetTrustAnchors from '../api/useGetTrustAnchors';
import {claimRowsToRequest, definitionToClaimRows, type ClaimRow} from '../models/claims';
import type {CreateVerifiablePresentationRequest} from '../models/requests';
import type {TrustAnchor, VerifiablePresentation} from '../models/vp';

export interface VerifiablePresentationFormProps {
  initial?: VerifiablePresentation;
  submitting: boolean;
  submitLabel: string;
  onSubmit: (data: CreateVerifiablePresentationRequest) => void;
  /** When provided (edit mode), renders a Danger Zone delete action in the General tab. */
  onDelete?: () => void;
}

interface TabPanelProps {
  children: ReactNode;
  value: number;
  index: number;
}

function TabPanel({children, value, index}: TabPanelProps): JSX.Element {
  return (
    <div role="tabpanel" hidden={value !== index}>
      {value === index && <Box sx={{pt: 3}}>{children}</Box>}
    </div>
  );
}

/**
 * Tabbed create/edit form for an OpenID4VP presentation definition: a General
 * tab (identity + credential type) and a Claims tab (the unified claim editor).
 */
export default function VerifiablePresentationForm({
  initial = undefined,
  submitting,
  submitLabel,
  onSubmit,
  onDelete = undefined,
}: VerifiablePresentationFormProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');

  const {hasMultipleOUs, ouList} = useHasMultipleOUs();

  const [tab, setTab] = useState<number>(0);
  const [handle, setHandle] = useState<string>(initial?.handle ?? '');
  const [ouId, setOuId] = useState<string>(initial?.ouId ?? '');
  const [displayName, setDisplayName] = useState<string>(initial?.displayName ?? '');
  const [vct, setVct] = useState<string>(initial?.vct ?? '');
  const [format, setFormat] = useState<string>(initial?.format ?? 'dc+sd-jwt');
  const [claims, setClaims] = useState<ClaimRow[]>(definitionToClaimRows(initial));
  const [enforceTrustedIssuer, setEnforceTrustedIssuer] = useState<boolean>(initial?.enforceTrustedIssuer ?? false);
  const [trustedAuthorities, setTrustedAuthorities] = useState<string[]>(initial?.trustedAuthorities ?? []);

  const {data: trustAnchors = [], isLoading: trustAnchorsLoading} = useGetTrustAnchors();

  // Issuer-trust enforcement is only meaningful when trust anchors exist; without
  // any, the verifier rejects every presentation, so the toggle is disabled. Guard
  // only once the anchors have loaded, so an existing enforce=true value is not
  // spuriously cleared during the initial fetch.
  const noTrustAnchors = !trustAnchorsLoading && trustAnchors.length === 0;

  const [copied, setCopied] = useState<boolean>(false);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    },
    [],
  );
  const handleCopy = useCallback(async (value: string): Promise<void> => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
  }, []);

  const definitionId = initial?.id;

  // In a single-OU deployment the sole OU is used implicitly; otherwise the picker drives ouId.
  const effectiveOuId = ouId !== '' ? ouId : !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';

  const valid = handle.trim() !== '' && vct.trim() !== '' && effectiveOuId !== '';

  const snapshot = (
    h: string,
    o: string,
    dn: string,
    v: string,
    f: string,
    cl: ClaimRow[],
    enforce: boolean,
    authorities: string[],
  ): string =>
    JSON.stringify({
      handle: h.trim(),
      ouId: o,
      displayName: dn.trim(),
      vct: v.trim(),
      format: f.trim(),
      ...claimRowsToRequest(cl),
      enforceTrustedIssuer: enforce,
      trustedAuthorities: authorities,
    });
  const initialSnapshot = useMemo(
    () =>
      snapshot(
        initial?.handle ?? '',
        initial?.ouId ?? '',
        initial?.displayName ?? '',
        initial?.vct ?? '',
        initial?.format ?? 'dc+sd-jwt',
        definitionToClaimRows(initial),
        initial?.enforceTrustedIssuer ?? false,
        initial?.trustedAuthorities ?? [],
      ),
    [initial],
  );
  // Never enforce issuer trust when no anchors exist (it would reject everything).
  const effectiveEnforce = enforceTrustedIssuer && !noTrustAnchors;
  const dirty =
    snapshot(handle, effectiveOuId, displayName, vct, format, claims, effectiveEnforce, trustedAuthorities) !==
    initialSnapshot;

  const handleSubmit = (): void => {
    onSubmit({
      handle: handle.trim(),
      ouId: effectiveOuId,
      displayName: displayName.trim() || undefined,
      vct: vct.trim(),
      format: format.trim() || undefined,
      ...claimRowsToRequest(claims),
      enforceTrustedIssuer: effectiveEnforce,
      trustedAuthorities: trustedAuthorities.length > 0 ? trustedAuthorities : undefined,
    });
  };

  const handleReset = (): void => {
    setHandle(initial?.handle ?? '');
    setOuId(initial?.ouId ?? '');
    setDisplayName(initial?.displayName ?? '');
    setVct(initial?.vct ?? '');
    setFormat(initial?.format ?? 'dc+sd-jwt');
    setClaims(definitionToClaimRows(initial));
    setEnforceTrustedIssuer(initial?.enforceTrustedIssuer ?? false);
    setTrustedAuthorities(initial?.trustedAuthorities ?? []);
  };

  const text = (
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

  return (
    <Stack spacing={3}>
      <Tabs
        value={tab}
        onChange={(_e: SyntheticEvent, v: number): void => setTab(v)}
        aria-label="presentation definition"
      >
        <Tab label={t('form.tabs.general')} />
        <Tab label={t('form.tabs.claims')} />
        <Tab label={t('form.tabs.issuerTrust')} />
      </Tabs>

      <TabPanel value={tab} index={0}>
        <Stack spacing={3}>
          {definitionId && (
            <SettingsCard title={t('form.quickCopy.title')} description={t('form.quickCopy.description')}>
              <FormControl fullWidth>
                <FormLabel htmlFor="vp-id">{t('form.id.label')}</FormLabel>
                <TextField
                  fullWidth
                  id="vp-id"
                  value={definitionId}
                  InputProps={{
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <Tooltip title={copied ? t('common:actions.copied') : t('form.copyId')}>
                          <IconButton
                            aria-label={t('form.copyId')}
                            edge="end"
                            onClick={(): void => {
                              handleCopy(definitionId).catch(() => null);
                            }}
                          >
                            {copied ? <Check size={16} /> : <Copy size={16} />}
                          </IconButton>
                        </Tooltip>
                      </InputAdornment>
                    ),
                  }}
                  sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
              </FormControl>
            </SettingsCard>
          )}

          <SettingsCard title={t('form.details.title')} description={t('form.details.description')}>
            <Stack spacing={3}>
              {text('vp-handle', t('form.handle.label'), handle, setHandle, 'eudi-pid', true)}
              {text('vp-display-name', t('form.displayName.label'), displayName, setDisplayName, 'EUDI Wallet PID')}
              {text('vp-vct', t('form.vct.label'), vct, setVct, 'urn:eudi:pid:de:1', true)}
              <FormControl fullWidth>
                <FormLabel htmlFor="vp-format">{t('form.format.label')}</FormLabel>
                <Select id="vp-format" value={format} onChange={(e): void => setFormat(e.target.value)}>
                  <MenuItem value="dc+sd-jwt">{t('form.format.sdJwt')}</MenuItem>
                </Select>
              </FormControl>
              {!definitionId && hasMultipleOUs && (
                <FormControl fullWidth required>
                  <FormLabel>{t('form.organizationUnit.label')}</FormLabel>
                  <OrganizationUnitTreePicker
                    id="vp-ou-picker"
                    value={effectiveOuId}
                    onChange={setOuId}
                    maxHeight={320}
                  />
                </FormControl>
              )}
              {definitionId && (
                <FormControl fullWidth>
                  <FormLabel htmlFor="vp-ou">{t('form.organizationUnit.label')}</FormLabel>
                  <TextField
                    id="vp-ou"
                    fullWidth
                    size="small"
                    value={initial?.ouHandle ?? initial?.ouId ?? ''}
                    slotProps={{input: {readOnly: true}}}
                    sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                  />
                </FormControl>
              )}
            </Stack>
          </SettingsCard>

          {definitionId && onDelete && (
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
          )}
        </Stack>
      </TabPanel>

      <TabPanel value={tab} index={1}>
        <ClaimsEditor claims={claims} onChange={setClaims} />
      </TabPanel>

      <TabPanel value={tab} index={2}>
        <SettingsCard title={t('form.issuerTrust.title')} description={t('form.issuerTrust.description')}>
          <Stack spacing={3}>
            <Box>
              <FormControlLabel
                control={
                  <Switch
                    checked={effectiveEnforce}
                    disabled={trustAnchorsLoading || noTrustAnchors}
                    onChange={(e: ChangeEvent<HTMLInputElement>): void => setEnforceTrustedIssuer(e.target.checked)}
                  />
                }
                label={t('form.issuerTrust.enforce.label')}
              />
              <FormHelperText>
                {noTrustAnchors ? t('form.issuerTrust.enforce.noAnchorsHint') : t('form.issuerTrust.enforce.hint')}
              </FormHelperText>
            </Box>
            <FormControl fullWidth>
              <FormLabel htmlFor="vp-trusted-authorities">{t('form.issuerTrust.authorities.label')}</FormLabel>
              <Autocomplete<TrustAnchor, true, false, false>
                multiple
                fullWidth
                disabled={!enforceTrustedIssuer}
                loading={trustAnchorsLoading}
                options={trustAnchors}
                value={trustAnchors.filter((anchor: TrustAnchor): boolean => trustedAuthorities.includes(anchor.name))}
                isOptionEqualToValue={(option: TrustAnchor, val: TrustAnchor): boolean => option.name === val.name}
                getOptionLabel={(option: TrustAnchor): string => option.name}
                onChange={(_event: SyntheticEvent, newValue: TrustAnchor[]): void =>
                  setTrustedAuthorities(newValue.map((anchor: TrustAnchor): string => anchor.name))
                }
                renderOption={(props, option: TrustAnchor) => (
                  <li {...props} key={option.name}>
                    <ListItemText
                      primary={option.name}
                      secondary={t('form.issuerTrust.authorities.optionSecondary', {
                        subject: option.subject,
                        notAfter: option.not_after,
                      })}
                    />
                  </li>
                )}
                renderTags={(value: TrustAnchor[], getTagProps) =>
                  value.map((option: TrustAnchor, index: number) => {
                    const {key, ...tagProps} = getTagProps({index});

                    return <Chip key={key} label={option.name} {...tagProps} />;
                  })
                }
                renderInput={(params) => (
                  <TextField
                    {...params}
                    id="vp-trusted-authorities"
                    helperText={t('form.issuerTrust.authorities.hint')}
                  />
                )}
              />
            </FormControl>
          </Stack>
        </SettingsCard>
      </TabPanel>

      {dirty && (
        <UnsavedChangesBar
          message={t('form.unsavedChanges')}
          resetLabel={t('common:actions.reset')}
          saveLabel={submitLabel}
          savingLabel={t('common:status.saving')}
          isSaving={submitting}
          saveDisabled={!valid}
          onReset={handleReset}
          onSave={handleSubmit}
        />
      )}
    </Stack>
  );
}
