// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import type {PropertyDefinition, ApiUserType} from '@thunderid/configure-user-types';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger';
import {useThunderID} from '@thunderid/react';
import {Stack} from '@wso2/oxygen-ui';
import {useState, useEffect, useMemo, useRef} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {z} from 'zod';
import ScopeSection from './ScopeSection';
import TokenUserAttributesSection from './TokenUserAttributesSection';
import TokenValidationSection from './TokenValidationSection';
import {isOAuthTokenMode} from '../../../utils/oauth2Rules';
import type {Application, AssertionConfig, OAuth2Config, ScopeClaims} from '@thunderid/configure-applications';

/**
 * Props for the {@link EditTokenSettings} component.
 */
interface EditTokenSettingsProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Pending, unsaved changes to the application as tracked by the edit page. Fields present here
   * take precedence over the stored ones, so successive edits to the same field accumulate instead
   * of each being applied on top of the stored value.
   */
  editedApp?: Partial<Application>;
  /**
   * OAuth2 configuration containing token settings (optional)
   */
  oauth2Config?: OAuth2Config;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  onValidationChange?: (hasErrors: boolean) => void;
  /**
   * Bumped by the parent on Save/Reset to reset the Token Validity form fields in place. Unlike
   * sibling sections, this doesn't remount the component — TokenValidationSection owns its own
   * local sub-tab selection (Access/ID/Refresh Token) that must survive a Reset.
   */
  sectionResetKey?: number;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
  /**
   * Whether to show the "User Info Endpoint" tab (OAuth mode only). Defaults to true;
   * agents don't expose a userinfo endpoint of their own, so they pass false.
   */
  showUserInfoTab?: boolean;
  /**
   * Whether the access token preview should include the RFC 8693 `act` (actor) claim.
   * Defaults to false (applications); agents pass true.
   */
  showActorClaim?: boolean;
  /**
   * Value shown for `act.sub` in the actor claim preview (the acting agent's ID).
   */
  actorSub?: string;
  /**
   * Name of the tab where the OAuth client certificate is configured, used in the
   * certificate-required hint. Defaults to "Advanced" (applications); agents pass
   * "Credentials".
   */
  certificateLocation?: string;
}

const createTokenConfigSchema = (t: (key: string) => string) => {
  const validityField = z
    .number({error: t('applications:edit.token.validity.error')})
    .min(1, t('applications:edit.token.validity.error'));

  return z.object({
    validityPeriod: validityField,
    accessTokenValidity: validityField,
    idTokenValidity: validityField,
    refreshTokenValidity: validityField,
  });
};

type TokenConfigFormData = z.infer<ReturnType<typeof createTokenConfigSchema>>;

const computeValidityDefaults = (
  config: OAuth2Config | undefined,
  assertion: AssertionConfig | undefined,
): TokenConfigFormData => ({
  validityPeriod: config?.token?.validityPeriod ?? assertion?.validityPeriod ?? 3600,
  accessTokenValidity: config?.token?.accessToken?.userConfig?.validityPeriod ?? 3600,
  idTokenValidity: config?.token?.idToken?.validityPeriod ?? 3600,
  refreshTokenValidity: config?.token?.refreshToken?.validityPeriod ?? 86400,
});

// Stable identity for the default, so an untouched application doesn't invalidate the memos below
// on every render.
const NO_PENDING_CHANGES: Partial<Application> = {};

type TokenAttributeScope = 'shared' | 'access' | 'id' | 'userinfo';

const createEmptyAttributeSetState = (): Record<TokenAttributeScope, Set<string>> => ({
  shared: new Set(),
  access: new Set(),
  id: new Set(),
  userinfo: new Set(),
});

const areAttributesEqual = (arr1: string[], arr2: string[]): boolean => {
  if (arr1.length !== arr2.length) return false;
  const sorted1 = [...arr1].sort();
  const sorted2 = [...arr2].sort();
  return sorted1.every((val, index) => val === sorted2[index]);
};

/**
 * Flattens a scope-to-claims mapping into the set of attributes it exposes. ScopeClaims values are
 * optional, so the undefined entries are dropped here rather than at every call site.
 */
const collectMappedAttributes = (scopeClaims: ScopeClaims): Set<string> =>
  new Set(
    Object.values(scopeClaims)
      .flat()
      .filter((attr): attr is string => attr !== undefined),
  );

