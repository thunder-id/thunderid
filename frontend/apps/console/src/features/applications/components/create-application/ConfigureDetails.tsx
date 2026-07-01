/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {zodResolver} from '@hookform/resolvers/zod';
import {useLogger} from '@thunderid/logger/react';
import {
  Box,
  Typography,
  Stack,
  TextField,
  Radio,
  RadioGroup,
  FormControlLabel,
  Alert,
  FormControl,
  FormLabel,
  Autocomplete,
  Chip,
  MenuItem,
  Select,
} from '@wso2/oxygen-ui';
import {Globe} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import {useForm, Controller, useWatch} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {z} from 'zod';
import {AuthenticatorTypes} from '../../../integrations/models/authenticators';
import {CUSTOM_WALLET_VENDOR, WALLET_VENDORS} from '../../constants/wallet-vendors';
import useApplicationCreate from '../../contexts/ApplicationCreate/useApplicationCreate';
import {ApplicationCreateFlowConfiguration} from '../../models/application-create-flow';
import type {PlatformApplicationTemplate, TechnologyApplicationTemplate} from '../../models/application-templates';
import getConfigurationTypeFromTemplate from '../../utils/getConfigurationTypeFromTemplate';

/**
 * Zod schema for validating URL inputs (hosting URLs and callback URLs).
 * Ensures URLs are properly formatted with http:// or https:// protocol.
 *
 * @internal
 */
const urlSchema: z.ZodString = z
  .string()
  .trim()
  .min(1, 'URL is required')
  .url('Please enter a valid URL')
  .refine((url) => url.startsWith('http://') || url.startsWith('https://'), {
    message: 'URL must start with http:// or https://',
  });

/**
 * Zod schema for validating deep links and universal links for mobile applications.
 * Accepts custom URL schemes (e.g., myapp://) or universal links (https://).
 *
 * @internal
 */
const deeplinkSchema: z.ZodString = z
  .string()
  .trim()
  .min(1, 'Deep link is required')
  .refine(
    (link) =>
      // Allow custom URL schemes (e.g., myapp://) or universal links (https://)
      /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/.+/.test(link),
    {
      message:
        'Please enter a valid deep link or universal link (e.g., myapp://callback or https://example.com/callback)',
    },
  );

/**
 * Zod schema for the configuration details form.
 * Validates hosting URLs, callback URLs, deep links, and user type selections
 * based on the configuration type required by the selected application template.
 *
 * @internal
 */
const formSchema = z
  .object({
    hostingUrl: z.string().optional(),
    callbackUrl: z.string().optional(),
    callbackMode: z.enum(['same', 'custom']),
    deeplink: z.string().optional(),
    relyingPartyId: z.string().optional(),
    relyingPartyName: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    // Validate hostingUrl for URL-based platforms
    if (data.hostingUrl !== undefined && data.hostingUrl !== '') {
      const result = urlSchema.safeParse(data.hostingUrl);
      if (!result.success) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.error.issues[0]?.message || 'Invalid URL',
          path: ['hostingUrl'],
        });
      }
    }

    // Validate callbackUrl when custom mode
    if (data.callbackMode === 'custom' && data.callbackUrl) {
      const result = urlSchema.safeParse(data.callbackUrl);
      if (!result.success) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.error.issues[0]?.message || 'Invalid callback URL',
          path: ['callbackUrl'],
        });
      }
    }

    // Validate deeplink for mobile platforms
    if (data.deeplink !== undefined && data.deeplink !== '') {
      const result = deeplinkSchema.safeParse(data.deeplink);
      if (!result.success) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.error.issues[0]?.message || 'Invalid deep link',
          path: ['deeplink'],
        });
      }
    }
  });

/**
 * Type definition for form data inferred from the Zod schema.
 *
 * @internal
 */
type FormData = z.infer<typeof formSchema>;

/**
 * User type structure for selection
 */
export interface UserType {
  id: string;
  name: string;
  ouId: string;
  allowSelfRegistration: boolean;
}

/**
 * Props for the {@link ConfigureDetails} component.
 *
 * @public
 */
export interface ConfigureDetailsProps {
  /**
   * The selected technology template (e.g., React, Next.js, Angular)
   */
  technology: TechnologyApplicationTemplate | null;

