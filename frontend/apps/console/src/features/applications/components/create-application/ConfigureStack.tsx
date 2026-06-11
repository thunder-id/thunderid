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

import {Box, Typography, Stack, Card, CardContent, Chip} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState, useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import PlatformBasedApplicationTemplateMetadata from '../../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../../config/TechnologyBasedApplicationTemplateMetadata';
import useApplicationCreate from '../../contexts/ApplicationCreate/useApplicationCreate';
import type {
  ApplicationTemplate,
  ApplicationTemplateMetadata,
  TemplateCategory,
} from '../../models/application-templates';
import {TechnologyApplicationTemplate, PlatformApplicationTemplate} from '../../models/application-templates';
import {TokenEndpointAuthMethods, type OAuth2Config} from '../../models/oauth';
import inferApplicationTemplateTechnologyFromConfig from '../../utils/inferApplicationTemplateTechnologyFromConfig';

type CategoryFilter = TemplateCategory | 'all';

const CATEGORIES: {value: CategoryFilter; titleKey: string}[] = [
  {value: 'all', titleKey: 'applications:onboarding.configure.stack.category.all'},
  {value: 'web', titleKey: 'applications:onboarding.configure.stack.category.web'},
  {value: 'backend', titleKey: 'applications:onboarding.configure.stack.category.backend'},
  {value: 'mobile', titleKey: 'applications:onboarding.configure.stack.category.mobile'},
];

const CATEGORY_I18N_KEY: Record<TemplateCategory, string> = {
  web: 'applications:onboarding.configure.stack.category.web',
  backend: 'applications:onboarding.configure.stack.category.backend',
  mobile: 'applications:onboarding.configure.stack.category.mobile',
};

type AnyTemplateMetadata = ApplicationTemplateMetadata<TechnologyApplicationTemplate | PlatformApplicationTemplate>;

const ALL_TEMPLATES: AnyTemplateMetadata[] = [
  ...(TechnologyBasedApplicationTemplateMetadata as AnyTemplateMetadata[]),
  ...(PlatformBasedApplicationTemplateMetadata as AnyTemplateMetadata[]),
];

const TECHNOLOGY_VALUES = new Set<string>(TechnologyBasedApplicationTemplateMetadata.map((m) => m.value));

const TechnologyBasedTemplates: Record<TechnologyApplicationTemplate, ApplicationTemplate> =
  TechnologyBasedApplicationTemplateMetadata.reduce(
    (acc, item) => ({...acc, [item.value]: item.template}),
    {} as Record<TechnologyApplicationTemplate, ApplicationTemplate>,
  );

const PlatformBasedTemplates: Record<PlatformApplicationTemplate, ApplicationTemplate> =
  PlatformBasedApplicationTemplateMetadata.reduce(
    (acc, item) => ({...acc, [item.value]: item.template}),
    {} as Record<PlatformApplicationTemplate, ApplicationTemplate>,
  );

/**
 * Props for the {@link ConfigureStack} component.
 *
 * @public
 */
export interface ConfigureStackProps {
  /**
   * OAuth configuration
   */
  oauthConfig: OAuth2Config | null;

  /**
   * Callback function when OAuth configuration changes
   */
  onOAuthConfigChange: (config: OAuth2Config | null) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;
}

/**
 * Unified template gallery for the application creation onboarding flow.
 *
 * @public
 */
