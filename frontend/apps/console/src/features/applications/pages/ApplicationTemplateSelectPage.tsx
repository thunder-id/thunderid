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

import {useLogger} from '@thunderid/logger/react';
import {
  Box,
  Card,
  CardContent,
  Chip,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Search} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useLocation, useNavigate, useSearchParams} from 'react-router';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';
import useApplicationCreate from '../contexts/ApplicationCreate/useApplicationCreate';
import {ApplicationCreateFlowStep} from '../models/application-create-flow';
import type {ApplicationTemplateMetadata, TemplateCategory} from '../models/application-templates';
import {PlatformApplicationTemplate, TechnologyApplicationTemplate} from '../models/application-templates';
import resolveCreationFlow from '../utils/resolveCreationFlow';

type CategoryFilter = TemplateCategory | 'all';

const CATEGORIES: {value: CategoryFilter; titleKey: string}[] = [
  {value: 'all', titleKey: 'applications:onboarding.configure.stack.category.all'},
  {value: 'web', titleKey: 'applications:onboarding.configure.stack.category.web'},
  {value: 'backend', titleKey: 'applications:onboarding.configure.stack.category.backend'},
  {value: 'mobile', titleKey: 'applications:onboarding.configure.stack.category.mobile'},
  {value: 'ai', titleKey: 'applications:onboarding.configure.stack.category.ai'},
];

const CATEGORY_I18N_KEY: Record<TemplateCategory, string> = {
  web: 'applications:onboarding.configure.stack.category.web',
  backend: 'applications:onboarding.configure.stack.category.backend',
  mobile: 'applications:onboarding.configure.stack.category.mobile',
  ai: 'applications:onboarding.configure.stack.category.ai',
};

type AnyTemplateMetadata = ApplicationTemplateMetadata<TechnologyApplicationTemplate | PlatformApplicationTemplate>;

const ALL_TEMPLATES: AnyTemplateMetadata[] = [
  ...(TechnologyBasedApplicationTemplateMetadata as AnyTemplateMetadata[]),
  ...(PlatformBasedApplicationTemplateMetadata as AnyTemplateMetadata[]),
];

const TECHNOLOGY_VALUES = new Set<string>(TechnologyBasedApplicationTemplateMetadata.map((m) => m.value));

/**
 * In-console template gallery. Selecting a template seeds the application-create context and
 * launches the full-screen creation wizard.
 *
 * @public
 */
export default function ApplicationTemplateSelectPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const [searchParams] = useSearchParams();
  const logger = useLogger('ApplicationTemplateSelectPage');

  const isWelcomeFlow = pathname.startsWith('/welcome');

  const {reset, setSelectedTechnology, setSelectedPlatform, setSelectedTemplateConfig, setCurrentStep} =
    useApplicationCreate();

  const [selectedCategory, setSelectedCategory] = useState<CategoryFilter>('all');
  const [searchQuery, setSearchQuery] = useState<string>('');

  const filteredTemplates: AnyTemplateMetadata[] = useMemo(() => {
    const query: string = searchQuery.trim().toLocaleLowerCase();
    return ALL_TEMPLATES.filter((tmpl) => {
      const matchesCategory: boolean = selectedCategory === 'all' || tmpl.categories.includes(selectedCategory);
      const matchesSearch: boolean = query === '' || t(tmpl.titleKey).toLocaleLowerCase().includes(query);
      return matchesCategory && matchesSearch;
    });
  }, [selectedCategory, searchQuery, t]);

  const handleTemplateSelect = (option: AnyTemplateMetadata): void => {
    if (option.disabled) return;

    reset();

    if (TECHNOLOGY_VALUES.has(option.value)) {
      setSelectedTechnology(option.value as TechnologyApplicationTemplate);
      setSelectedPlatform(null);
    } else {
      setSelectedTechnology(null);
      setSelectedPlatform(option.value as PlatformApplicationTemplate);
    }

    setSelectedTemplateConfig(option.template);

    // The wizard no longer owns the template step, so advance to the first real step of this
    // template's creation flow before handing off.
    const firstStep = resolveCreationFlow(option.template).steps.find(
      (step) => step !== ApplicationCreateFlowStep.STACK,
    );
    if (firstStep) {
      setCurrentStep(firstStep);
    }

    const wizardPath = isWelcomeFlow ? '/welcome/get-started/applications/create' : '/applications/create';

    (async () => {
      await navigate(`${wizardPath}?type=${option.value}`);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to application creation wizard', {error, template: option.value});
    });
  };

  // Entry points elsewhere in the console (e.g. the home page's framework picker) deep-link here
  // with a preselected type, skipping the gallery straight to the wizard.
  useEffect(() => {
    const typeParam = searchParams.get('type');
    if (!typeParam) return;

    const preselected = ALL_TEMPLATES.find((tmpl) => tmpl.value === typeParam);
    if (preselected && !preselected.disabled) {
      handleTemplateSelect(preselected);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to={isWelcomeFlow ? '/welcome/get-started' : '/applications'} />}>
          {isWelcomeFlow ? t('common:welcome.getStarted.breadcrumb') : t('applications:edit.page.back')}
        </PageTitle.BackButton>
        <PageTitle.Header>{t('applications:onboarding.configure.stack.title', 'Choose a type')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'applications:onboarding.templateSelect.subtitle',
            'Pick the technology that best matches your application, selecting one starts the setup.',
          )}
        </PageTitle.SubHeader>
      </PageTitle>

      {/* Search and category filters */}
      <TextField
        placeholder={t('applications:onboarding.templateSelect.searchPlaceholder', 'Search types by name')}
        size="small"
        value={searchQuery}
        onChange={(e) => setSearchQuery(e.target.value)}
        fullWidth
        sx={{mb: 2}}
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={16} />
              </InputAdornment>
            ),
          },
        }}
      />

      <Stack direction="row" spacing={1} mb={2} flexWrap="wrap" useFlexGap>
        {CATEGORIES.map((cat) => {
          const isActive: boolean = selectedCategory === cat.value;
          return (
            <Chip
              key={cat.value}
              label={t(cat.titleKey)}
              onClick={() => setSelectedCategory(cat.value)}
              variant={isActive ? 'filled' : 'outlined'}
              color={isActive ? 'primary' : 'default'}
              sx={{borderRadius: '20px', cursor: 'pointer'}}
            />
          );
        })}
      </Stack>

      <Typography variant="body2" color="text.secondary" mb={2}>
        {t('applications:onboarding.templateSelect.count', 'Showing {{count}} types', {
          count: filteredTemplates.length,
        })}
      </Typography>

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
          const isDisabled: boolean = option.disabled ?? false;

          return (
            <Card
              key={option.value}
              variant="outlined"
              role="button"
              tabIndex={isDisabled ? -1 : 0}
              aria-disabled={isDisabled}
              data-testid={`template-card-${option.value}`}
              onClick={isDisabled ? undefined : () => handleTemplateSelect(option)}
              onKeyDown={(e) => {
                if (!isDisabled && (e.key === 'Enter' || e.key === ' ')) {
                  e.preventDefault();
                  handleTemplateSelect(option);
                }
              }}
              sx={{
                position: 'relative',
                cursor: isDisabled ? 'not-allowed' : 'pointer',
                opacity: isDisabled ? 0.5 : 1,
                transition: 'border-color 0.15s',
                '&:hover': isDisabled ? {} : {borderColor: 'primary.main'},
                '&:focus-visible': isDisabled ? {} : {outline: 'none', borderColor: 'primary.main'},
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
                  <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', width: 48, height: 48}}>
                    {option.icon}
                  </Box>

                  <Stack direction="column" spacing={0.75} sx={{flex: 1}}>
                    <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                      {t(option.titleKey)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                      {t(option.descriptionKey)}
                    </Typography>
                  </Stack>

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
    </PageContent>
  );
}
