// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDocsVersion} from '@docusaurus/plugin-content-docs/client';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Button, styled} from '@wso2/oxygen-ui';
import {DownloadIcon, Sparkles} from '@wso2/oxygen-ui-icons-react';
import React, {useEffect, useState} from 'react';
import type {ReactNode} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import GradientBorderButton from '@site/src/components/GradientBorderButton';

interface SdkReleaseAsset {
  downloadUrl: string;
  sizeLabel: string;
}

interface SdkReleaseEntry {
  assets: SdkReleaseAsset[];
  packageId: string;
  packageName: string | null;
}

interface SdkReleasesData {
  releases: SdkReleaseEntry[];
}

/**
 * Props for the SdkQuickstartDownload component.
 */
interface SdkQuickstartDownloadProps {
  /**
   * The `packageId` of the SDK package, as tracked in `sdk-releases.json` (e.g. `react`, `nextjs`, `express`).
   */
  packageId: string;
  /**
   * The framework logo to show in the icon badge, e.g. `<ReactLogo size={22} />`.
   */
  icon?: ReactNode;
  /**
   * Which prewritten LLM prompt to offer for copying, matching a file under
   * `content/getting-started/connect-your-application/prompts/<packageId>/<promptFlow>.txt`.
   * Omit for quickstarts with no matching prompt.
   */
  promptFlow?: 'redirect-based' | 'embedded';
}

const Callout = styled('div')({
  alignItems: 'center',
  background:
    'linear-gradient(90deg, color-mix(in srgb, var(--oxygen-palette-primary-main) 10%, transparent) 0%, transparent 70%)',
  border: '1px solid color-mix(in srgb, var(--oxygen-palette-primary-main) 24%, transparent)',
  borderRadius: '0.75rem',
  display: 'flex',
  flexWrap: 'wrap',
  gap: '1.1rem',
  marginBottom: '1.75rem',
  padding: '1.1rem 1.25rem',
});

const IconBadge = styled('span')({
  alignItems: 'center',
  background: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.12)',
  borderRadius: '0.625rem',
  display: 'inline-flex',
  flexShrink: 0,
  height: '2.375rem',
  justifyContent: 'center',
  width: '2.375rem',
});

const CardBody = styled('div')({
  display: 'flex',
  flex: '1 1 auto',
  flexDirection: 'column',
  gap: '0.25rem',
  minWidth: 0,
});

const CardTitle = styled('div')({
  color: 'var(--ifm-color-emphasis-900)',
  fontSize: '0.95rem',
  fontWeight: 600,
});

const CardMeta = styled('div')({
  alignItems: 'center',
  color: 'var(--ifm-color-emphasis-600)',
  display: 'flex',
  flexWrap: 'wrap',
  fontFamily: 'var(--ifm-font-family-monospace)',
  fontSize: '0.75rem',
  gap: '0.5rem',
});

const Actions = styled('div')({
  alignItems: 'center',
  display: 'flex',
  flexShrink: 0,
  flexWrap: 'wrap',
  gap: '0.75rem',
});

/**
 * Renders a callout linking to the latest quickstart sample archive for the given SDK package, sourced
 * from `/data/sdk-releases.json`. Renders nothing if no sample has been published yet for this package.
 */
export default function SdkQuickstartDownload({
  packageId,
  icon = undefined,
  promptFlow = undefined,
}: SdkQuickstartDownloadProps): React.ReactElement | null {
  const {withBaseUrl} = useBaseUrlUtils();
  const {version} = useDocsVersion();
  const {siteConfig} = useDocusaurusContext();
  const [asset, setAsset] = useState<SdkReleaseAsset | null>(null);
  const [packageName, setPackageName] = useState('');
  const [copyState, setCopyState] = useState<'idle' | 'copying' | 'copied'>('idle');

  useEffect(() => {
    const controller = new AbortController();
    fetch(withBaseUrl('/data/sdk-releases.json'), {signal: controller.signal})
      .then((r) => r.json() as Promise<SdkReleasesData>)
      .then((data) => {
        const entry = data.releases?.find((release) => release.packageId === packageId);
        const match = entry?.assets.find((a) => a.downloadUrl.endsWith('.zip'));
        if (!entry || !match) return;
        setPackageName(entry.packageName ?? '');
        setAsset(match);
      })
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') return;
      });
    return () => controller.abort();
  }, [withBaseUrl, packageId]);

  const handleCopyPrompt = async (): Promise<void> => {
    if (!promptFlow || copyState === 'copying') return;
    setCopyState('copying');
    try {
      // Docusaurus "current" version (labeled "Next") -> 'next', matching the directory
      // convention generate-prompts.mjs mirrors these files under (static/docs/<versionPath>/...).
      const versionPath = version === 'current' ? 'next' : version;
      const res = await fetch(
        withBaseUrl(
          `/docs/${versionPath}/getting-started/connect-your-application/prompts/${packageId}/${promptFlow}.txt`,
        ),
      );
      const text = await res.text();
      const config = siteConfig.customFields?.product as DocusaurusProductConfig;
      const filled = text
        .replace(/\{\{productName\}\}/g, config.project.name)
        .replace(/\{\{clientId\}\}/g, '<your-client-id>');
      await navigator.clipboard.writeText(filled);
      setCopyState('copied');
      setTimeout(() => setCopyState('idle'), 1800);
    } catch {
      setCopyState('idle');
    }
  };

  if (!asset) return null;

  return (
    <Callout>
      <IconBadge aria-hidden="true">{icon}</IconBadge>
      <CardBody>
        <CardTitle>Skip the setup, run the sample app instead</CardTitle>
        <CardMeta>
          <span>{asset.sizeLabel}</span>
          {packageName && (
            <>
              <span aria-hidden="true">&middot;</span>
              <span>
                prewired with <code>{packageName}</code>
              </span>
            </>
          )}
        </CardMeta>
      </CardBody>
      <Actions>
        <Button
          variant="outlined"
          href={asset.downloadUrl}
          target="_blank"
          rel="noreferrer"
          endIcon={<DownloadIcon size={16} />}
          sx={{
            borderRadius: '999px',
            flexShrink: 0,
            fontWeight: 600,
            px: 2.5,
            py: 1,
            textTransform: 'none',
          }}
        >
          Download app
        </Button>
        {promptFlow && (
          <GradientBorderButton
            onClick={() => void handleCopyPrompt()}
            disabled={copyState === 'copying'}
            startIcon={<Sparkles size={14} />}
          >
            {copyState === 'copied' ? 'Copied!' : 'Copy prompt'}
          </GradientBorderButton>
        )}
      </Actions>
    </Callout>
  );
}
