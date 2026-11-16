// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  CircularProgress,
  Link,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Paper,
} from '@wso2/oxygen-ui';
import {Download} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import GuideCode from './GuideCode';
import GuideStepList from './GuideStepList';
import useGetQuickstartGuide from '../../../api/useGetQuickstartGuide';
import useGetSampleRelease from '../../../api/useGetSampleRelease';
import type {
  QuickstartGuideAlertKind,
  QuickstartGuideBlock,
  QuickstartGuideSample,
  QuickstartGuideStep,
} from '../../../utils/parseQuickstartGuide';

/**
 * A quickstart guide the {@link QuickstartGuide} component can show.
 *
 * @public
 */
export interface QuickstartGuideSource {
  /**
   * The platform or technology the guide covers (e.g. "React")
   */
  label: string;
  /**
   * The URL of the guide on the docs site
   */
  docsUrl: string;
  /**
   * The platform's logo
   */
  icon?: ReactNode;
  /**
   * The URL of the platform's coding agent prompt, for a template that covers several platforms
   */
  promptUrl?: string;
  /**
   * The anchor of the section of the guide that holds the steps, when it is not the whole page
   */
  section?: string;
}

/**
 * Props for the {@link QuickstartGuide} component.
 *
 * @public
 */
export interface QuickstartGuideProps {
  /**
   * The guide to show
   */
  guide: QuickstartGuideSource;
  /**
   * Placeholder text in the docs code samples, mapped to this application's value for it
   */
  codeValues: Record<string, string | undefined>;
  /**
   * Opens an external URL, after whatever confirmation the host page wants
   */
  onOpenLink: (url: string) => void;
  /**
   * What to show: the steps that build the app (the default), or the sample app that runs instead
   */
  view?: 'steps' | 'sample';
}

/** What every block of a guide needs to render, passed down unchanged through nested blocks. */
interface GuideRenderContext {
  codeValues: Record<string, string | undefined>;
  onOpenLink: (url: string) => void;
  /** The code group option (e.g. a package manager) last picked, applied to every group. */
  codeOption: string | undefined;
  onCodeOptionChange: (label: string) => void;
}

const ALERT_SEVERITY: Record<QuickstartGuideAlertKind, 'info' | 'success' | 'warning' | 'error'> = {
  note: 'info',
  tip: 'success',
  important: 'info',
  warning: 'warning',
  caution: 'error',
};

const INLINE_MARKDOWN = /`([^`]+)`|\*\*([^*]+)\*\*|\[([^\]]+)\]\((https?:[^)\s]+)\)/g;

/** Renders the inline Markdown of a guide: code spans, bold text, and links. */
function GuideText({text, onOpenLink}: {text: string; onOpenLink: (url: string) => void}): ReactNode {
  const nodes: ReactNode[] = [];
  let position = 0;

  Array.from(text.matchAll(INLINE_MARKDOWN)).forEach((match) => {
    const [whole, code, bold, linkText, linkUrl] = match;
    if (match.index > position) {
      nodes.push(text.slice(position, match.index));
    }
    if (code !== undefined) {
      nodes.push(
        <Box
          key={match.index}
          component="code"
          sx={{fontFamily: 'monospace', fontSize: '0.9em', bgcolor: 'action.hover', borderRadius: 0.5, px: 0.5}}
        >
          {code}
        </Box>,
      );
    } else if (bold !== undefined) {
      nodes.push(
        <Box key={match.index} component="strong">
          {bold}
        </Box>,
      );
    } else {
      nodes.push(
        <Link
          key={match.index}
          component="button"
          type="button"
          underline="hover"
          sx={{font: 'inherit', verticalAlign: 'baseline', textAlign: 'start'}}
          // The docs export links to the Markdown export of other pages. A person wants the page.
          onClick={() => onOpenLink(linkUrl.replace(/\.md(?=$|#)/, ''))}
        >
          {linkText.replace(/`/g, '')}
        </Link>,
      );
    }
    position = match.index + whole.length;
  });

  if (position < text.length) {
    nodes.push(text.slice(position));
  }

  return nodes;
}