  /**
   * The selected platform template (e.g., Browser, Mobile, Backend)
   */
  platform: PlatformApplicationTemplate | null;

  /**
   * Callback function invoked when the hosting URL changes
   */
  onHostingUrlChange: (url: string) => void;

  /**
   * Callback function invoked when the callback URL (or deep link) changes
   */
  onCallbackUrlChange: (url: string) => void;

  /**
   * Callback invoked when the wallet client id changes (wallet template only).
   */
  onClientIdChange?: (clientId: string) => void;

  /**
   * Callback function to notify parent component whether this step is ready to proceed
   */
  onReadyChange: (isReady: boolean) => void;

  /**
   * Available user types for selection (optional)
   */
  userTypes?: UserType[];

  /**
   * Currently selected user type names (optional)
   */
  selectedUserTypes?: string[];

  /**
   * Callback function invoked when user type selection changes (optional)
   */
  onUserTypesChange?: (userTypes: string[]) => void;
}

/**
 * React component that renders the configuration details step in the
 * application creation onboarding flow.
 *
 * This component dynamically displays configuration options based on the selected
 * application template's requirements. It handles three configuration types:
 *
 * 1. **URL Configuration** (Browser/Server applications):
 *    - Hosting URL input for where the application is hosted
 *    - Callback URL configuration with options to use the same URL or a custom one
 *    - Real-time validation and synchronization of callback URL when "same as hosting" is selected
 *
 * 2. **Deep Link Configuration** (Mobile applications):
 *    - Deep link or universal link input for mobile app authentication redirects
 *    - Validation for custom URL schemes (e.g., myapp://) and universal links
 *
 * 3. **No Configuration** (Backend services):
 *    - Displays a message indicating no additional configuration is needed
 *
 * Additionally, if the selected template requires user type selection (indicated by an empty
 * allowedUserTypes array) and multiple user types are available, the component displays
 * a multi-select autocomplete for choosing applicable user types.
 *
 * The component uses React Hook Form with Zod validation to provide real-time form
 * validation and error messages. It notifies the parent component of readiness status
 * based on form validity and configuration requirements.
 *
 * @param props - The component props
 * @param props.technology - Selected technology template
 * @param props.platform - Selected platform template
 * @param props.onHostingUrlChange - Callback for hosting URL changes
 * @param props.onCallbackUrlChange - Callback for callback URL changes
 * @param props.onReadyChange - Callback for step readiness changes
 * @param props.userTypes - Available user types for selection
 * @param props.selectedUserTypes - Currently selected user type names
 * @param props.onUserTypesChange - Callback for user type selection changes
 *
 * @returns JSX element displaying the appropriate configuration interface
 *
 * @example
 * ```tsx
 * import ConfigureDetails from './ConfigureDetails';
 *
 * function OnboardingFlow() {
 *   const [hostingUrl, setHostingUrl] = useState('');
 *   const [callbackUrl, setCallbackUrl] = useState('');
 *   const [isReady, setIsReady] = useState(false);
 *
 *   return (
 *     <ConfigureDetails
 *       technology="react"
 *       platform="browser"
 *       onHostingUrlChange={setHostingUrl}
 *       onCallbackUrlChange={setCallbackUrl}
 *       onReadyChange={setIsReady}
 *       userTypes={[{id: '1', name: 'Customer'}, {id: '2', name: 'Employee'}]}
 *       selectedUserTypes={['Customer']}
 *       onUserTypesChange={(types) => console.log('Selected types:', types)}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigureDetails({
  onHostingUrlChange,
  onCallbackUrlChange,
  onClientIdChange = () => null,
  onReadyChange,
  userTypes = [],
  selectedUserTypes = [],
  onUserTypesChange = () => null,
}: ConfigureDetailsProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('ConfigureDetails');
  const {
    selectedTemplateConfig,
    integrations,
    selectedAuthFlow,
    relyingPartyId: contextRelyingPartyId,
    setRelyingPartyId,
    relyingPartyName: contextRelyingPartyName,
    setRelyingPartyName,
    appName,
  } = useApplicationCreate();
  const {
    control,
    formState: {errors, isValid},
    setValue,
    trigger,
  } = useForm<FormData>({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: {
      hostingUrl: '',
      callbackUrl: '',
      callbackMode: 'same',
      deeplink: '',
      relyingPartyId: '',
      relyingPartyName: '',
    },
  });

  const isPasskeyConfigEnabled: boolean = !selectedAuthFlow && (integrations[AuthenticatorTypes.PASSKEY] ?? false);

  const configurationType: ApplicationCreateFlowConfiguration =
    getConfigurationTypeFromTemplate(selectedTemplateConfig);

  const isWallet: boolean = selectedTemplateConfig?.id === 'wallet';
  const [walletVendor, setWalletVendor] = useState<string>(CUSTOM_WALLET_VENDOR);
  const [customClientId, setCustomClientId] = useState<string>('');
  const isWalletCustom: boolean = walletVendor === CUSTOM_WALLET_VENDOR;
  const selectedVendor = WALLET_VENDORS.find((v) => v.id === walletVendor);

  // Known wallets present a fixed client id + redirect URI, so selecting one
  // prefills both; "Custom" lets the admin enter them for any other wallet.
  const applyVendor = (vendorId: string): void => {
    setWalletVendor(vendorId);
    if (vendorId === CUSTOM_WALLET_VENDOR) {
      onClientIdChange(customClientId.trim());
      setValue('deeplink', '', {shouldValidate: true});
      return;
    }
    const vendor = WALLET_VENDORS.find((v) => v.id === vendorId);
    onClientIdChange(vendor?.clientId ?? '');
    setValue('deeplink', vendor?.redirectUri ?? '', {shouldValidate: true});
  };

  const applyCustomClientId = (value: string): void => {
    setCustomClientId(value);
    if (walletVendor === CUSTOM_WALLET_VENDOR) {
      onClientIdChange(value.trim());
    }
  };

  const hostingUrl: string = useWatch({control, name: 'hostingUrl'}) ?? '';
  const callbackUrl: string = useWatch({control, name: 'callbackUrl'}) ?? '';
  const callbackMode: 'same' | 'custom' = useWatch({control, name: 'callbackMode'}) ?? 'same';
  const deeplink: string = useWatch({control, name: 'deeplink'}) ?? '';
  const relyingPartyId: string = useWatch({control, name: 'relyingPartyId'}) ?? '';
  const relyingPartyName: string = useWatch({control, name: 'relyingPartyName'}) ?? '';
  const defaultHostDisplay: string = hostingUrl;

  /**
   * Sync callback URL with hosting URL when checkbox is checked.
   */
  useEffect((): void => {
    const syncCallbackUrl = async (): Promise<void> => {
      if (callbackMode === 'same') {
        setValue('callbackUrl', hostingUrl);
        onCallbackUrlChange(hostingUrl);

        try {
          await trigger('callbackUrl');
        } catch (error) {
          logger.error('Failed to trigger callback URL validation', {error});
        }
      }
    };

    syncCallbackUrl().catch((): void => {
      // optional: swallow/handle error
    });
  }, [callbackMode, hostingUrl, setValue, onCallbackUrlChange, trigger, logger]);

  /**
   * Initialize relying party fields with defaults if empty.
   */
  useEffect(() => {
    if (isPasskeyConfigEnabled) {
      if (!relyingPartyId && !contextRelyingPartyId) {
        // Default to hostname from window location (or hostingUrl if valid domain?)
        // Better to use window.location.hostname as a sensible default for local dev
        // or extract domain from hostingUrl if possible.
        // For now using window.location.hostname to match previous behavior
        setValue('relyingPartyId', window.location.hostname);
      }
      if (!relyingPartyName && !contextRelyingPartyName) {
        setValue('relyingPartyName', appName);
      }
    }
  }, [
    isPasskeyConfigEnabled,
    relyingPartyId,
    relyingPartyName,
    contextRelyingPartyId,
    contextRelyingPartyName,
    appName,
    setValue,
  ]);

  /**
   * Sync relying party fields with context.
   */
  useEffect(() => {
    setRelyingPartyId(relyingPartyId);
  }, [relyingPartyId, setRelyingPartyId]);

  useEffect(() => {
    setRelyingPartyName(relyingPartyName);
  }, [relyingPartyName, setRelyingPartyName]);

  /**
   * Notify parent of hosting URL changes.
   */
  useEffect((): void => {
    onHostingUrlChange(hostingUrl);
  }, [hostingUrl, onHostingUrlChange]);

  /**
   * Notify parent of callback URL changes (when not using same as hosting).
   */
  useEffect((): void => {
    if (callbackMode === 'custom') {
      onCallbackUrlChange(callbackUrl);
    }
  }, [callbackUrl, callbackMode, onCallbackUrlChange]);

  /**
   * Notify parent of deep link changes for mobile platforms.
   */
  useEffect((): void => {
    if (configurationType === ApplicationCreateFlowConfiguration.DEEPLINK) {
      onCallbackUrlChange(deeplink);
    }
  }, [deeplink, configurationType, onCallbackUrlChange]);

  /**
   * Determine if step is ready based on validity and configuration type.
   */
  useEffect((): void => {
    // If Passkey is enabled, we MUST have valid relying party info
    if (isPasskeyConfigEnabled) {
      if (!relyingPartyId || !relyingPartyName) {
        onReadyChange(false);
        return;
      }
    }

    if (configurationType === ApplicationCreateFlowConfiguration.NONE) {
      // Even if no base config needed, if Passkey is enabled we need those fields valid
      // The Passkey check block above handles returning false if invalid.
      // If we are here, it means either Passkey is disabled OR Passkey fields are valid.
      onReadyChange(true);
      return;
    }

    // For URL-based config, need valid hosting URL
    if (configurationType === ApplicationCreateFlowConfiguration.URL) {
      const hasValidHostingUrl: boolean = !!hostingUrl && !errors.hostingUrl;
      const hasValidCallbackUrl: boolean = callbackMode === 'same' || (!!callbackUrl && !errors.callbackUrl);
      onReadyChange(!!hasValidHostingUrl && !!hasValidCallbackUrl);
      return;
    }

    // For deeplink config, need valid deeplink
    if (configurationType === ApplicationCreateFlowConfiguration.DEEPLINK) {
      onReadyChange(!!deeplink && !errors.deeplink);
      return;
    }

    onReadyChange(isValid);
  }, [
    isValid,
    configurationType,
    hostingUrl,
    callbackUrl,
    callbackMode,
    deeplink,
    relyingPartyId,
    relyingPartyName,
    isPasskeyConfigEnabled,
    errors,
    onReadyChange,
    selectedTemplateConfig,
  ]);

  // For platforms that don't require configuration AND no passkey configuration needed
  if (configurationType === ApplicationCreateFlowConfiguration.NONE && !isPasskeyConfigEnabled) {
    return (
      <Stack spacing={3}>
        <Box sx={{textAlign: 'center', py: 4}}>
          <Globe size={48} style={{color: 'var(--oxygen-palette-text-secondary)', marginBottom: '16px'}} />
          <Typography variant="h6" gutterBottom>
            {t('applications:onboarding.configure.details.noConfigRequired.title')}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t('applications:onboarding.configure.details.noConfigRequired.description')}
          </Typography>
        </Box>
      </Stack>
    );
  }

  return (
    <Stack spacing={3} data-testid="application-configure-details">
      <Stack direction="column" spacing={1}>
        <Typography variant="h1" gutterBottom>
          {t('applications:onboarding.configure.details.title')}
        </Typography>
        {configurationType !== ApplicationCreateFlowConfiguration.NONE && (
          <Typography variant="subtitle1" gutterBottom>
            {configurationType === ApplicationCreateFlowConfiguration.DEEPLINK
              ? t('applications:onboarding.configure.details.mobile.description')
              : t('applications:onboarding.configure.details.description')}
          </Typography>
        )}
      </Stack>

      {/* User Type Selection - shown when template requires it and user types are available */}
      {userTypes &&
        userTypes.length > 0 &&
        selectedTemplateConfig?.defaults?.allowedUserTypes !== undefined &&
        Array.isArray(selectedTemplateConfig.defaults?.allowedUserTypes) &&
        selectedTemplateConfig.defaults?.allowedUserTypes.length === 0 && (
          <FormControl fullWidth>
            <FormLabel htmlFor="user-types-select">
              {t('applications:onboarding.configure.details.userTypes.label')}
            </FormLabel>
            <Autocomplete
              multiple
              id="user-types-select"
              options={userTypes.map((ut) => ut.name)}
              value={selectedUserTypes}
              onChange={(_event, newValue) => {
                if (onUserTypesChange) {
                  onUserTypesChange(newValue);
                }
              }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  placeholder={t('applications:onboarding.configure.details.userTypes.placeholder')}
                  helperText={t('applications:onboarding.configure.details.userTypes.helperText')}
                />
              )}
              renderTags={(value: string[], getTagProps) =>
                value.map((option: string, index: number) => (
                  <Chip {...getTagProps({index})} key={option} label={option} />
                ))
              }
            />
          </FormControl>
        )}

      {/* Mobile / wallet platform - Deep link / Universal link configuration */}
      {configurationType === ApplicationCreateFlowConfiguration.DEEPLINK && (
        <>
          {/* Mobile (non-wallet): the admin enters the app's deep link directly. */}
          {!isWallet && (
            <>
              <FormControl fullWidth required>
                <FormLabel htmlFor="deeplink-input">
                  {t('applications:onboarding.configure.details.deeplink.label')}
                </FormLabel>
                <Controller
                  name="deeplink"
                  control={control}
                  render={({field}) => (
                    <TextField
                      {...field}
                      fullWidth
                      id="deeplink-input"
                      placeholder={t('applications:onboarding.configure.details.deeplink.placeholder')}
                      error={!!errors.deeplink}
                      helperText={
                        errors.deeplink?.message ?? t('applications:onboarding.configure.details.deeplink.helperText')
                      }
                    />
                  )}
                />
              </FormControl>
              <Alert severity="info">{t('applications:onboarding.configure.details.mobile.info')}</Alert>
            </>
          )}

          {/* Wallet: pick a vendor first. Known wallets prefill client id + redirect (read-only); Custom asks for both. */}
          {isWallet && (
            <>
              <FormControl fullWidth>
                <FormLabel htmlFor="wallet-vendor-select">
                  {t('applications:onboarding.configure.details.wallet.vendor.label')}
                </FormLabel>
                <Select
                  id="wallet-vendor-select"
                  value={walletVendor}
                  onChange={(e): void => applyVendor(e.target.value)}
                >
                  {WALLET_VENDORS.map((vendor) => (
                    <MenuItem key={vendor.id} value={vendor.id}>
                      {vendor.id === CUSTOM_WALLET_VENDOR
                        ? t('applications:onboarding.configure.details.wallet.vendor.custom')
                        : vendor.label}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <FormControl fullWidth required>
                <FormLabel htmlFor="wallet-client-id-input">
                  {t('applications:onboarding.configure.details.wallet.clientId.label')}
                </FormLabel>
                <TextField
                  fullWidth
                  id="wallet-client-id-input"
                  value={isWalletCustom ? customClientId : (selectedVendor?.clientId ?? '')}
                  disabled={!isWalletCustom}
                  placeholder={t('applications:onboarding.configure.details.wallet.clientId.placeholder')}
                  helperText={
                    isWalletCustom
                      ? t('applications:onboarding.configure.details.wallet.clientId.helperText')
                      : t('applications:onboarding.configure.details.wallet.prefilled.helperText')
                  }
                  onChange={(e): void => applyCustomClientId(e.target.value)}
                />
              </FormControl>

              <FormControl fullWidth required>
                <FormLabel htmlFor="wallet-deeplink-input">
                  {t('applications:onboarding.configure.details.deeplink.label')}
                </FormLabel>
                {isWalletCustom ? (
                  <Controller
                    name="deeplink"
                    control={control}
                    render={({field}) => (
                      <TextField
                        {...field}
                        fullWidth
                        id="wallet-deeplink-input"
                        placeholder={t('applications:onboarding.configure.details.deeplink.placeholder')}
                        error={!!errors.deeplink}
                        helperText={
                          errors.deeplink?.message ?? t('applications:onboarding.configure.details.deeplink.helperText')
                        }
                      />
                    )}
                  />
                ) : (
                  <TextField
                    fullWidth
                    id="wallet-deeplink-input"
                    value={selectedVendor?.redirectUri ?? ''}
                    disabled
                    helperText={t('applications:onboarding.configure.details.wallet.prefilled.helperText')}
                  />
                )}
              </FormControl>
            </>
          )}
        </>
      )}

      {/* Browser/Server platform - URL configuration */}
      {configurationType === ApplicationCreateFlowConfiguration.URL && (
        <>
          {/* Hosting URL */}
          <FormControl fullWidth required>
            <FormLabel htmlFor="hosting-url-input">
              {t('applications:onboarding.configure.details.hostingUrl.label')}
            </FormLabel>
            <Controller
              name="hostingUrl"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="hosting-url-input"
                  placeholder={t('applications:onboarding.configure.details.hostingUrl.placeholder')}
                  error={!!errors.hostingUrl}
                  helperText={
                    errors.hostingUrl?.message ?? t('applications:onboarding.configure.details.hostingUrl.helperText')
                  }
                />
              )}
            />
          </FormControl>

          {/* After Sign-in URL (Callback URL) */}
          <Stack spacing={2}>
            <FormControl component="fieldset">
              <FormLabel id="callback-url-label">
                {t('applications:onboarding.configure.details.callbackUrl.label')}
              </FormLabel>
              <Controller
                name="callbackMode"
                control={control}
                render={({field}) => (
                  <RadioGroup {...field} aria-labelledby="callback-url-label">
                    <FormControlLabel
                      value="same"
                      control={<Radio />}
                      label={
                        <Stack direction="row" alignItems="center" spacing={1}>
                          <Typography variant="body1">
                            {t('applications:onboarding.configure.details.callbackMode.same')}
                          </Typography>
                          {defaultHostDisplay && (
                            <Typography variant="body2" color="text.secondary">
                              ({defaultHostDisplay})
                            </Typography>
                          )}
                        </Stack>
                      }
                    />
                    <FormControlLabel
                      value="custom"
                      control={<Radio />}
                      label={t('applications:onboarding.configure.details.callbackMode.custom')}
                    />
                  </RadioGroup>
                )}
              />
            </FormControl>

            {callbackMode === 'custom' && (
              <FormControl fullWidth>
                <FormLabel htmlFor="callback-url-input" id="custom-callback-url-label">
                  {t('applications:onboarding.configure.details.callbackUrl.label')}
                </FormLabel>
                <Controller
                  name="callbackUrl"
                  control={control}
                  render={({field}) => (
                    <TextField
                      {...field}
                      fullWidth
                      id="callback-url-input"
                      placeholder={t('applications:onboarding.configure.details.callbackUrl.placeholder')}
                      error={!!errors.callbackUrl}
                      helperText={
                        errors.callbackUrl?.message ??
                        t('applications:onboarding.configure.details.callbackUrl.helperText')
                      }
                    />
                  )}
                />
              </FormControl>
            )}

            <Alert severity="info">{t('applications:onboarding.configure.details.callbackUrl.info')}</Alert>
          </Stack>
        </>
      )}

      {/* Passkey Relying Party Configuration */}
      {isPasskeyConfigEnabled && (
        <Stack spacing={2}>
          <Typography variant="subtitle1" gutterBottom>
            {t('applications:onboarding.configure.details.passkey.title') || 'Passkey Settings'}
          </Typography>
          <FormControl fullWidth required>
            <FormLabel htmlFor="relying-party-id-input">
              {t('applications:onboarding.configure.details.relyingPartyId.label') || 'Relying Party ID'}
            </FormLabel>
            <Controller
              name="relyingPartyId"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="relying-party-id-input"
                  placeholder={
                    t('applications:onboarding.configure.details.relyingPartyId.placeholder') || 'e.g., example.com'
                  }
                  error={!!errors.relyingPartyId}
                  helperText={
                    errors.relyingPartyId?.message ??
                    t('applications:onboarding.configure.details.relyingPartyId.helperText')
                  }
                />
              )}
            />
          </FormControl>
          <FormControl fullWidth required>
            <FormLabel htmlFor="relying-party-name-input">
              {t('applications:onboarding.configure.details.relyingPartyName.label') || 'Relying Party Name'}
            </FormLabel>
            <Controller
              name="relyingPartyName"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="relying-party-name-input"
                  placeholder={
                    t('applications:onboarding.configure.details.relyingPartyName.placeholder') || 'e.g., My App'
                  }
                  error={!!errors.relyingPartyName}
                  helperText={
                    errors.relyingPartyName?.message ??
                    t('applications:onboarding.configure.details.relyingPartyName.helperText')
                  }
                />
              )}
            />
          </FormControl>
        </Stack>
      )}
    </Stack>
  );
}
