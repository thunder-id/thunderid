// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GithubIcon, GoogleIcon, ResourceAvatar} from '@thunderid/components';
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  InputAdornment,
  Stack,
  TextField,
  Typography,
  useColorScheme,
} from '@wso2/oxygen-ui';
import {Lock, Plus, Search} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import useGetFlowsMeta from '../../api/useGetFlowsMeta';
import type {FlowType} from '../../models/flows';
import type {FlowTemplate} from '../../models/templates';
import resolveStaticResourcePath from '../../utils/resolveStaticResourcePath';

interface SelectFlowTemplateProps {
  flowType: FlowType;
  selectedTemplate: FlowTemplate | null;
  onTemplateChange: (template: FlowTemplate) => void;
}

const CATEGORY_ORDER = ['PASSWORD', 'SOCIAL_LOGIN', 'MFA', 'PASSWORDLESS'];

const CATEGORY_LABELS: Record<string, string> = {
  PASSWORD: 'Password',
  SOCIAL_LOGIN: 'Social Login',
  MFA: 'Multi-Factor',
  PASSWORDLESS: 'Passwordless',
};

// Icons that use brand colors and should not be inverted in dark mode
const BRANDED_ICONS = new Set(['assets/images/icons/google.svg']);

const TEMPLATE_ICONS: Record<string, string[]> = {
  CREDENTIALS_AUTH: ['assets/images/icons/password.svg'],
  GOOGLE: ['assets/images/icons/google.svg'],
  GITHUB: ['assets/images/icons/github.svg'],
  GOOGLE_GITHUB: ['assets/images/icons/google.svg', 'assets/images/icons/github.svg'],
  BASIC_GOOGLE: ['assets/images/icons/password.svg', 'assets/images/icons/google.svg'],
  BASIC_GITHUB: ['assets/images/icons/password.svg', 'assets/images/icons/github.svg'],
  BASIC_GOOGLE_GITHUB: [
    'assets/images/icons/password.svg',
    'assets/images/icons/google.svg',
    'assets/images/icons/github.svg',
  ],
  BASIC_GOOGLE_GITHUB_SMS: [
    'assets/images/icons/password.svg',
    'assets/images/icons/google.svg',
    'assets/images/icons/github.svg',
    'assets/images/icons/mobile-message.svg',
  ],
  SMS_OTP: ['assets/images/icons/mobile-message.svg'],
  PASSKEY: ['assets/images/icons/fingerprint.svg'],
  BASIC_PASSKEY: ['assets/images/icons/password.svg', 'assets/images/icons/fingerprint.svg'],
  BASIC: ['assets/images/icons/password.svg'],
  BASIC_WITH_PROMPT: ['assets/images/icons/password.svg', 'assets/images/icons/form.svg'],
  BASIC_WITH_CONSENT: ['assets/images/icons/password.svg', 'assets/images/icons/stamp.svg'],
  EMAIL_SELF_INVITE: ['assets/images/icons/email.svg'],
  SMS_SELF_INVITE: ['assets/images/icons/mobile-message.svg'],
  Email_Link: ['assets/images/icons/email.svg'],
  MAGIC_LINK: ['assets/images/icons/link.svg'],
  BASIC_MAGIC_LINK: ['assets/images/icons/password.svg', 'assets/images/icons/link.svg'],
  CIBA_EMAIL_NOTIFICATION: ['assets/images/icons/email-down.svg'],
  DISAMBIGUATION_LOGIN: ['assets/images/icons/magnifying-glass.svg'],
  DEFAULT: ['assets/images/icons/user.svg'],
};

