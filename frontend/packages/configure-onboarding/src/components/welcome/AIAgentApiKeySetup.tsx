// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Divider, IconButton, Link, Stack, Typography} from '@wso2/oxygen-ui';
import {Bot, Check, Copy, ExternalLink, FilePen} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import useWayfinderReleases from '../../api/useWayfinderReleases';

const ASSET_PATTERN = /^sample-app-wayfinder-[0-9A-Za-z.+-]+\.zip$/i;

const ENV_LINES = [
  {text: '# --- LLM Provider ---', comment: true},
  {text: '# Which LLM to use: "anthropic" (default) or "google".', comment: true},
  {text: 'LLM_PROVIDER=', key: 'LLM_PROVIDER', value: 'anthropic'},
  {text: ''},
  {text: '# LLM API key obtained from your LLM provider.', comment: true},
  {text: 'LLM_API_KEY=', key: 'LLM_API_KEY', value: 'your-llm-api-key'},
  {text: ''},
  {
    text: '# Model name override. Defaults to claude-sonnet-4-6 (Anthropic)',
    comment: true,
  },
  {
    text: '# or gemini-3.1-flash-lite (Gemini).',
    comment: true,
  },
  {text: '# MODEL_NAME=', comment: true},
] as const;

const ENV_COPY_TEXT = [
  '# --- LLM Provider ---',
  '# Which LLM to use: "anthropic" (default) or "google".',
  'LLM_PROVIDER=anthropic',
  '',
  '# LLM API key obtained from your LLM provider.',
  'LLM_API_KEY=your-llm-api-key',
  '',
  '# Model name override. Defaults to claude-sonnet-4-6 (Anthropic) or gemini-3.1-flash-lite (Gemini).',
  '# MODEL_NAME=',
].join('\n');

function EnvFileBlock({folderPrefix}: {folderPrefix: string | null}): JSX.Element {
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    void navigator.clipboard.writeText(ENV_COPY_TEXT).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden', fontSize: '0.8rem'}}>
      <Box
        sx={{
          px: 2,
          py: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid',
          borderColor: 'divider',
          bgcolor: 'action.selected',
        }}
      >
        <Typography variant="caption" fontFamily="monospace" color="text.secondary">
          {folderPrefix ? `${folderPrefix}/ai-agent/.env` : 'ai-agent/.env'}
        </Typography>
        <IconButton
          size="small"
          aria-label="Copy snippet"
          onClick={handleCopy}
          sx={{color: copied ? 'success.main' : 'text.secondary'}}
        >
          {copied ? <Check size={13} /> : <Copy size={13} />}
        </IconButton>
      </Box>
      <Box sx={{px: 2, py: 1.5, bgcolor: 'background.paper', overflowX: 'auto'}}>
        {ENV_LINES.map((line, i) => (
          <Box
            // eslint-disable-next-line react/no-array-index-key
            key={i}
            component="pre"
            sx={{m: 0, fontFamily: 'monospace', fontSize: '0.8rem', lineHeight: 1.6, whiteSpace: 'pre'}}
          >
            {line.text === '' ? (
              ' '
            ) : 'comment' in line && line.comment ? (
              <Box component="span" sx={{color: 'text.disabled'}}>
                {line.text}
              </Box>
            ) : 'key' in line ? (
              <>
                <Box component="span" sx={{color: 'primary.main', fontWeight: 600}}>
                  {line.key}
                </Box>
                {'='}
                <Box component="span" sx={{color: 'success.main'}}>
                  {line.value}
                </Box>
              </>
            ) : (
              line.text
            )}
          </Box>
        ))}
      </Box>
    </Box>
  );
}

export default function AIAgentApiKeySetup({releasesUrl = ''}: {releasesUrl?: string}): JSX.Element {
  const {t} = useTranslation(['common']);
  const {data} = useWayfinderReleases(releasesUrl);
  const folderPrefix = useMemo(() => {
    if (!data) return null;
    const release = data.latestRelease ?? data.releases?.[0] ?? null;
    const asset = release?.assets.find((a) => ASSET_PATTERN.test(a.name));
    return asset ? asset.name.replace(/\.zip$/i, '') : null;
  }, [data]);

  return (
    <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}>
      {/* Header */}
      <Box
        sx={{
          p: 2.5,
          display: 'flex',
          alignItems: 'center',
          gap: 2,
          bgcolor: 'action.selected',
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Box
          sx={{
            width: 40,
            height: 40,
            borderRadius: 2,
            bgcolor: 'background.paper',
            color: 'text.secondary',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <Bot size={22} />
        </Box>
        <Typography variant="subtitle2" fontWeight={600}>
          {t('common:welcome.aiAgentsTryout.steps.configureSample.title')}
        </Typography>
      </Box>

      {/* Steps */}
      <Stack divider={<Divider />}>
        {/* Obtain an API key */}
        <Box sx={{p: 2.5}}>
          <Stack direction="row" spacing={2} alignItems="flex-start">
            <Box
              sx={{
                width: 24,
                height: 24,
                borderRadius: '50%',
                bgcolor: 'action.selected',
                color: 'text.primary',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '0.75rem',
                fontWeight: 700,
                flexShrink: 0,
                mt: 0.25,
              }}
            >
              1
            </Box>
            <Box sx={{flex: 1}}>
              <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 0.5}}>
                <Box sx={{color: 'text.secondary', display: 'flex'}}>
                  <ExternalLink size={16} />
                </Box>
                <Typography variant="subtitle2" fontWeight={600}>
                  {t('common:welcome.aiAgentsTryout.apiKeySetup.getKey.title')}
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={{mb: 1}}>
                {t('common:welcome.aiAgentsTryout.apiKeySetup.getKey.description')}
              </Typography>
              <Stack direction="row" spacing={2}>
                <Link
                  href="https://console.anthropic.com/settings/keys"
                  target="_blank"
                  rel="noopener noreferrer"
                  variant="body2"
                  sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5}}
                >
                  Anthropic Console
                  <ExternalLink size={12} />
                </Link>
                <Link
                  href="https://aistudio.google.com/apikey"
                  target="_blank"
                  rel="noopener noreferrer"
                  variant="body2"
                  sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5}}
                >
                  Google AI Studio
                  <ExternalLink size={12} />
                </Link>
              </Stack>
            </Box>
          </Stack>
        </Box>

        {/* Set API key in env */}
        <Box sx={{p: 2.5}}>
          <Stack direction="row" spacing={2} alignItems="flex-start">
            <Box
              sx={{
                width: 24,
                height: 24,
                borderRadius: '50%',
                bgcolor: 'action.selected',
                color: 'text.primary',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '0.75rem',
                fontWeight: 700,
                flexShrink: 0,
                mt: 0.25,
              }}
            >
              2
            </Box>
            <Box sx={{flex: 1}}>
              <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 0.5}}>
                <Box sx={{color: 'text.secondary', display: 'flex'}}>
                  <FilePen size={16} />
                </Box>
                <Typography variant="subtitle2" fontWeight={600}>
                  {t('common:welcome.aiAgentsTryout.apiKeySetup.setKey.title')}
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                {t('common:welcome.aiAgentsTryout.apiKeySetup.setKey.description')}
              </Typography>
              <EnvFileBlock folderPrefix={folderPrefix} />
            </Box>
          </Stack>
        </Box>
      </Stack>
    </Box>
  );
}
