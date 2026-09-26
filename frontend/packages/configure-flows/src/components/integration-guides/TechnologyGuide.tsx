// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GradientBorderButton} from '@thunderid/components';
import {
  getIntegrationGuideVariantKey,
  resolveTemplateLink,
  type IntegrationGuides,
} from '@thunderid/configure-applications';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger';
import {Box, Typography, Stack, Card, CardContent, Tooltip} from '@wso2/oxygen-ui';
import {Sparkles, Copy} from '@wso2/oxygen-ui-icons-react';
import type {JSX, MouseEvent} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link TechnologyGuide} component.
 *
 * @public
 */
export interface TechnologyGuideProps {
  /**
   * Integration guides structure containing the LLM prompt guide
   */
  guides: IntegrationGuides | null;
  /**
   * The template ID used to create the application (e.g., 'react', 'react-embedded')
   */
  templateId?: string | null;
  /**
   * The OAuth2 client ID to replace {{clientId}} placeholders
   */
  clientId?: string;
  /**
   * The application ID to replace {{applicationId}} placeholders
   */
  applicationId?: string;
}

/**
 * React component that displays the integration guide option for technology templates.
 *
 * This component renders the LLM prompt option as a clickable card.
 *
 * The displayed guide varies based on the template ID:
 * - Templates with '-embedded' suffix (e.g., 'react-embedded'): Shows 'embedded' guide for custom login UI
 * - Templates without '-embedded' suffix (e.g., 'react'): Shows 'redirect_based' guide for product's hosted login
 *
 * @param props - The component props
 * @param props.guides - Integration guides structure
 * @param props.templateId - The template ID used to create the application
 *
 * @returns JSX element displaying the integration guide option
 *
 * @public
 */
export default function TechnologyGuide({
  guides,
  templateId = null,
  clientId = '',
  applicationId = '',
}: TechnologyGuideProps): JSX.Element | null {
  const logger = useLogger('TechnologyGuide');
  const {t} = useTranslation();
  const {config, getDocumentationLink} = useConfig();
  const productName = config.brand.product_name;

  const [copiedPrompt, setCopiedPrompt] = useState<boolean>(false);
  const [isFetchingPrompt, setIsFetchingPrompt] = useState<boolean>(false);

  if (!guides) {
    return null;
  }

  // Select the guide for the variant determined by the template ID.
  // Templates with the -embedded suffix (e.g., react-embedded) use the EMBEDDED guide;
  // all others use the REDIRECT_BASED guide.
  const selectedGuide = guides[getIntegrationGuideVariantKey(templateId)];

  if (!selectedGuide) {
    return null;
  }

  const {llm_prompt: llmPrompt} = selectedGuide;
  const promptUrl = resolveTemplateLink(llmPrompt.docsUrl, getDocumentationLink);

  const replacePlaceholders = (text: string): string => {
    let result = text;

    if (productName && productName.trim() !== '') {
      result = result.replace(/\{\{productName\}\}/g, productName);
    }

    // Replace clientId if available
    if (clientId && clientId.trim() !== '') {
      result = result.replace(/\{\{clientId\}\}/g, clientId);
    }

    // Replace applicationId if available
    if (applicationId && applicationId.trim() !== '') {
      result = result.replace(/\{\{applicationId\}\}/g, applicationId);
    }

    return result;
  };

  const copyText = async (text: string): Promise<void> => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedPrompt(true);
      setTimeout(() => setCopiedPrompt(false), 2000);
    } catch {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');

      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.opacity = '0';
      document.body.appendChild(textArea);
      textArea.select();

      try {
        document.execCommand('copy');
        setCopiedPrompt(true);
        setTimeout(() => setCopiedPrompt(false), 2000);
      } catch {
        logger.error('Failed to copy the prompt to clipboard.');
      }

      document.body.removeChild(textArea);
    }
  };

  const handleCopyPrompt = async (e: MouseEvent): Promise<void> => {
    e.stopPropagation();
    if (!promptUrl) return;

    setIsFetchingPrompt(true);

    try {
      const response = await fetch(promptUrl);

      if (!response.ok) {
        throw new Error(`Failed to fetch prompt: ${response.status}`);
      }

      const promptText = await response.text();

      await copyText(replacePlaceholders(promptText));
    } catch (error) {
      logger.error('Failed to fetch the integration prompt.', {error});
    } finally {
      setIsFetchingPrompt(false);
    }
  };

  return (
    <Stack direction="column" spacing={3} sx={{width: '100%'}}>
      {/* LLM Prompt Option */}
      {llmPrompt && (
        <Card variant="outlined">
          <CardContent sx={{p: 3}}>
            <Stack direction="row" spacing={2} alignItems="center">
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 48,
                  height: 48,
                  borderRadius: 1,
                  color: 'primary.main',
                }}
              >
                <Sparkles size={24} />
              </Box>
              <Box sx={{flex: 1}}>
                <Typography variant="subtitle1" sx={{mb: 0.5, fontWeight: 600}}>
                  {t('applications:edit.overview.agentPrompt.title', 'Integrate with a coding agent')}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {t(
                    'applications:edit.overview.agentPrompt.description',
                    'Copy a ready-made prompt for Claude, Cursor, or any agent.',
                  )}
                </Typography>
              </Box>
              {promptUrl && (
                <Tooltip title={copiedPrompt ? t('applications:clientSecret.copied') : ''} open={copiedPrompt} arrow>
                  <GradientBorderButton
                    data-testid="copy-prompt-button"
                    disabled={isFetchingPrompt}
                    onClick={(e) => {
                      handleCopyPrompt(e).catch(() => {
                        /* Error already handled */
                      });
                    }}
                    startIcon={<Copy size={16} />}
                  >
                    Copy Prompt
                  </GradientBorderButton>
                </Tooltip>
              )}
            </Stack>
          </CardContent>
        </Card>
      )}
    </Stack>
  );
}
