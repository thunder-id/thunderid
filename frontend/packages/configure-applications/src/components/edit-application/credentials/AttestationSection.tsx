// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  FormControl,
  FormControlLabel,
  FormLabel,
  IconButton,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import DevModeConfirmDialog from './DevModeConfirmDialog';
import type {AttestationConfig} from '@thunderid/configure-applications';

/**
 * The attestation platform an application is configured for. An application configures exactly one
 * platform; 'none' means attestation is disabled.
 */
type AttestationPlatform = 'none' | 'android' | 'apple';

/**
 * Props for the {@link AttestationSection} component.
 */
interface AttestationSectionProps {
  /**
   * The current attestation config.
   * null or undefined means no attestation is configured.
   */
  attestation?: AttestationConfig | null;
  /**
   * Called when the user changes any attestation field.
   * Passes null when no attestation fields are set.
   */
  onAttestationChange: (attestation: AttestationConfig | null) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
  /**
   * Called whenever the section's validation state changes, so the parent can block Save while an
   * incomplete apple config exists. true means the section currently has a validation error.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Derives the configured platform from an attestation config.
 */
function platformOf(attestation?: AttestationConfig | null): AttestationPlatform {
  if (attestation?.apple) {
    return 'apple';
  }
  if (attestation?.android) {
    return 'android';
  }
  return 'none';
}

/**
 * Section component for configuring platform attestation for mobile clients.
 *
 * A mobile application that configures attestation may initiate an authentication flow directly by
 * presenting an attestation token, which the server verifies against the registered identity —
 * Google Play Integrity for Android (package name + signing certificate digests, with write-only
 * service account credentials) or Apple App Attest for iOS (Team ID + Bundle ID). An application
 * configures exactly one platform, so the platform selector switches between the two field sets.
 * Apple's Team ID and Bundle ID are required together: a config with only one of the two is never
 * emitted to the parent, since the backend cannot verify an incomplete identity.
 *
 * Dev Mode is independent of the platform fields: when enabled, the application may initiate flows
 * without presenting an attestation at all, regardless of whether a platform is configured. Its toggle
 * lives in the card header, to the right of the title, matching the enable toggle placement used
 * elsewhere in the edit page (e.g. flow sections). It is disabled by default and intended only for
 * testing or trying out sample/development mobile clients; a warning banner is shown while it is on.
 * Turning it on requires confirmation via {@link DevModeConfirmDialog}, since it skips attestation
 * verification; turning it off applies immediately.
 *
 * @param props - Component props
 * @returns Attestation configuration UI within a SettingsCard
 */
export default function AttestationSection({
  attestation = undefined,
  onAttestationChange,
  disabled = false,
  onValidationChange = undefined,
}: AttestationSectionProps) {
  const {t} = useTranslation();

  const android = attestation?.android;
  const apple = attestation?.apple;
  const propPlatform = platformOf(attestation);

  // The fields are backed by local state (seeded from props) so editing is never gated on the
  // parent's config round-trip. Credentials are write-only and never seeded from props.
  const [platform, setPlatform] = useState<AttestationPlatform>(propPlatform);
  const [packageName, setPackageName] = useState<string>(android?.packageName ?? '');
  const [digests, setDigests] = useState<string[]>(android?.certificateSha256Digests ?? []);
  // An empty list renders as a bare "Add Digest" button with no field to type into, which reads
  // as broken rather than "nothing added yet". Show one empty row by default instead; typing into
  // it (or removing it) operates on the real (currently empty) `digests` array via the index above.
  const displayDigests = digests.length > 0 ? digests : [''];
  const [credentials, setCredentials] = useState<string>('');
  const [teamId, setTeamId] = useState<string>(apple?.teamId ?? '');
  const [bundleId, setBundleId] = useState<string>(apple?.bundleId ?? '');
  const [devMode, setDevMode] = useState<boolean>(attestation?.devMode ?? false);
  const [isDevModeConfirmOpen, setIsDevModeConfirmOpen] = useState<boolean>(false);

  // Canonical identity of the incoming config (platform + non-secret fields). The effect below
  // resyncs local state when the attestation prop is replaced externally — e.g. the application
  // reloads, or the config is cleared — while ignoring the echo of this component's own emissions
  // (tracked via the ref). Credentials are write-only and never part of the identity.
  const computeIdentity = (
    p: AttestationPlatform,
    pkg: string,
    digs: string[],
    team: string,
    bundle: string,
    dev: boolean,
  ) => JSON.stringify({platform: p, packageName: pkg, digests: digs, teamId: team, bundleId: bundle, devMode: dev});

  const identityKey = computeIdentity(
    propPlatform,
    android?.packageName ?? '',
    android?.certificateSha256Digests ?? [],
    apple?.teamId ?? '',
    apple?.bundleId ?? '',
    attestation?.devMode ?? false,
  );
  const lastSyncedKeyRef = useRef<string>(identityKey);

  useEffect(() => {
    if (identityKey === lastSyncedKeyRef.current) {
      return;
    }
    lastSyncedKeyRef.current = identityKey;
    setPlatform(propPlatform);
    setPackageName(android?.packageName ?? '');
    setDigests(android?.certificateSha256Digests ?? []);
    setTeamId(apple?.teamId ?? '');
    setBundleId(apple?.bundleId ?? '');
    setDevMode(attestation?.devMode ?? false);
    // Credentials are write-only; an external config change resets the editable field to blank.
    setCredentials('');
    // identityKey is the canonical trigger; the config values are read for what it encodes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [identityKey]);

  // Apple's Team ID and Bundle ID are required together (the backend cannot verify a partial
  // identity). While exactly one of the two is populated, the config is incomplete: emit() below
  // skips the update entirely rather than propagating a partial (invalid) apple config, and this
  // flag drives an inline validation hint on whichever field is still empty.
  const appleIncomplete = platform === 'apple' && (teamId.trim() === '') !== (bundleId.trim() === '');

  useEffect(() => {
    onValidationChange?.(appleIncomplete);
  }, [appleIncomplete, onValidationChange]);

  const emit = (
    nextPlatform: AttestationPlatform,
    pkg: string,
    digs: string[],
    creds: string,
    team: string,
    bundle: string,
    dev: boolean,
  ) => {
    const cleanedDigests = digs.map((d) => d.trim()).filter((d) => d !== '');
    const cleanedPackageName = pkg.trim();
    const cleanedTeamId = team.trim();
    const cleanedBundleId = bundle.trim();

    let config: AttestationConfig | null = null;
    let identityPlatform: AttestationPlatform = 'none';

    if (nextPlatform === 'android') {
      const androidConfig: NonNullable<AttestationConfig['android']> = {};
      if (cleanedPackageName !== '') {
        androidConfig.packageName = cleanedPackageName;
      }
      if (cleanedDigests.length > 0) {
        androidConfig.certificateSha256Digests = cleanedDigests;
      }
      if (creds !== '') {
        androidConfig.serviceAccountCredentials = creds;
      }
      if (Object.keys(androidConfig).length > 0) {
        config = {android: androidConfig};
        identityPlatform = 'android';
      }
    } else if (nextPlatform === 'apple') {
      if (cleanedTeamId !== '' && cleanedBundleId !== '') {
        config = {apple: {teamId: cleanedTeamId, bundleId: cleanedBundleId}};
        identityPlatform = 'apple';
      } else if (cleanedTeamId !== '' || cleanedBundleId !== '') {
        // Exactly one of the two is set: an incomplete apple config. Never emit it as the platform
        // config — that would persist an identity the backend can never verify; appleIncomplete
        // (above) surfaces a validation hint instead. If this call isn't actually changing dev
        // mode, skip the update entirely so the parent keeps its last valid value while the user
        // finishes entering the other field. If it is a dev-mode transition, fall through with the
        // last valid platform preserved (from the attestation prop) so the transition still reaches
        // the parent instead of being silently dropped.
        if (dev === devMode) {
          return;
        }
        config = apple ? {apple} : android ? {android} : null;
        identityPlatform = propPlatform;
      }
      // Both empty falls through with config left at null, clearing any stored apple config.
    }

    // Dev mode is independent of the platform fields, so it may keep a config alive (or create one)
    // even when no platform is configured.
    if (dev) {
      config = {...config, devMode: true};
    }

    // Record the identity being emitted so the resync effect ignores the resulting prop echo and
    // preserves the user's in-progress edits. Derived from the emitted config itself, rather than
    // the raw typed fields, so a preserved (not freshly typed) platform config above stays
    // consistent with what was actually emitted.
    const emittedAndroid = identityPlatform === 'android' ? config?.android : undefined;
    const emittedApple = identityPlatform === 'apple' ? config?.apple : undefined;
    lastSyncedKeyRef.current = computeIdentity(
      identityPlatform,
      emittedAndroid?.packageName ?? '',
      emittedAndroid?.certificateSha256Digests ?? [],
      emittedApple?.teamId ?? '',
      emittedApple?.bundleId ?? '',
      dev,
    );
    onAttestationChange(config);
  };

  const platformOptions: {value: AttestationPlatform; label: string}[] = [
    {value: 'none', label: t('applications:edit.advanced.attestation.platform.none', 'None')},
    {
      value: 'android',
      label: t('applications:edit.advanced.attestation.platform.android', 'Android (Play Integrity)'),
    },
    {value: 'apple', label: t('applications:edit.advanced.attestation.platform.apple', 'iOS (App Attest)')},
  ];

  const handlePlatformChange = (next: AttestationPlatform) => {
    setPlatform(next);
    emit(next, packageName, digests, credentials, teamId, bundleId, devMode);
  };

  const handlePackageNameChange = (value: string) => {
    setPackageName(value);
    emit(platform, value, digests, credentials, teamId, bundleId, devMode);
  };

  const handleCredentialsChange = (value: string) => {
    setCredentials(value);
    emit(platform, packageName, digests, value, teamId, bundleId, devMode);
  };

  const handleTeamIdChange = (value: string) => {
    setTeamId(value);
    emit(platform, packageName, digests, credentials, value, bundleId, devMode);
  };

  const handleBundleIdChange = (value: string) => {
    setBundleId(value);
    emit(platform, packageName, digests, credentials, teamId, value, devMode);
  };

  const commitDigests = (nextDigests: string[]) => {
    setDigests(nextDigests);
    emit(platform, packageName, nextDigests, credentials, teamId, bundleId, devMode);
  };

  const applyDevModeChange = (value: boolean) => {
    setDevMode(value);
    emit(platform, packageName, digests, credentials, teamId, bundleId, value);
  };

  // Turning dev mode on skips attestation verification entirely, so it requires confirmation.
  // Turning it off is always safe and applies immediately.
  const handleDevModeToggle = (value: boolean) => {
    if (value) {
      setIsDevModeConfirmOpen(true);
      return;
    }
    applyDevModeChange(false);
  };

  const handleDevModeConfirm = () => {
    setIsDevModeConfirmOpen(false);
    applyDevModeChange(true);
  };

  const handleDevModeCancel = () => {
    setIsDevModeConfirmOpen(false);
  };

  // Appends relative to what's on screen (displayDigests), not the possibly-empty backing
  // `digests` array — otherwise the first click while the list is empty turns [] into [''],
  // which renders identically to the placeholder row already shown and looks like nothing happened.
  const handleAddDigest = () => {
    setDigests([...displayDigests, '']);
  };

  const handleDigestChange = (index: number, value: string) => {
    setDigests((prev) => {
      // Typing into the placeholder row shown for an empty list must create the first real
      // entry rather than mapping over (and no-op'ing on) an empty array.
      const next = prev.length > 0 ? [...prev] : [''];
      next[index] = value;
      return next;
    });
  };

  const handleRemoveDigest = (index: number) => {
    commitDigests(digests.filter((_, i) => i !== index));
  };

  const devModeLabel = t('applications:edit.advanced.attestation.labels.devMode', 'Dev Mode');
  const devModeSwitch = (
    <FormControlLabel
      labelPlacement="start"
      control={
        <Switch
          checked={devMode}
          disabled={disabled}
          onChange={(e) => handleDevModeToggle(e.target.checked)}
          inputProps={{'aria-label': devModeLabel}}
        />
      }
      label={<Typography variant="subtitle2">{devModeLabel}</Typography>}
      sx={{ml: 0}}
    />
  );

  return (
    <SettingsCard
      title={t('applications:edit.advanced.labels.attestation', 'Platform Attestation')}
      description={t(
        'applications:edit.advanced.attestation.intro',
        'Verify the binary identity of a mobile client when it initiates a flow directly. Choose the platform the ' +
          'application is built for.',
      )}
      headerAction={
        <Tooltip
          title={t(
            'applications:edit.advanced.attestation.hint.devMode',
            'Skips attestation verification for this application. Enable only for testing or trying out ' +
              'sample/development mobile clients; leave disabled otherwise.',
          )}
        >
          <span>{devModeSwitch}</span>
        </Tooltip>
      }
    >
      <Stack spacing={2}>
        {devMode && (
          <Alert severity="warning">
            {t(
              'applications:edit.advanced.attestation.warning.devMode',
              'Dev mode is enabled. Attestation verification is skipped for this application. Use this only ' +
                'for testing, do not enable it in production.',
            )}
          </Alert>
        )}

        <FormControl fullWidth>
          <FormLabel htmlFor="attestation-platform">
            {t('applications:edit.advanced.attestation.labels.platform', 'Platform')}
          </FormLabel>
          <Autocomplete
            id="attestation-platform"
            value={platformOptions.find((opt) => opt.value === platform) ?? platformOptions[0]}
            onChange={(_, newValue) => handlePlatformChange(newValue?.value ?? 'none')}
            options={platformOptions}
            getOptionLabel={(option) => option.label}
            isOptionEqualToValue={(option, value) => option.value === value.value}
            renderInput={(params) => <TextField {...params} fullWidth />}
            disableClearable
            disabled={disabled}
          />
        </FormControl>

        {platform === 'android' && (
          <>
            <FormControl fullWidth>
              <FormLabel htmlFor="attestation-package-name">
                {t('applications:edit.advanced.attestation.labels.packageName', 'Package Name')}
              </FormLabel>
              <TextField
                id="attestation-package-name"
                fullWidth
                value={packageName}
                onChange={(e) => handlePackageNameChange(e.target.value)}
                placeholder={t('applications:edit.advanced.attestation.placeholder.packageName', 'com.example.myapp')}
                helperText={t(
                  'applications:edit.advanced.attestation.hint.packageName',
                  'The Android application package name that must match the attested app.',
                )}
                disabled={disabled}
              />
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor="attestation-digests-section">
                {t(
                  'applications:edit.advanced.attestation.labels.certificateSha256Digests',
                  'Signing Certificate SHA-256 Digests',
                )}
              </FormLabel>
              <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
                {t(
                  'applications:edit.advanced.attestation.hint.certificateSha256Digests',
                  'Allowed signing certificate digests, in the URL-safe base64 form reported by Play Integrity. ' +
                    'The attested app must match one of these.',
                )}
              </Typography>
              <Stack spacing={2} id="attestation-digests-section">
                {displayDigests.map((digest, index) => (
                  // IMPORTANT: Do not remove the suppression since it affects functionality.
                  // eslint-disable-next-line react/no-array-index-key
                  <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
                    <FormControl fullWidth sx={{flex: 1}}>
                      <TextField
                        fullWidth
                        id={`attestation-digest-${index}-input`}
                        // Each repeated field needs a unique accessible name; the shared FormLabel
                        // points at the surrounding Stack, not the individual inputs.
                        aria-label={`${t(
                          'applications:edit.advanced.attestation.labels.certificateSha256Digests',
                          'Signing Certificate SHA-256 Digests',
                        )} ${index + 1}`}
                        value={digest}
                        onChange={(e) => handleDigestChange(index, e.target.value)}
                        onBlur={() => commitDigests(digests)}
                        placeholder={t(
                          'applications:edit.advanced.attestation.placeholder.certificateSha256Digest',
                          'URL-safe base64 SHA-256 digest',
                        )}
                        disabled={disabled}
                      />
                    </FormControl>
                    <Tooltip title={t('common:actions.delete')}>
                      <IconButton
                        onClick={() => handleRemoveDigest(index)}
                        color="error"
                        sx={{mt: 1}}
                        disabled={disabled}
                      >
                        <Trash size={20} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                ))}
                <Box>
                  <Button
                    variant="text"
                    color="primary"
                    startIcon={<Plus />}
                    onClick={handleAddDigest}
                    size="small"
                    disabled={disabled}
                  >
                    {t('applications:edit.advanced.attestation.addDigest', 'Add Digest')}
                  </Button>
                </Box>
              </Stack>
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor="attestation-service-account">
                {t(
                  'applications:edit.advanced.attestation.labels.serviceAccountCredentials',
                  'Service Account Credentials',
                )}
              </FormLabel>
              <TextField
                id="attestation-service-account"
                fullWidth
                multiline
                rows={4}
                value={credentials}
                onChange={(e) => handleCredentialsChange(e.target.value)}
                placeholder={t(
                  'applications:edit.advanced.attestation.placeholder.serviceAccountCredentials',
                  'Paste the Google Cloud service account JSON',
                )}
                helperText={t(
                  'applications:edit.advanced.attestation.hint.serviceAccountCredentials',
                  'Write-only. Used to call the Play Integrity API. Leave blank to keep the existing credentials.',
                )}
                disabled={disabled}
              />
            </FormControl>
          </>
        )}

        {platform === 'apple' && (
          <>
            <FormControl fullWidth>
              <FormLabel htmlFor="attestation-team-id">
                {t('applications:edit.advanced.attestation.labels.teamId', 'Team ID')}
              </FormLabel>
              <TextField
                id="attestation-team-id"
                fullWidth
                value={teamId}
                onChange={(e) => handleTeamIdChange(e.target.value)}
                placeholder={t('applications:edit.advanced.attestation.placeholder.teamId', 'ABCDE12345')}
                error={appleIncomplete && teamId.trim() === ''}
                helperText={
                  appleIncomplete && teamId.trim() === ''
                    ? t(
                        'applications:edit.advanced.attestation.error.appleIncomplete',
                        'Both Team ID and Bundle ID are required together.',
                      )
                    : t('applications:edit.advanced.attestation.hint.teamId', 'The Apple Developer Team ID.')
                }
                disabled={disabled}
              />
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor="attestation-bundle-id">
                {t('applications:edit.advanced.attestation.labels.bundleId', 'Bundle ID')}
              </FormLabel>
              <TextField
                id="attestation-bundle-id"
                fullWidth
                value={bundleId}
                onChange={(e) => handleBundleIdChange(e.target.value)}
                placeholder={t('applications:edit.advanced.attestation.placeholder.bundleId', 'com.example.myapp')}
                error={appleIncomplete && bundleId.trim() === ''}
                helperText={
                  appleIncomplete && bundleId.trim() === ''
                    ? t(
                        'applications:edit.advanced.attestation.error.appleIncomplete',
                        'Both Team ID and Bundle ID are required together.',
                      )
                    : t(
                        'applications:edit.advanced.attestation.hint.bundleId',
                        'The iOS bundle identifier that must match the attested app.',
                      )
                }
                disabled={disabled}
              />
            </FormControl>
          </>
        )}
      </Stack>

      <DevModeConfirmDialog
        open={isDevModeConfirmOpen}
        onClose={handleDevModeCancel}
        onConfirm={handleDevModeConfirm}
      />
    </SettingsCard>
  );
}