export default function SelectFlowTemplate({
  flowType,
  selectedTemplate,
  onTemplateChange,
}: SelectFlowTemplateProps): JSX.Element {
  const {t} = useTranslation();
  const {data} = useGetFlowsMeta({flowType});
  const templates = data.templates;

  const {mode, systemMode} = useColorScheme();
  const effectiveMode = mode === 'system' ? systemMode : mode;

  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [prevFlowType, setPrevFlowType] = useState(flowType);

  // Reset filters when flow type changes to avoid stale "no results" state
  if (prevFlowType !== flowType) {
    setPrevFlowType(flowType);
    setSelectedCategory(null);
    setSearchQuery('');
  }

  const blankTemplate = useMemo(() => templates.find((tmpl) => tmpl.type === 'BLANK'), [templates]);
  const nonBlankTemplates = useMemo(() => templates.filter((tmpl) => tmpl.type !== 'BLANK'), [templates]);

  useEffect(() => {
    if (!selectedTemplate && templates.length > 0) {
      onTemplateChange(templates[0]);
    }
  }, [templates, selectedTemplate, onTemplateChange]);

  const categories = useMemo(() => {
    const present = new Set(nonBlankTemplates.map((template) => template.category));
    return CATEGORY_ORDER.filter((cat) => present.has(cat));
  }, [nonBlankTemplates]);

  const filteredTemplates = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return nonBlankTemplates.filter((template) => {
      if (selectedCategory && template.category !== selectedCategory) return false;
      if (query) {
        const inLabel = template.display.label.toLowerCase().includes(query);
        const inDescription = template.display.description?.toLowerCase().includes(query) ?? false;
        return inLabel || inDescription;
      }
      return true;
    });
  }, [nonBlankTemplates, selectedCategory, searchQuery]);

  const isBlankSelected = selectedTemplate?.type === 'BLANK' && selectedTemplate?.flowType === flowType;

  /** A brand-icon component when the path is one of the vendor marks used elsewhere in the console, else `null`. */
  const brandIcon = (path: string, size: number): ReactNode => {
    if (path === 'assets/images/icons/google.svg') return <GoogleIcon size={size} />;
    if (path === 'assets/images/icons/github.svg') return <GithubIcon size={size} />;
    return null;
  };

  const renderIcon = (path: string, size: number): ReactNode =>
    brandIcon(path, size) ?? (
      <img
        src={resolveStaticResourcePath(path)}
        alt=""
        width={size}
        height={size}
        style={effectiveMode === 'dark' && !BRANDED_ICONS.has(path) ? {filter: 'brightness(0.9) invert(1)'} : undefined}
      />
    );

  /**
   * The icon area for a template card, sized and aligned like the application-type gallery's icon
   * slot. Templates with a single icon go through {@link ResourceAvatar} (transparent, so brand
   * marks and asset icons alike sit directly on the card); composites with several icons (e.g.
   * "Google + GitHub") render as a plain row instead, since ResourceAvatar has no notion of more
   * than one icon.
   */
  const renderTemplateIcon = (type: string): JSX.Element => {
    const icons = TEMPLATE_ICONS[type];
    if (!icons) {
      return <ResourceAvatar transparent variant="rounded" size={48} fallback={<Lock size={24} />} />;
    }
    if (icons.length === 1) {
      return <ResourceAvatar transparent variant="rounded" size={48} fallback={renderIcon(icons[0], 30)} />;
    }
    return (
      <Box sx={{width: 48, height: 48, display: 'flex', alignItems: 'center', gap: 0.5}}>
        {icons.map((icon, idx) => (
          <Box key={icon} sx={{display: 'flex', alignItems: 'center', gap: 0.5}}>
            {idx > 0 && (
              <Typography variant="caption" color="text.disabled" sx={{fontSize: '0.6rem', mx: 0.25}}>
                +
              </Typography>
            )}
            {renderIcon(icon, 18)}
          </Box>
        ))}
      </Box>
    );
  };

  return (
    <Stack direction="column" spacing={3} data-testid="select-flow-template">
      <Typography variant="h1">{t('flows:create.template.title', 'Choose a starting template')}</Typography>

      {/* Start from Scratch */}
      {blankTemplate && (
        <Card
          variant="outlined"
          sx={{
            borderWidth: 2,
            borderStyle: isBlankSelected ? 'solid' : 'dashed',
            borderColor: isBlankSelected ? 'primary.main' : 'divider',
            borderRadius: 2,
            bgcolor: isBlankSelected ? 'action.selected' : 'transparent',
            transition: 'all 0.15s ease-in-out',
            '&:hover': {
              borderColor: 'primary.main',
              bgcolor: isBlankSelected ? 'action.selected' : 'action.hover',
            },
          }}
        >
          <CardActionArea onClick={() => onTemplateChange(blankTemplate)}>
            <CardContent sx={{py: 2, px: 2.5}}>
              <Stack direction="row" spacing={2} alignItems="center">
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 36,
                    height: 36,
                    borderRadius: 1.5,
                    bgcolor: isBlankSelected ? 'primary.main' : 'action.hover',
                    color: isBlankSelected ? 'primary.contrastText' : 'text.secondary',
                    transition: 'all 0.15s ease-in-out',
                  }}
                >
                  <Plus size={18} />
                </Box>
                <Box>
                  <Typography variant="subtitle2" sx={{fontWeight: 600}}>
                    {t('flows:create.template.blank.title', 'Start from scratch')}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {t(
                      'flows:create.template.blank.description',
                      'Build your flow from the ground up with an empty canvas',
                    )}
                  </Typography>
                </Box>
              </Stack>
            </CardContent>
          </CardActionArea>
        </Card>
      )}

      {/* Search + category filters */}
      <Stack direction="row" spacing={2} alignItems="center" sx={{flexWrap: 'wrap', gap: 1}}>
        <TextField
          size="small"
          placeholder={t('flows:create.template.search', 'Search templates...')}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          sx={{width: 240, flexShrink: 0}}
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
        <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 0.75}}>
          <Chip
            label={t('common:all', 'All')}
            onClick={() => setSelectedCategory(null)}
            color={selectedCategory === null ? 'primary' : 'default'}
            variant={selectedCategory === null ? 'filled' : 'outlined'}
            size="small"
          />
          {categories.map((cat) => (
            <Chip
              key={cat}
              label={CATEGORY_LABELS[cat] ?? cat}
              onClick={() => setSelectedCategory(cat)}
              color={selectedCategory === cat ? 'primary' : 'default'}
              variant={selectedCategory === cat ? 'filled' : 'outlined'}
              size="small"
            />
          ))}
        </Box>
      </Stack>

      {/* Template grid */}
      {filteredTemplates.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          {t('flows:create.template.noResults', 'No templates match your search.')}
        </Typography>
      ) : (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {
              xs: '1fr',
              sm: 'repeat(2, 1fr)',
              md: 'repeat(3, 1fr)',
              lg: 'repeat(4, 1fr)',
              xl: 'repeat(6, 1fr)',
            },
            gap: 2,
          }}
        >
          {filteredTemplates.map((template) => {
            const isSelected =
              selectedTemplate?.type === template.type && selectedTemplate?.flowType === template.flowType;
            return (
              <Card
                key={`${template.flowType}-${template.type}`}
                variant="outlined"
                role="button"
                tabIndex={0}
                aria-pressed={isSelected}
                onClick={() => onTemplateChange(template)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onTemplateChange(template);
                  }
                }}
                sx={{
                  cursor: 'pointer',
                  borderColor: isSelected ? 'primary.main' : 'divider',
                  bgcolor: isSelected ? 'action.selected' : undefined,
                  transition: 'border-color 0.15s',
                  '&:hover': {borderColor: 'primary.main'},
                  '&:focus-visible': {outline: 'none', borderColor: 'primary.main'},
                }}
              >
                <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
                  <Stack direction="column" spacing={2}>
                    {renderTemplateIcon(template.type)}

                    <Stack direction="column" spacing={0.75}>
                      <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                        {template.display.label}
                      </Typography>
                      {template.display.description && (
                        <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                          {template.display.description}
                        </Typography>
                      )}
                    </Stack>

                    <Typography variant="caption" color="text.disabled" sx={{fontWeight: 500, letterSpacing: 0.2}}>
                      {`#${(CATEGORY_LABELS[template.category] ?? template.category).toLowerCase()}`}
                    </Typography>
                  </Stack>
                </CardContent>
              </Card>
            );
          })}
        </Box>
      )}
    </Stack>
  );
}