/** Renders the blocks of a guide step, or of a list item or alert nested inside one. */
function GuideBlocks({blocks, context}: {blocks: QuickstartGuideBlock[]; context: GuideRenderContext}): JSX.Element {
  return (
    <Stack spacing={1.5} sx={{minWidth: 0}}>
      {blocks.map((block, index) => {
        // Blocks come from a parsed document that never reorders, so the position is a stable key.
        const key = `${block.type}-${index}`;

        switch (block.type) {
          case 'paragraph':
            return (
              <Typography key={key} variant="body2">
                <GuideText text={block.text} onOpenLink={context.onOpenLink} />
              </Typography>
            );
          case 'heading':
            return (
              <Typography key={key} variant="subtitle2" component="h4">
                <GuideText text={block.text} onOpenLink={context.onOpenLink} />
              </Typography>
            );
          case 'code':
            return (
              <GuideCode
                key={key}
                title={block.title}
                options={[block]}
                codeValues={context.codeValues}
                codeOption={context.codeOption}
                onCodeOptionChange={context.onCodeOptionChange}
              />
            );
          case 'codeGroup':
            return (
              <GuideCode
                key={key}
                options={block.options}
                codeValues={context.codeValues}
                codeOption={context.codeOption}
                onCodeOptionChange={context.onCodeOptionChange}
              />
            );
          case 'list':
            return (
              <Stack key={key} component={block.ordered ? 'ol' : 'ul'} spacing={1} sx={{m: 0, pl: 3}}>
                {block.items.map((item, itemIndex) => (
                  // eslint-disable-next-line react/no-array-index-key
                  <Box key={itemIndex} component="li" sx={{typography: 'body2'}}>
                    <GuideBlocks blocks={item} context={context} />
                  </Box>
                ))}
              </Stack>
            );
          case 'alert':
            return (
              <Alert key={key} severity={ALERT_SEVERITY[block.kind]} sx={{'& .MuiAlert-message': {minWidth: 0}}}>
                {block.title && <AlertTitle>{block.title}</AlertTitle>}
                <GuideBlocks blocks={block.blocks} context={context} />
              </Alert>
            );
          default:
            return (
              <TableContainer key={key}>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      {block.header.map((cell) => (
                        <TableCell key={cell}>
                          <GuideText text={cell} onOpenLink={context.onOpenLink} />
                        </TableCell>
                      ))}
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {block.rows.map((row) => (
                      <TableRow key={row.join('|')}>
                        {row.map((cell, cellIndex) => (
                          // eslint-disable-next-line react/no-array-index-key
                          <TableCell key={cellIndex}>
                            <GuideText text={cell} onOpenLink={context.onOpenLink} />
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            );
        }
      })}
    </Stack>
  );
}

/** The steps of one guide, all open. */
function GuideSteps({
  steps,
  codeValues,
  onOpenLink,
}: {
  steps: QuickstartGuideStep[];
  codeValues: Record<string, string | undefined>;
  onOpenLink: (url: string) => void;
}): JSX.Element {
  const [codeOption, setCodeOption] = useState<string | undefined>(undefined);
  const context: GuideRenderContext = {codeValues, onOpenLink, codeOption, onCodeOptionChange: setCodeOption};

  return (
    <GuideStepList
      steps={steps.map((step) => ({
        key: step.title,
        title: <GuideText text={step.title} onOpenLink={onOpenLink} />,
        content: <GuideBlocks blocks={step.blocks} context={context} />,
      }))}
    />
  );
}

/** The guide's sample app: its released archive to download, then how the guide says to run it. */
function GuideSample({
  guide,
  sample,
  codeValues,
  onOpenLink,
}: {
  guide: QuickstartGuideSource;
  sample: QuickstartGuideSample;
  codeValues: Record<string, string | undefined>;
  onOpenLink: (url: string) => void;
}): JSX.Element {
  const {t} = useTranslation();
  const {config} = useConfig();
  const {
    data: release,
    error: releaseError,
    refetch: refetchRelease,
  } = useGetSampleRelease(config.documentation?.ecosystemReleasesUrl, sample.id);
  const [codeOption, setCodeOption] = useState<string | undefined>(undefined);
  const context: GuideRenderContext = {codeValues, onOpenLink, codeOption, onCodeOptionChange: setCodeOption};

  return (
    <Stack spacing={2}>
      {/* A failed lookup of the archive says so, and the instructions below still stand. */}
      {releaseError && (
        <QueryErrorNotice
          variant="inline"
          error={releaseError}
          t={t}
          fallbackKey="applications:edit.overview.sample.error"
          fallbackDefaultValue="The sample's download could not be found. You can still download it from its releases."
          onRetry={() => {
            void refetchRelease();
          }}
        />
      )}
      {release && (
        <Paper variant="outlined" sx={{p: 2, display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2}}>
          {guide.icon}
          <Box sx={{flex: '1 1 200px', minWidth: 0}}>
            <Typography variant="subtitle2">
              {t('applications:edit.overview.sample.title', '{{label}} sample app', {label: guide.label})}
            </Typography>
            {release.version && (
              <Typography variant="body2" color="text.secondary">
                {release.sizeLabel
                  ? t('applications:edit.overview.sample.versionWithSize', 'Version {{version}}, {{size}}', {
                      version: release.version,
                      size: release.sizeLabel,
                    })
                  : t('applications:edit.overview.sample.version', 'Version {{version}}', {version: release.version})}
              </Typography>
            )}
          </Box>
          <Button
            variant="contained"
            size="small"
            startIcon={<Download size={16} aria-hidden />}
            // The product's own released sample, so it downloads without the leaving-the-console confirmation.
            component="a"
            href={release.downloadUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            {t('applications:edit.overview.sample.download', 'Download sample')}
          </Button>
        </Paper>
      )}
      <GuideBlocks blocks={sample.blocks} context={context} />
    </Stack>
  );
}

/**
 * Shows a docs quickstart inside the Console, with the application's own values filled into the
 * code samples: either the steps that build the app, or the sample app the guide offers instead.
 * The content is fetched from the docs site, so it never drifts from the published guide.
 *
 * @param props - The component props
 * @returns The guide's steps or sample, or a notice when they cannot be shown
 *
 * @public
 */
export default function QuickstartGuide({
  guide,
  codeValues,
  onOpenLink,
  view = 'steps',
}: QuickstartGuideProps): JSX.Element {
  const {t} = useTranslation();
  const {data: content, isLoading, error, refetch} = useGetQuickstartGuide(guide.docsUrl, guide.section);
  const steps = content === undefined ? undefined : (content?.steps ?? null);
  const sample = content?.sample;

  return (
    <Stack spacing={2}>
      {isLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
          <CircularProgress size={24} aria-label={t('applications:edit.overview.guide.loading', 'Loading the guide')} />
        </Box>
      )}

      {error && (
        <QueryErrorNotice
          variant="inline"
          error={error}
          t={t}
          fallbackKey="applications:edit.overview.guide.error"
          fallbackDefaultValue="The guide could not be loaded. You can still open it on the docs site."
          onRetry={() => {
            void refetch();
          }}
        />
      )}

      {steps === null && (
        <Typography variant="body2" color="text.secondary">
          {t('applications:edit.overview.guide.unavailable', 'Follow the step-by-step guide on the docs site.')}
        </Typography>
      )}

      {view === 'steps' && steps && (
        <GuideSteps key={guide.docsUrl} steps={steps} codeValues={codeValues} onOpenLink={onOpenLink} />
      )}

      {view === 'sample' && sample && (
        <GuideSample
          key={guide.docsUrl}
          guide={guide}
          sample={sample}
          codeValues={codeValues}
          onOpenLink={onOpenLink}
        />
      )}
    </Stack>
  );
}
