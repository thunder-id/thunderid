// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useCopyToClipboard} from '@thunderid/hooks';
import {
  Alert,
  AlertTitle,
  alpha,
  Box,
  Button,
  CircularProgress,
  IconButton,
  Link,
  Stack,
  Step,
  StepButton,
  StepContent,
  Stepper,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {ArrowUpRight, Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import useGetQuickstartGuide from '../../../api/useGetQuickstartGuide';
import {
  fillQuickstartGuideCode,
  type QuickstartGuideAlertKind,
  type QuickstartGuideBlock,
  type QuickstartGuideStep,
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
}

/**
 * Props for the {@link QuickstartGuide} component.
 *
 * @public
 */
export interface QuickstartGuideProps {
  /**
   * The guides to offer. More than one renders a platform switcher.
   */
  guides: QuickstartGuideSource[];
  /**
   * Placeholder text in the docs code samples, mapped to this application's value for it
   */
  codeValues: Record<string, string | undefined>;
  /**
   * Opens an external URL, after whatever confirmation the host page wants
   */
  onOpenLink: (url: string) => void;
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

/**
 * A code sample with a copy button. Several options render as tabs, one per option. Values filled
 * in from the application are highlighted so they stand apart from the sample's own code.
 */
function GuideCode({
  title = undefined,
  options,
  context,
}: {
  title?: string;
  options: {label?: string; language?: string; code: string}[];
  context: GuideRenderContext;
}): JSX.Element {
  const {t} = useTranslation();
  const {copied, copy} = useCopyToClipboard();
  const selected = options.findIndex((option) => option.label === context.codeOption);
  const active = options[Math.max(selected, 0)];
  const segments = fillQuickstartGuideCode(active.code, context.codeValues);
  const copyLabel = copied ? t('common:actions.copied', 'Copied!') : t('common:actions.copy', 'Copy');

  return (
    <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden', minWidth: 0}}>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        spacing={1}
        sx={{pl: options.length > 1 ? 0 : 2, pr: 1, borderBottom: 1, borderColor: 'divider', bgcolor: 'action.hover'}}
      >
        {options.length > 1 ? (
          <Tabs
            value={Math.max(selected, 0)}
            onChange={(_, index: number) => context.onCodeOptionChange(options[index].label ?? '')}
            variant="scrollable"
            scrollButtons="auto"
            aria-label={t('applications:edit.overview.guide.codeOptions', 'Code sample options')}
          >
            {options.map((option) => (
              <Tab key={option.label} label={option.label} />
            ))}
          </Tabs>
        ) : (
          <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}} noWrap>
            {title ?? active.language}
          </Typography>
        )}
        <Tooltip title={copyLabel}>
          <IconButton
            size="small"
            aria-label={copyLabel}
            onClick={() => {
              copy(segments.map((segment) => segment.text).join('')).catch(() => undefined);
            }}
          >
            {copied ? <Check size={16} aria-hidden /> : <Copy size={16} aria-hidden />}
          </IconButton>
        </Tooltip>
      </Stack>
      <Box
        component="pre"
        tabIndex={0}
        aria-label={title ?? active.label ?? t('applications:edit.overview.guide.codeSample', 'Code sample')}
        sx={{m: 0, p: 2, overflow: 'auto', typography: 'body2', fontFamily: 'monospace'}}
      >
        <code>
          {segments.map((segment, index) =>
            segment.filled ? (
              <Box
                // eslint-disable-next-line react/no-array-index-key
                key={index}
                component="mark"
                sx={(theme) => ({
                  color: 'inherit',
                  bgcolor: alpha(theme.palette.primary.main, 0.2),
                  borderRadius: 0.5,
                  px: 0.25,
                })}
              >
                {segment.text}
              </Box>
            ) : (
              segment.text
            ),
          )}
        </code>
      </Box>
    </Box>
  );
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
            return <GuideCode key={key} title={block.title} options={[block]} context={context} />;
          case 'codeGroup':
            return <GuideCode key={key} options={block.options} context={context} />;
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

/** The steps of one guide, one open at a time. */
function GuideSteps({
  steps,
  codeValues,
  onOpenLink,
}: {
  steps: QuickstartGuideStep[];
  codeValues: Record<string, string | undefined>;
  onOpenLink: (url: string) => void;
}): JSX.Element {
  const {t} = useTranslation();
  const [activeStep, setActiveStep] = useState(0);
  const [codeOption, setCodeOption] = useState<string | undefined>(undefined);
  const context: GuideRenderContext = {codeValues, onOpenLink, codeOption, onCodeOptionChange: setCodeOption};

  return (
    <Stepper nonLinear orientation="vertical" activeStep={activeStep}>
      {steps.map((step, index) => (
        <Step key={step.title} completed={index < activeStep}>
          <StepButton onClick={() => setActiveStep(index)}>
            <Typography variant="subtitle2" component="span">
              <GuideText text={step.title} onOpenLink={onOpenLink} />
            </Typography>
          </StepButton>
          <StepContent slotProps={{transition: {unmountOnExit: true}}}>
            <Box sx={{pt: 1, pb: 2}}>
              <GuideBlocks blocks={step.blocks} context={context} />
              <Stack direction="row" spacing={1} sx={{mt: 2}}>
                {index < steps.length - 1 && (
                  <Button variant="contained" size="small" onClick={() => setActiveStep(index + 1)}>
                    {t('applications:edit.overview.guide.nextStep', 'Next step')}
                  </Button>
                )}
                {index > 0 && (
                  <Button size="small" onClick={() => setActiveStep(index - 1)}>
                    {t('common:actions.back', 'Back')}
                  </Button>
                )}
              </Stack>
            </Box>
          </StepContent>
        </Step>
      ))}
    </Stepper>
  );
}

/**
 * Shows a docs quickstart as steps inside the Console, with the application's own values filled
 * into the code samples. The content is fetched from the docs site, so it never drifts from the
 * published guide. When the guide cannot be shown inline, the link to the docs site remains.
 *
 * @param props - The component props
 * @returns The guide, with a platform switcher when there is more than one
 *
 * @public
 */
export default function QuickstartGuide({guides, codeValues, onOpenLink}: QuickstartGuideProps): JSX.Element {
  const {t} = useTranslation();
  const [selectedLabel, setSelectedLabel] = useState(guides[0].label);
  const guide = guides.find((entry) => entry.label === selectedLabel) ?? guides[0];
  const {data: steps, isLoading, error, refetch} = useGetQuickstartGuide(guide.docsUrl);

  return (
    <Stack spacing={2}>
      <Stack direction="row" flexWrap="wrap" alignItems="center" justifyContent="space-between" gap={1}>
        {guides.length > 1 ? (
          <ToggleButtonGroup
            exclusive
            size="small"
            value={guide.label}
            onChange={(_, label: string | null) => {
              if (label) {
                setSelectedLabel(label);
              }
            }}
            aria-label={t('applications:edit.overview.guide.platform', 'Platform')}
          >
            {guides.map((entry) => (
              <ToggleButton key={entry.label} value={entry.label} sx={{gap: 1}}>
                {entry.icon}
                {entry.label}
              </ToggleButton>
            ))}
          </ToggleButtonGroup>
        ) : (
          <Stack direction="row" alignItems="center" spacing={1}>
            {guide.icon}
            <Typography variant="subtitle2">
              {t('applications:edit.overview.guide.quickstartFor', '{{label}} quickstart', {label: guide.label})}
            </Typography>
          </Stack>
        )}
        <Button
          variant="outlined"
          size="small"
          endIcon={<ArrowUpRight size={16} />}
          onClick={() => onOpenLink(guide.docsUrl)}
        >
          {t('applications:edit.overview.guide.openDocs', 'Open in docs')}
        </Button>
      </Stack>

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
            refetch().catch(() => undefined);
          }}
        />
      )}

      {steps === null && (
        <Typography variant="body2" color="text.secondary">
          {t('applications:edit.overview.guide.unavailable', 'Follow the step-by-step guide on the docs site.')}
        </Typography>
      )}

      {steps && <GuideSteps key={guide.docsUrl} steps={steps} codeValues={codeValues} onOpenLink={onOpenLink} />}
    </Stack>
  );
}