const areSetsEqual = (set1: Set<string>, set2: Set<string>): boolean => {
  if (set1.size !== set2.size) return false;

  return Array.from(set1).every((value) => set2.has(value));
};

/**
 * Container component for token configuration settings.
 *
 * Manages token settings for both OAuth2/OIDC mode and Native mode:
 * - OAuth2/OIDC mode: Separate access token and ID token configurations
 * - Native mode: Shared token configuration
 *
 * Provides sections for:
 * - Token validity periods (with real-time validation)
 * - User attributes to include in tokens
 * - JWT preview with syntax highlighting
 *
 * Features:
 * - Fetches user types for available user types
 * - Debounced updates (500ms) when changes are made
 * - Visual feedback for pending additions/removals
 * - Tab-based interface for access vs ID tokens in OAuth mode
 *
 * @param props - Component props
 * @returns Token settings UI sections wrapped in a Stack
 */
export default function EditTokenSettings({
  application,
  editedApp = NO_PENDING_CHANGES,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
  sectionResetKey = 0,
  entityLabel = 'application',
  showUserInfoTab = true,
  showActorClaim = false,
  actorSub = '<agent-id>',
  certificateLocation = 'Advanced',
}: EditTokenSettingsProps) {
  const logger = useLogger('EditTokenSettings');
  const {t} = useTranslation();
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  const [userTypes, setUserTypes] = useState<ApiUserType[]>([]);
  // The algorithm tokens are signed with is determined by the deployment's signing key, not a
  // per-application choice. It is surfaced read-only from the OIDC discovery document.
  const [signingAlg, setSigningAlg] = useState<string | undefined>(undefined);

  const {data: userTypesData, isLoading: userTypesLoading} = useGetUserTypes();
  const [activeTokenType, setActiveTokenType] = useState<'access' | 'id' | 'userinfo'>('access');
  const [pendingAdditionsByToken, setPendingAdditionsByToken] = useState<Record<TokenAttributeScope, Set<string>>>(() =>
    createEmptyAttributeSetState(),
  );
  const [pendingRemovalsByToken, setPendingRemovalsByToken] = useState<Record<TokenAttributeScope, Set<string>>>(() =>
    createEmptyAttributeSetState(),
  );
  const [highlightedAttributesByToken, setHighlightedAttributesByToken] = useState<
    Record<TokenAttributeScope, Set<string>>
  >(() => createEmptyAttributeSetState());

  // Stabilize allowedUserTypes array reference
  const allowedUserTypes = useMemo(() => application.allowedUserTypes ?? [], [application.allowedUserTypes]);

  // Get schema IDs for allowed user types
  const schemaIds = useMemo(() => {
    if (!userTypesData?.types || allowedUserTypes.length === 0) {
      return [];
    }

    return userTypesData.types.filter((schema) => allowedUserTypes.includes(schema.name)).map((schema) => schema.id);
  }, [userTypesData, allowedUserTypes]);

  // Determine if this is OAuth/OIDC mode (has separate token configs) or Native mode
  const isOAuthMode = useMemo(() => isOAuthTokenMode(oauth2Config), [oauth2Config]);

  // In Native mode both the attribute list and the validity period live on this single config, so
  // each is written by spreading the other's current value. Prefer the pending assertion whenever
  // the field has been touched — including an explicit undefined — otherwise the second edit would
  // spread the stored value and discard the first.
  const currentAssertion = useMemo(
    () => ('assertion' in editedApp ? editedApp.assertion : application.assertion),
    [editedApp, application.assertion],
  );

  const tokenConfigSchema = useMemo(() => createTokenConfigSchema(t), [t]);

  const {
    control,
    trigger,
    reset,
    formState: {errors, isValid},
  } = useForm<TokenConfigFormData>({
    resolver: zodResolver(tokenConfigSchema),
    mode: 'onChange',
    defaultValues: computeValidityDefaults(oauth2Config, currentAssertion),
  });

  const [validityPeriod, accessTokenValidity, idTokenValidity, refreshTokenValidity] = useWatch({
    control,
    name: ['validityPeriod', 'accessTokenValidity', 'idTokenValidity', 'refreshTokenValidity'],
  });

  useEffect(() => {
    onValidationChange?.(!isValid);
  }, [isValid, onValidationChange]);

  const oauth2ConfigRef = useRef(oauth2Config);
  const applicationRef = useRef(application);
  const currentAssertionRef = useRef(currentAssertion);
  const isFirstRenderRef = useRef(true);
  const prevSectionResetKeyRef = useRef(sectionResetKey);
  // Attributes this editing session added to the token allow-lists on behalf of a scope mapping.
  // Only these are auto-removed when the mapping drops them, so attributes added by hand in the
  // advanced panel are never taken away.
  const autoSyncedAttributes = useRef<Set<string>>(new Set());

  useEffect(() => {
    oauth2ConfigRef.current = oauth2Config;
  }, [oauth2Config]);

  useEffect(() => {
    applicationRef.current = application;
  }, [application]);

  useEffect(() => {
    currentAssertionRef.current = currentAssertion;
  }, [currentAssertion]);

  // Resets the Token Validity fields in place on Save/Reset — deliberately not a remount
  //  since TokenValidationSection owns its own local Access/ID/Refresh Token
  // sub-tab selection that must survive this. Also clears the pending attribute add/remove
  // highlight state below, which is edit-tracking state that should be cleared on Save/Reset.
  // Relies on the ref-sync effects above running first in the same commit so oauth2ConfigRef and
  // currentAssertionRef hold the latest values by the time this reads them — keep this effect
  // declared after them.
  useEffect(() => {
    if (sectionResetKey === prevSectionResetKeyRef.current) return;
    prevSectionResetKeyRef.current = sectionResetKey;

    reset(computeValidityDefaults(oauth2ConfigRef.current, currentAssertionRef.current));

    setPendingAdditionsByToken(createEmptyAttributeSetState());
    setPendingRemovalsByToken(createEmptyAttributeSetState());
    setHighlightedAttributesByToken(createEmptyAttributeSetState());
  }, [sectionResetKey, reset]);

  useEffect(() => {
    if (isFirstRenderRef.current) {
      isFirstRenderRef.current = false;
      return;
    }

    let cancelled = false;

    const applyIfValid = async () => {
      const valid = await trigger();
      if (cancelled || !valid) return;

      const baseline = computeValidityDefaults(oauth2ConfigRef.current, currentAssertionRef.current);

      if (isOAuthMode) {
        const config = oauth2ConfigRef.current;

        if (
          baseline.accessTokenValidity === accessTokenValidity &&
          baseline.idTokenValidity === idTokenValidity &&
          baseline.refreshTokenValidity === refreshTokenValidity
        ) {
          return; // No changes, skip update
        }

        const updatedConfig = {
          ...config,
          token: {
            ...config?.token,
            accessToken: {
              ...config?.token?.accessToken,
              userConfig: {
                ...config?.token?.accessToken?.userConfig,
                validityPeriod: accessTokenValidity,
              },
            },
            idToken: {
              ...config?.token?.idToken,
              validityPeriod: idTokenValidity,
            },
            refreshToken: {
              ...config?.token?.refreshToken,
              validityPeriod: refreshTokenValidity,
            },
          },
        };

        const updatedInboundAuth = applicationRef.current.inboundAuthConfig?.map((c) =>
          c.type === 'oauth2' ? {...c, config: updatedConfig} : c,
        );
        onFieldChange('inboundAuthConfig', updatedInboundAuth);
      } else {
        if (baseline.validityPeriod === validityPeriod) {
          return; // No changes, skip update
        }

        onFieldChange('assertion', {...currentAssertionRef.current, validityPeriod});
      }
    };

    applyIfValid().catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [validityPeriod, accessTokenValidity, idTokenValidity, refreshTokenValidity, trigger, isOAuthMode, onFieldChange]);

  /**
   * Fetch user types for all allowed user types
   */
  useEffect(() => {
    if (schemaIds.length === 0) return;

    const fetchSchemas = async () => {
      const serverUrl = getServerUrl();

      try {
        const schemaPromises = schemaIds.map(async (id) => {
          try {
            const response = await http.request({
              url: `${serverUrl}/user-types/${id}`,
              method: 'GET',
            } as unknown as Parameters<typeof http.request>[0]);
            return response.data as ApiUserType;
          } catch (err) {
            logger.error('Failed to fetch user type', {error: err, userTypeId: id});
            return null;
          }
        });

        const responses = await Promise.all(schemaPromises);
        const schemas = responses.filter((schema): schema is ApiUserType => schema !== null);
        setUserTypes(schemas);
      } catch (err) {
        logger.error('Failed to fetch user types', {error: err});
        setUserTypes([]);
      }
    };

    fetchSchemas().catch((err) => {
      logger.error('Unexpected error in fetchUserTypes', {error: err});
    });
  }, [schemaIds, http, getServerUrl, logger]);

  /**
   * Fetch the deployment's signing algorithm from the OIDC discovery document. Signing is done
   * with the server key, so this is informational only and shown read-only in the token sections.
   * The discovery document is public, so it is fetched without credentials; sending an
   * Authorization header would fail its CORS preflight (only Content-Type is allowed).
   */
  useEffect(() => {
    if (!isOAuthMode) return undefined;

    let cancelled = false;

    const fetchSigningAlg = async () => {
      try {
        const response = await fetch(`${getServerUrl()}/.well-known/openid-configuration`);
        if (cancelled) return;
        if (!response.ok) {
          logger.error('Discovery request for signing algorithm returned a non-OK status', {
            status: response.status,
          });
          return;
        }
        const data = (await response.json()) as {id_token_signing_alg_values_supported?: string[]};
        if (cancelled) return;
        const algs = data?.id_token_signing_alg_values_supported;
        if (Array.isArray(algs) && algs.length > 0) {
          setSigningAlg(algs[0]);
        }
      } catch (err) {
        if (!cancelled) {
          logger.error('Failed to fetch signing algorithm from discovery', {error: err});
        }
      }
    };

    void fetchSigningAlg();

    return () => {
      cancelled = true;
    };
  }, [isOAuthMode, getServerUrl, logger]);

  const userAttributes = useMemo(() => {
    if (userTypes.length === 0) return [];

    const flattenAttributes = (schema: Record<string, PropertyDefinition>, prefix = ''): string[] => {
      const attributes: string[] = [];

      Object.entries(schema).forEach(([key, value]) => {
        const fullKey = `${prefix}${key}`;

        if ('credential' in value && value.credential) {
          return;
        }

        if (value.type === 'object' && 'properties' in value) {
          // Recursively flatten nested objects
          attributes.push(...flattenAttributes(value.properties, `${fullKey}.`));
        } else if (value.type !== 'array') {
          // Add primitive types (string, number, boolean)
          attributes.push(fullKey);
        }
      });

      return attributes;
    };

    // Combine attributes from all allowed user types and remove duplicates
    const allAttributes = new Set<string>();
    userTypes.forEach((userType) => {
      const attributes = flattenAttributes(userType.schema);
      attributes.forEach((attr) => allAttributes.add(attr));
    });

    return Array.from(allAttributes).sort();
  }, [userTypes]);

  const isLoadingUserAttributes = userTypesLoading;

  const sharedUserAttributes = useMemo(() => {
    if (isOAuthMode) {
      // For OAuth mode, this is not used but kept for compatibility
      return [];
    }

    return currentAssertion?.userAttributes ?? [];
  }, [isOAuthMode, currentAssertion]);

  const currentAccessTokenAttributes = useMemo(
    () => oauth2Config?.token?.accessToken?.userConfig?.attributes ?? [],
    [oauth2Config],
  );

  const currentIdTokenAttributes = useMemo(() => oauth2Config?.token?.idToken?.userAttributes ?? [], [oauth2Config]);

  const currentUserInfoAttributes = useMemo(() => {
    if (!isOAuthMode || !oauth2Config) return [];

    return oauth2Config.userInfo?.userAttributes ?? oauth2Config.token?.idToken?.userAttributes ?? [];
  }, [isOAuthMode, oauth2Config]);

  const derivedIsCustom = useMemo(() => {
    if (!isOAuthMode || !oauth2Config?.userInfo) return false;
    const userInfoAttrs = oauth2Config.userInfo.userAttributes ?? [];
    const idTokenAttrs = oauth2Config.token?.idToken?.userAttributes ?? [];
    return !areAttributesEqual(userInfoAttrs, idTokenAttrs);
  }, [isOAuthMode, oauth2Config]);

  const [isUserInfoCustomAttributes, setIsUserInfoCustomAttributes] = useState(derivedIsCustom);

  // Sync local toggle state when the derived value changes due to external config updates
  useEffect(() => {
    setIsUserInfoCustomAttributes(derivedIsCustom);
  }, [derivedIsCustom]);

  const handleToggleUserInfo = (checked: boolean) => {
    setIsUserInfoCustomAttributes(checked);

    if (!checked && activeTokenType === 'userinfo') {
      setPendingAdditionsByToken((prev) => ({...prev, userinfo: new Set()}));
      setPendingRemovalsByToken((prev) => ({...prev, userinfo: new Set()}));
      setHighlightedAttributesByToken((prev) => ({...prev, userinfo: new Set()}));
    }

    if (checked) {
      // When enabling, start with ID token attributes if current UserInfo attrs are empty/undefined
      if (!oauth2Config?.userInfo?.userAttributes?.length) {
        const updatedConfig = {
          ...oauth2Config,
          userInfo: {
            ...oauth2Config?.userInfo,
            userAttributes: [...currentIdTokenAttributes],
          },
        };

        const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
          if (config.type === 'oauth2') {
            return {...config, config: updatedConfig};
          }
          return config;
        });
        onFieldChange('inboundAuthConfig', updatedInboundAuth);
      }
    } else if (oauth2Config) {
      // When disabling custom mode, sync userInfo attributes with ID token attributes
      const updatedConfig = {
        ...oauth2Config,
        userInfo: {
          ...oauth2Config.userInfo,
          userAttributes: [...currentIdTokenAttributes],
        },
      };

      const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
        if (config.type === 'oauth2') {
          return {...config, config: updatedConfig};
        }
        return config;
      });
      onFieldChange('inboundAuthConfig', updatedInboundAuth);
    }
  };

  const handleScopeClaimsChange = (newScopeClaims: ScopeClaims) => {
    const mappedBefore = collectMappedAttributes(oauth2Config?.scopeClaims ?? {});
    const mappedAfter = collectMappedAttributes(newScopeClaims);

    const added = [...mappedAfter].filter((attr) => !mappedBefore.has(attr));
    // Only attributes this session added through the mapper are auto-removed: anything added by
    // hand in the advanced panel stays, since it may be there for the claims request parameter.
    const removed = [...mappedBefore].filter(
      (attr) => !mappedAfter.has(attr) && autoSyncedAttributes.current.has(attr),
    );

    added.forEach((attr) => autoSyncedAttributes.current.add(attr));
    removed.forEach((attr) => autoSyncedAttributes.current.delete(attr));

    const syncList = (current: string[]): string[] => [
      ...current.filter((attr) => !removed.includes(attr)),
      ...added.filter((attr) => !current.includes(attr)),
    ];

    // The mapping and both allow-lists are written as one config object: writing them through
    // separate onFieldChange calls would have each rebuild oauth2Config from the same stale copy,
    // so the last write would drop the other's change.
    const idTokenAttributes = syncList(currentIdTokenAttributes);
    const updatedConfig = {
      ...oauth2Config,
      scopeClaims: newScopeClaims,
      token: {
        ...oauth2Config?.token,
        idToken: {
          validityPeriod: oauth2Config?.token?.idToken?.validityPeriod ?? 3600,
          ...oauth2Config?.token?.idToken,
          userAttributes: idTokenAttributes,
        },
      },
      userInfo: {
        ...oauth2Config?.userInfo,
        userAttributes: syncList(currentUserInfoAttributes),
      },
    };

    if (added.length > 0 || removed.length > 0) {
      setHighlightedAttributesByToken((prev) => ({
        ...prev,
        id: new Set([...prev.id, ...added, ...removed]),
        userinfo: new Set([...prev.userinfo, ...added, ...removed]),
      }));
    }

    const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
      if (config.type === 'oauth2') {
        return {...config, config: updatedConfig};
      }

      return config;
    });
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleIdTokenConfigChange = (field: string, value: string) => {
    const nextIdToken = {
      ...oauth2Config?.token?.idToken,
      userAttributes: oauth2Config?.token?.idToken?.userAttributes ?? [],
      validityPeriod: oauth2Config?.token?.idToken?.validityPeriod ?? 3600,
      [field]: value,
    };
    // Switching to a non-encrypted format must drop the encryption fields, otherwise the backend
    // rejects the config (encryption fields require an encrypted response type and a certificate).
    if (field === 'responseType' && value !== 'JWE' && value !== 'NESTED_JWT') {
      delete nextIdToken.encryptionAlg;
      delete nextIdToken.encryptionEnc;
    }
    const updatedConfig = {
      ...oauth2Config,
      token: {
        ...oauth2Config?.token,
        idToken: nextIdToken,
      },
    };
    const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
      if (config.type === 'oauth2') {
        return {...config, config: updatedConfig};
      }
      return config;
    });
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleUserInfoConfigChange = (field: string, value: string) => {
    const nextUserInfo = {
      ...oauth2Config?.userInfo,
      userAttributes: oauth2Config?.userInfo?.userAttributes ?? oauth2Config?.token?.idToken?.userAttributes ?? [],
      [field]: value,
    };
    // Switching to a non-encrypted format must drop the encryption fields, otherwise the backend
    // rejects the config (encryption fields require an encrypted response type and a certificate).
    if (field === 'responseType' && value !== 'JWE' && value !== 'NESTED_JWT') {
      delete nextUserInfo.encryptionAlg;
      delete nextUserInfo.encryptionEnc;
    }
    // Signing always uses the server key, so a stale per-app signing algorithm is never sent.
    if (field === 'responseType') {
      delete nextUserInfo.signingAlg;
    }
    const updatedConfig = {
      ...oauth2Config,
      userInfo: nextUserInfo,
    };
    const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
      if (config.type === 'oauth2') {
        return {...config, config: updatedConfig};
      }
      return config;
    });
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const currentAttributesByToken = useMemo<Record<TokenAttributeScope, string[]>>(
    () => ({
      shared: sharedUserAttributes,
      access: currentAccessTokenAttributes,
      id: currentIdTokenAttributes,
      userinfo: currentUserInfoAttributes,
    }),
    [sharedUserAttributes, currentAccessTokenAttributes, currentIdTokenAttributes, currentUserInfoAttributes],
  );

  // Derive effective pending sets by filtering out stale entries
  const effectivePendingAdditions = useMemo(() => {
    let hasChanges = false;
    const result = {...pendingAdditionsByToken};
    const allScopes: TokenAttributeScope[] = ['shared', 'access', 'id', 'userinfo'];

    allScopes.forEach((scope) => {
      const remaining = new Set(
        Array.from(pendingAdditionsByToken[scope]).filter((attr) => !currentAttributesByToken[scope].includes(attr)),
      );

      if (!areSetsEqual(pendingAdditionsByToken[scope], remaining)) {
        result[scope] = remaining;
        hasChanges = true;
      }
    });

    return hasChanges ? result : pendingAdditionsByToken;
  }, [pendingAdditionsByToken, currentAttributesByToken]);

  const effectivePendingRemovals = useMemo(() => {
    let hasChanges = false;
    const result = {...pendingRemovalsByToken};
    const allScopes: TokenAttributeScope[] = ['shared', 'access', 'id', 'userinfo'];

    allScopes.forEach((scope) => {
      const remaining = new Set(
        Array.from(pendingRemovalsByToken[scope]).filter((attr) => currentAttributesByToken[scope].includes(attr)),
      );

      if (!areSetsEqual(pendingRemovalsByToken[scope], remaining)) {
        result[scope] = remaining;
        hasChanges = true;
      }
    });

    return hasChanges ? result : pendingRemovalsByToken;
  }, [pendingRemovalsByToken, currentAttributesByToken]);

  // Clear highlights when all pending changes for a scope have been applied
  useEffect(() => {
    const allScopes: TokenAttributeScope[] = ['shared', 'access', 'id', 'userinfo'];
    const clearedScopes = allScopes.filter(
      (scope) =>
        (pendingRemovalsByToken[scope].size > 0 || pendingAdditionsByToken[scope].size > 0) &&
        effectivePendingRemovals[scope].size === 0 &&
        effectivePendingAdditions[scope].size === 0,
    );

    if (clearedScopes.length === 0) return;

    const timeout = setTimeout(() => {
      setHighlightedAttributesByToken((prev) => {
        let hasUpdates = false;
        const next = {...prev};

        clearedScopes.forEach((scope) => {
          if (next[scope].size > 0) {
            next[scope] = new Set();
            hasUpdates = true;
          }
        });

        return hasUpdates ? next : prev;
      });
    }, 500);

    return () => clearTimeout(timeout);
  }, [effectivePendingAdditions, effectivePendingRemovals, pendingAdditionsByToken, pendingRemovalsByToken]);

  const applyAttributeChange = (scope: TokenAttributeScope, nextAttrs: string[]) => {
    if (isOAuthMode && oauth2Config) {
      let updatedConfig = {...oauth2Config};

      const defaultTokenConfig = {validityPeriod: 3600, userAttributes: [] as string[]};
      const defaultAccessToken = {userConfig: {validityPeriod: 3600, attributes: [] as string[]}};
      const currentAccessToken = updatedConfig.token?.accessToken ?? defaultAccessToken;
      const currentIdToken = updatedConfig.token?.idToken ?? defaultTokenConfig;

      if (scope === 'access') {
        updatedConfig = {
          ...updatedConfig,
          token: {
            ...updatedConfig.token,
            accessToken: {
              ...currentAccessToken,
              userConfig: {...currentAccessToken.userConfig, attributes: nextAttrs},
            },
            idToken: currentIdToken,
          },
        };
      } else if (scope === 'id') {
        updatedConfig = {
          ...updatedConfig,
          token: {
            ...updatedConfig.token,
            accessToken: currentAccessToken,
            idToken: {...currentIdToken, userAttributes: nextAttrs},
          },
        };

        // While User Info inherits from the ID token the two lists must stay identical, since the
        // inherit toggle is derived from their equality. Writing only the ID token list would both
        // flip the toggle off and leave User Info on the pre-edit list.
        if (!isUserInfoCustomAttributes) {
          updatedConfig = {
            ...updatedConfig,
            userInfo: {...updatedConfig.userInfo, userAttributes: nextAttrs},
          };
        }
      } else if (scope === 'userinfo') {
        updatedConfig = {
          ...updatedConfig,
          userInfo: {
            ...updatedConfig.userInfo,
            userAttributes: nextAttrs,
          },
        };
      }

      const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
        if (config.type === 'oauth2') {
          return {...config, config: updatedConfig};
        }
        return config;
      });
      onFieldChange('inboundAuthConfig', updatedInboundAuth);
    } else if (scope === 'shared') {
      onFieldChange('assertion', {...currentAssertion, userAttributes: nextAttrs});
    }
  };

  // Handle attribute click
  const handleAttributeClick = (attr: string, tokenType: 'shared' | 'access' | 'id' | 'userinfo') => {
    let currentAttributes: string[];
    if (tokenType === 'shared') {
      currentAttributes = sharedUserAttributes;
    } else if (tokenType === 'access') {
      currentAttributes = currentAccessTokenAttributes;
    } else if (tokenType === 'id') {
      currentAttributes = currentIdTokenAttributes;
    } else {
      currentAttributes = currentUserInfoAttributes;
    }

    const isAdded = currentAttributes.includes(attr);
    const isPendingAddition = effectivePendingAdditions[tokenType].has(attr);
    const isPendingRemoval = effectivePendingRemovals[tokenType].has(attr);

    setHighlightedAttributesByToken((prev) => ({
      ...prev,
      [tokenType]: new Set([...prev[tokenType], attr]),
    }));

    const currentlyActive = (isAdded && !isPendingRemoval) || isPendingAddition;

    let nextAttrs: string[];
    if (currentlyActive) {
      // Removing the attribute
      nextAttrs = currentAttributes.filter((a) => a !== attr);
      if (isPendingAddition) {
        setPendingAdditionsByToken((prev) => {
          const newSet = new Set(prev[tokenType]);
          newSet.delete(attr);
          return {...prev, [tokenType]: newSet};
        });
      } else if (isAdded) {
        setPendingRemovalsByToken((prev) => ({
          ...prev,
          [tokenType]: new Set([...prev[tokenType], attr]),
        }));
      }
    } else {
      // Adding the attribute
      nextAttrs = [...currentAttributes, attr];
      if (isPendingRemoval) {
        setPendingRemovalsByToken((prev) => {
          const newSet = new Set(prev[tokenType]);
          newSet.delete(attr);
          return {...prev, [tokenType]: newSet};
        });
      } else {
        setPendingAdditionsByToken((prev) => ({
          ...prev,
          [tokenType]: new Set([...prev[tokenType], attr]),
        }));
      }
    }

    applyAttributeChange(tokenType, nextAttrs);
  };

  const visibleScope: TokenAttributeScope = isOAuthMode ? activeTokenType : 'shared';
  const visiblePendingAdditions = effectivePendingAdditions[visibleScope];
  const visiblePendingRemovals = effectivePendingRemovals[visibleScope];
  const visibleHighlightedAttributes = highlightedAttributesByToken[visibleScope];

  return (
    <Stack spacing={3}>
      {/* OAuth/OIDC Mode */}
      {isOAuthMode ? (
        <>
          {/* Merged User Attributes (Access Token / ID Token / User Info tabs). The scope-to-claims
              mapping is passed in as a slot so it renders once, under the two tabs it feeds. */}
          <TokenUserAttributesSection
            accessTokenAttributes={currentAccessTokenAttributes}
            idTokenAttributes={currentIdTokenAttributes}
            userInfoAttributes={currentUserInfoAttributes}
            activeTab={activeTokenType}
            onTabChange={setActiveTokenType}
            isUserInfoCustomAttributes={isUserInfoCustomAttributes}
            onToggleUserInfo={handleToggleUserInfo}
            userAttributes={userAttributes}
            isLoadingUserAttributes={isLoadingUserAttributes}
            pendingAdditions={visiblePendingAdditions}
            pendingRemovals={visiblePendingRemovals}
            highlightedAttributes={visibleHighlightedAttributes}
            onAttributeClick={handleAttributeClick}
            entityLabel={entityLabel}
            showUserInfoTab={showUserInfoTab}
            showActorClaim={showActorClaim}
            actorSub={actorSub}
            disabled={application.isReadOnly}
            signingAlg={signingAlg}
            hasCertificate={Boolean(oauth2Config?.certificate?.type)}
            certificateLocation={certificateLocation}
            idTokenResponseType={oauth2Config?.token?.idToken?.responseType}
            idTokenEncryptionAlg={oauth2Config?.token?.idToken?.encryptionAlg}
            idTokenEncryptionEnc={oauth2Config?.token?.idToken?.encryptionEnc}
            onIdTokenConfigChange={handleIdTokenConfigChange}
            userInfoResponseType={oauth2Config?.userInfo?.responseType}
            userInfoEncryptionAlg={oauth2Config?.userInfo?.encryptionAlg}
            userInfoEncryptionEnc={oauth2Config?.userInfo?.encryptionEnc}
            onUserInfoConfigChange={handleUserInfoConfigChange}
            scopeMapping={
              <ScopeSection
                scopeClaims={oauth2Config?.scopeClaims ?? {}}
                userAttributes={userAttributes}
                isLoadingUserAttributes={isLoadingUserAttributes}
                hasAllowedUserTypes={allowedUserTypes.length > 0}
                onScopeClaimsChange={handleScopeClaimsChange}
                entityLabel={entityLabel}
                disabled={application.isReadOnly}
              />
            }
          />

          {/* Merged Token Validation (Access Token / ID Token tabs) */}
          <TokenValidationSection
            control={control}
            errors={errors}
            tokenType="oauth"
            disabled={application.isReadOnly}
          />
        </>
      ) : (
        <>
          {/* Native Flow Mode */}
          <TokenUserAttributesSection
            sharedAttributes={sharedUserAttributes}
            userAttributes={userAttributes}
            isLoadingUserAttributes={isLoadingUserAttributes}
            pendingAdditions={visiblePendingAdditions}
            pendingRemovals={visiblePendingRemovals}
            highlightedAttributes={visibleHighlightedAttributes}
            onAttributeClick={handleAttributeClick}
            entityLabel={entityLabel}
            disabled={application.isReadOnly}
          />

          {/* Token Validation */}
          <TokenValidationSection
            control={control}
            errors={errors}
            tokenType="shared"
            disabled={application.isReadOnly}
          />
        </>
      )}
    </Stack>
  );
}