export default function ConfigureStack({
  oauthConfig,
  onOAuthConfigChange,
  onReadyChange = undefined,
}: ConfigureStackProps): JSX.Element {
  const {t} = useTranslation();

  const {selectedTechnology, setSelectedTechnology, selectedPlatform, setSelectedPlatform, setSelectedTemplateConfig} =
    useApplicationCreate();

  const [selectedCategory, setSelectedCategory] = useState<CategoryFilter>('all');

  const defaultTechnology: TechnologyApplicationTemplate =
    TechnologyBasedApplicationTemplateMetadata[0]?.value ?? TechnologyApplicationTemplate.REACT;
  const inferredTechnology: TechnologyApplicationTemplate = inferApplicationTemplateTechnologyFromConfig(oauthConfig);
  const defaultPlatform: PlatformApplicationTemplate =
    PlatformBasedApplicationTemplateMetadata[0]?.value ?? PlatformApplicationTemplate.BROWSER;

  const getResolvedTechnology = (): TechnologyApplicationTemplate => {
    if (selectedTechnology) return selectedTechnology;
    if (selectedPlatform) return TechnologyApplicationTemplate.OTHER;
    if (oauthConfig) return inferredTechnology;
    return defaultTechnology;
  };

  const resolvedTechnology: TechnologyApplicationTemplate = getResolvedTechnology();
  const platformForTemplate: PlatformApplicationTemplate = selectedPlatform ?? defaultPlatform;

  const technologyConfig: ApplicationTemplate =
    resolvedTechnology === TechnologyApplicationTemplate.OTHER
      ? PlatformBasedTemplates[platformForTemplate]
      : TechnologyBasedTemplates[resolvedTechnology];

  useEffect((): void => {
    setSelectedTemplateConfig(technologyConfig);

    const oauthInboundConfig: OAuth2Config = technologyConfig.defaults?.inboundAuthConfig?.[0]?.config ?? {
      publicClient: false,
      pkceRequired: false,
      grantTypes: [],
      responseTypes: [],
      redirectUris: [],
      tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
    };

    onOAuthConfigChange({
      publicClient: oauthInboundConfig.publicClient,
      pkceRequired: oauthInboundConfig.pkceRequired,
      grantTypes: [...oauthInboundConfig.grantTypes],
      responseTypes: [...(oauthInboundConfig.responseTypes ?? [])],
      redirectUris: oauthInboundConfig.redirectUris ? [...oauthInboundConfig.redirectUris] : [],
      tokenEndpointAuthMethod: oauthInboundConfig.tokenEndpointAuthMethod,
      scopes: ['openid', 'profile', 'email'],
    });
  }, [resolvedTechnology, platformForTemplate, onOAuthConfigChange, technologyConfig, setSelectedTemplateConfig]);

  useEffect((): void => {
    const isReady: boolean = resolvedTechnology !== TechnologyApplicationTemplate.OTHER || selectedPlatform !== null;
    onReadyChange?.(isReady);
  }, [resolvedTechnology, selectedPlatform, onReadyChange]);

  const handleTemplateSelect = (value: TechnologyApplicationTemplate | PlatformApplicationTemplate): void => {
    if (TECHNOLOGY_VALUES.has(value)) {
      setSelectedTechnology(value as TechnologyApplicationTemplate);
      setSelectedPlatform(null);
    } else {
      setSelectedTechnology(null);
      setSelectedPlatform(value as PlatformApplicationTemplate);
    }
  };

  const isTemplateSelected = (value: TechnologyApplicationTemplate | PlatformApplicationTemplate): boolean => {
    if (TECHNOLOGY_VALUES.has(value)) return resolvedTechnology === value;
    return selectedPlatform === value;
  };

  const filteredTemplates: AnyTemplateMetadata[] =
    selectedCategory === 'all'
      ? ALL_TEMPLATES
      : ALL_TEMPLATES.filter((tmpl) => tmpl.categories.includes(selectedCategory));

  return (
    <Stack direction="column" spacing={3} data-testid="application-configure-stack">
      {/* Header */}
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1">{t('applications:onboarding.configure.stack.title')}</Typography>
        <Typography variant="body1" color="text.secondary">
          {t('applications:onboarding.configure.stack.subtitle')}
        </Typography>
      </Stack>

      {/* Category filter chips */}
      <Stack direction="row" spacing={1} flexWrap="wrap">
        {CATEGORIES.map((cat) => {
          const isActive = selectedCategory === cat.value;
          return (
            <Chip
              key={cat.value}
              label={t(cat.titleKey)}
              onClick={() => setSelectedCategory(cat.value)}
              variant={isActive ? 'filled' : 'outlined'}
              color={isActive ? 'primary' : 'default'}
              sx={{
                fontWeight: isActive ? 600 : 400,
                borderRadius: '20px',
                cursor: 'pointer',
              }}
            />
          );
        })}
      </Stack>

      {/* Template grid */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {
            xs: '1fr',
            sm: 'repeat(2, 1fr)',
            md: 'repeat(3, 1fr)',
            xl: 'repeat(4, 1fr)',
          },
          gap: 2,
        }}
      >
        {filteredTemplates.map((option) => {
          const isSelected: boolean = isTemplateSelected(option.value);
          const isDisabled: boolean = option.disabled ?? false;

          return (
            <Card
              key={option.value}
              variant="outlined"
              role="button"
              tabIndex={isDisabled ? -1 : 0}
              aria-disabled={isDisabled}
              aria-pressed={isSelected}
              onClick={isDisabled ? undefined : () => handleTemplateSelect(option.value)}
              onKeyDown={(e) => {
                if (!isDisabled && (e.key === 'Enter' || e.key === ' ')) {
                  e.preventDefault();
                  handleTemplateSelect(option.value);
                }
              }}
              sx={{
                position: 'relative',
                borderRadius: 2,
                borderWidth: isSelected ? 2 : 1,
                borderColor: isSelected ? 'primary.main' : 'divider',
                cursor: isDisabled ? 'not-allowed' : 'pointer',
                opacity: isDisabled ? 0.5 : 1,
                bgcolor: isSelected ? 'action.selected' : 'background.paper',
                transition: 'border-color 0.15s, box-shadow 0.15s, transform 0.15s',
                '&:hover': isDisabled
                  ? {}
                  : {
                      borderColor: 'primary.main',
                      boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                      transform: 'translateY(-2px)',
                    },
                '&:focus-visible': isDisabled
                  ? {}
                  : {
                      outline: 'none',
                      borderColor: 'primary.main',
                      boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                      transform: 'translateY(-2px)',
                    },
              }}
            >
              {isDisabled && (
                <Box
                  sx={{
                    position: 'absolute',
                    top: 10,
                    right: 10,
                    bgcolor: 'warning.main',
                    color: 'warning.contrastText',
                    px: 1,
                    py: 0.25,
                    borderRadius: 1,
                    fontSize: '0.7rem',
                    fontWeight: 700,
                    letterSpacing: 0.3,
                    zIndex: 1,
                  }}
                >
                  {t('applications:onboarding.configure.stack.comingSoon')}
                </Box>
              )}

              <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
                <Stack direction="column" spacing={2}>
                  {/* Icon */}
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      width: 48,
                      height: 48,
                    }}
                  >
                    {option.icon}
                  </Box>

                  {/* Title + description */}
                  <Stack direction="column" spacing={0.75} sx={{flex: 1}}>
                    <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                      {t(option.titleKey)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                      {t(option.descriptionKey)}
                    </Typography>
                  </Stack>

                  {/* Category tags */}
                  <Stack direction="row" spacing={0.75} flexWrap="wrap">
                    {option.categories.map((cat) => (
                      <Typography
                        key={cat}
                        variant="caption"
                        color="text.disabled"
                        sx={{fontWeight: 500, letterSpacing: 0.2}}
                      >
                        #{t(CATEGORY_I18N_KEY[cat]).toLocaleLowerCase()}
                      </Typography>
                    ))}
                  </Stack>
                </Stack>
              </CardContent>
            </Card>
          );
        })}
      </Box>
    </Stack>
  );
}
