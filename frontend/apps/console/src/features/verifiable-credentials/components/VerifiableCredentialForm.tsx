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
import ClaimsEditor from './ClaimsEditor';
import {claimRowsToRequest, credentialToClaimRows, type ClaimRow} from '../models/claims';
import type {CreateVerifiableCredentialRequest} from '../models/requests';
import type {VerifiableCredential} from '../models/vc';

export interface VerifiableCredentialFormProps {
  initial?: VerifiableCredential;
  /** Name, edited inline in the page header rather than in this form. */
  name: string;
  /** Description, edited inline in the page header rather than in this form. */
  description: string;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
  submitting: boolean;
  submitLabel: string;
  onSubmit: (data: CreateVerifiableCredentialRequest) => void;
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
 * Tabbed create/edit form for an OpenID4VCI credential configuration: a General
 * tab (identity, format, display) and a Claims tab (attribute -> display name).
 */
export default function VerifiableCredentialForm({
  initial = undefined,
  name,
  description,
  onNameChange,
  onDescriptionChange,
  submitting,
  submitLabel,
  onSubmit,
  onDelete = undefined,
}: VerifiableCredentialFormProps): JSX.Element {
  const {t} = useTranslation('verifiable-credentials');

  const {hasMultipleOUs, ouList} = useHasMultipleOUs();

  const [tab, setTab] = useState<number>(0);
  const [handle, setHandle] = useState<string>(initial?.handle ?? '');
  const [ouId, setOuId] = useState<string>(initial?.ouId ?? '');
  const [vct, setVct] = useState<string>(initial?.vct ?? '');
  const [format, setFormat] = useState<string>(initial?.format ?? 'dc+sd-jwt');
  const [locale, setLocale] = useState<string>(initial?.display?.locale ?? '');
  const [logoUri, setLogoUri] = useState<string>(initial?.display?.logoUri ?? '');
  const [claims, setClaims] = useState<ClaimRow[]>(credentialToClaimRows(initial));

  const configurationId = initial?.id;
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

  // In a single-OU deployment the sole OU is used implicitly; otherwise the picker drives ouId.
  const effectiveOuId = ouId !== '' ? ouId : !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';

  const valid = handle.trim() !== '' && vct.trim() !== '' && effectiveOuId !== '';

  const buildRequest = (): CreateVerifiableCredentialRequest => {
    const display =
      locale.trim() || logoUri.trim()
        ? {
            locale: locale.trim() || undefined,
            logoUri: logoUri.trim() || undefined,
          }
        : undefined;
    return {
      handle: handle.trim(),
      ouId: effectiveOuId,
      name: name.trim() || undefined,
      description: description.trim() || undefined,
      vct: vct.trim(),
      format: format.trim() || undefined,
      claims: claimRowsToRequest(claims),
      display,
    };
  };

  const snapshot = (req: CreateVerifiableCredentialRequest): string => JSON.stringify(req);
  const initialSnapshot = useMemo(() => {
    const loc = (initial?.display?.locale ?? '').trim();
    const logo = (initial?.display?.logoUri ?? '').trim();
    return snapshot({
      handle: (initial?.handle ?? '').trim(),
      ouId: initial?.ouId ?? '',
      name: (initial?.name ?? '').trim() || undefined,
      description: (initial?.description ?? '').trim() || undefined,
      vct: (initial?.vct ?? '').trim(),
      format: (initial?.format ?? '').trim() || undefined,
      claims: claimRowsToRequest(credentialToClaimRows(initial)),
      display: loc || logo ? {locale: loc || undefined, logoUri: logo || undefined} : undefined,
    });
  }, [initial]);
  const dirty = snapshot(buildRequest()) !== initialSnapshot;

  const handleReset = (): void => {
    setHandle(initial?.handle ?? '');
    setOuId(initial?.ouId ?? '');
    onNameChange(initial?.name ?? '');
    onDescriptionChange(initial?.description ?? '');
    setVct(initial?.vct ?? '');
    setFormat(initial?.format ?? 'dc+sd-jwt');
    setLocale(initial?.display?.locale ?? '');
    setLogoUri(initial?.display?.logoUri ?? '');
    setClaims(credentialToClaimRows(initial));
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

  return (
    <Stack spacing={3}>
      <Tabs value={tab} onChange={(_e: SyntheticEvent, v: number): void => setTab(v)} aria-label="credential template">
        <Tab label={t('form.tabs.general')} />
        <Tab label={t('form.tabs.protocolSettings')} />
        <Tab label={t('form.tabs.claims')} />
      </Tabs>

      <TabPanel value={tab} index={0}>
        <Stack spacing={3}>
          {configurationId && (
            <SettingsCard title={t('form.quickCopy.title')} description={t('form.quickCopy.description')}>
              <Stack spacing={3}>
                <FormControl fullWidth>
                  <FormLabel htmlFor="vc-id">{t('form.id.label')}</FormLabel>
                  <TextField
                    fullWidth
                    id="vc-id"
                    value={configurationId}
                    InputProps={{
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <Tooltip title={copiedField === 'id' ? t('common:actions.copied') : t('form.copyId')}>
                            <IconButton
                              aria-label={t('form.copyId')}
                              edge="end"
                              onClick={(): void => {
                                handleCopy(configurationId, 'id').catch(() => null);
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
                </FormControl>
                <FormControl fullWidth>
                  <FormLabel htmlFor="vc-handle">{t('form.handle.label')}</FormLabel>
                  <TextField
                    fullWidth
                    id="vc-handle"
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
                </FormControl>
              </Stack>
            </SettingsCard>
          )}

          <SettingsCard title={t('form.organizationUnit.title')} description={t('form.organizationUnit.description')}>
            <Stack spacing={3}>
              {!initial && hasMultipleOUs && (
                <FormControl fullWidth required>
                  <FormLabel>{t('form.organizationUnit.label')}</FormLabel>
                  <OrganizationUnitTreePicker
                    id="vc-ou-picker"
                    value={effectiveOuId}
                    onChange={setOuId}
                    maxHeight={320}
                    helperText={t('form.organizationUnit.pickerHint')}
                  />
                </FormControl>
              )}
              {initial && (
                <>
                  <FormControl fullWidth>
                    <FormLabel htmlFor="vc-ou-handle">{t('form.organizationUnit.handleLabel')}</FormLabel>
                    <TextField
                      id="vc-ou-handle"
                      fullWidth
                      size="small"
                      value={initial.ouHandle ?? '-'}
                      InputProps={{
                        readOnly: true,
                        endAdornment: initial.ouHandle ? (
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
                  </FormControl>
                  <FormControl fullWidth>
                    <FormLabel htmlFor="vc-ou-id">{t('form.organizationUnit.idLabel')}</FormLabel>
                    <TextField
                      id="vc-ou-id"
                      fullWidth
                      size="small"
                      value={initial.ouId}
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
                                  handleCopy(initial.ouId, 'ouId').catch(() => null);
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
                  </FormControl>
                </>
              )}
            </Stack>
          </SettingsCard>

          {initial?.id && onDelete && (
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
        <SettingsCard title={t('form.protocol.title')} description={t('form.protocol.description')}>
          <Stack spacing={3}>
            {text('vc-vct', t('form.vct.label'), vct, setVct, 'urn:eudi:pid:de:1', true, t('form.vct.hint'))}
            <FormControl fullWidth>
              <FormLabel htmlFor="vc-format">{t('form.format.label')}</FormLabel>
              <Select id="vc-format" value={format} onChange={(e): void => setFormat(e.target.value)}>
                <MenuItem value="dc+sd-jwt">{t('form.format.sdJwt')}</MenuItem>
              </Select>
              <FormHelperText>{t('form.format.hint')}</FormHelperText>
            </FormControl>
            {text(
              'vc-locale',
              t('form.display.locale'),
              locale,
              setLocale,
              'en-US',
              false,
              t('form.display.localeHint'),
            )}
            {text(
              'vc-logo',
              t('form.display.logo'),
              logoUri,
              setLogoUri,
              'https://example.com/logo.png',
              false,
              t('form.display.logoHint'),
            )}
          </Stack>
        </SettingsCard>
      </TabPanel>

      <TabPanel value={tab} index={2}>
        <ClaimsEditor claims={claims} onChange={setClaims} />
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
          onSave={(): void => onSubmit(buildRequest())}
        />
      )}
    </Stack>
  );
}
