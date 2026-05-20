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
import type {JSX} from 'react';
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
  BASIC_AUTH: ['assets/images/icons/password.svg'],
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
  SELF_INVITE: ['assets/images/icons/email.svg'],
  Email_Link: ['assets/images/icons/email.svg'],
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
            gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
            gap: 1.5,
          }}
        >
          {filteredTemplates.map((template) => {
            const isSelected =
              selectedTemplate?.type === template.type && selectedTemplate?.flowType === template.flowType;
            return (
              <Card
                key={`${template.flowType}-${template.type}`}
                variant="outlined"
                sx={{
                  display: 'flex',
                  flexDirection: 'column',
                  borderRadius: 2,
                }}
              >
                <CardActionArea
                  onClick={() => onTemplateChange(template)}
                  sx={{
                    flex: 1,
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'stretch',
                    borderWidth: 2,
                    borderStyle: 'solid',
                    borderColor: isSelected ? 'primary.main' : 'transparent',
                    borderRadius: 2,
                    transition: 'all 0.15s ease-in-out',
                    '&:hover': {
                      borderColor: isSelected ? 'primary.main' : 'divider',
                      bgcolor: isSelected ? 'action.selected' : 'action.hover',
                    },
                  }}
                >
                  <CardContent
                    sx={{
                      py: 2,
                      px: 2,
                      flex: 1,
                      display: 'flex',
                      flexDirection: 'column',
                      '&:last-child': {pb: 2},
                    }}
                  >
                    <Box sx={{mb: 1, display: 'flex', alignItems: 'center', gap: 0.5}}>
                      {TEMPLATE_ICONS[template.type] ? (
                        TEMPLATE_ICONS[template.type].map((icon, idx) => (
                          <Box key={icon} sx={{display: 'flex', alignItems: 'center', gap: 0.5}}>
                            {idx > 0 && (
                              <Typography variant="caption" color="text.disabled" sx={{fontSize: '0.6rem', mx: 0.25}}>
                                +
                              </Typography>
                            )}
                            <img
                              src={resolveStaticResourcePath(icon)}
                              alt=""
                              width={18}
                              height={18}
                              style={
                                effectiveMode === 'dark' && !BRANDED_ICONS.has(icon)
                                  ? {filter: 'brightness(0.9) invert(1)'}
                                  : undefined
                              }
                            />
                          </Box>
                        ))
                      ) : (
                        <Lock size={18} />
                      )}
                    </Box>
                    <Typography variant="subtitle2" sx={{fontWeight: 600}}>
                      {template.display.label}
                    </Typography>
                    {template.display.description && (
                      <Typography variant="caption" color="text.secondary" sx={{mt: 0.5, lineHeight: 1.4}}>
                        {template.display.description}
                      </Typography>
                    )}
                  </CardContent>
                </CardActionArea>
              </Card>
            );
          })}
        </Box>
      )}
    </Stack>
  );
}
