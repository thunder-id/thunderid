// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useId, useRef, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

/** How much of a long step shows while it is collapsed, in pixels. */
const COLLAPSED_HEIGHT = 320;
/** How much taller than that a step has to be before collapsing it hides enough to be worth a click. */
const COLLAPSE_SLACK = 120;

/**
 * One step of a {@link GuideStepList}.
 *
 * @public
 */
export interface GuideStepItem {
  /** A key unique among the steps */
  key: string;
  /** The step's heading */
  title: ReactNode;
  /** What the step says to do */
  content: ReactNode;
}

/**
 * Props for the {@link GuideStepList} component.
 *
 * @public
 */
export interface GuideStepListProps {
  /** The steps, in order */
  steps: GuideStepItem[];
  /** Whether a step with long content starts collapsed, with a way to expand it */
  collapseLongSteps?: boolean;
}

/** A step's content, collapsed while it is long, until it is expanded. */
function CollapsibleStepContent({children}: {children: ReactNode}): JSX.Element {
  const {t} = useTranslation();
  const contentId = useId();
  const contentRef = useRef<HTMLDivElement>(null);
  const [isLong, setIsLong] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);

  // The content's height is known only once it is laid out, and changes with it, as when a code
  // sample is swapped for another language's.
  useEffect(() => {
    // The content is rendered whatever its length, so it is there by the time an effect runs.
    const content = contentRef.current!;
    const measure = (): void => setIsLong(content.offsetHeight > COLLAPSED_HEIGHT + COLLAPSE_SLACK);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(content);
    return () => observer.disconnect();
  }, []);

  const isCollapsed = isLong && !isExpanded;

  return (
    <Stack spacing={1}>
      <Box
        id={contentId}
        // Content below the cut is still reached with the keyboard, so the step opens for it rather than
        // leaving the focused control out of sight.
        onFocus={(event) => {
          if (isCollapsed && event.target.matches(':focus-visible')) {
            setIsExpanded(true);
          }
        }}
        sx={
          isCollapsed
            ? {
                maxHeight: COLLAPSED_HEIGHT,
                overflow: 'hidden',
                // The content fades out into the card, whatever its color, rather than ending mid-line.
                maskImage: 'linear-gradient(to bottom, black 55%, transparent)',
              }
            : undefined
        }
      >
        <Stack ref={contentRef} spacing={1.5} sx={{minWidth: 0}}>
          {children}
        </Stack>
      </Box>
      {isLong && (
        <Box sx={{display: 'flex', justifyContent: 'center'}}>
          <Button
            size="small"
            aria-expanded={isExpanded}
            aria-controls={contentId}
            startIcon={isExpanded ? <ChevronUp size={14} aria-hidden /> : <ChevronDown size={14} aria-hidden />}
            onClick={() => setIsExpanded(!isExpanded)}
          >
            {isExpanded
              ? t('applications:edit.overview.guide.collapseStep', 'Collapse step')
              : t('applications:edit.overview.guide.expandStep', 'Expand step')}
          </Button>
        </Box>
      )}
    </Stack>
  );
}

/**
 * Numbered steps, each a heading followed by what to do. All are open, unless long steps are
 * collapsed: then a step with long content shows its start, and expands on request.
 *
 * @public
 */
export default function GuideStepList({steps, collapseLongSteps = false}: GuideStepListProps): JSX.Element {
  return (
    <Stack component="ol" spacing={3.5} sx={{listStyle: 'none', m: 0, p: 0}}>
      {steps.map((step, index) => (
        <Box
          key={step.key}
          component="li"
          sx={{display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr)', columnGap: 2, alignItems: 'start'}}
        >
          <Box
            aria-hidden
            sx={{
              width: 24,
              height: 24,
              borderRadius: '50%',
              bgcolor: 'action.selected',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              typography: 'caption',
            }}
          >
            {index + 1}
          </Box>
          <Stack spacing={1.5} sx={{minWidth: 0}}>
            <Typography variant="subtitle1" component="h3">
              {step.title}
            </Typography>
            {collapseLongSteps ? <CollapsibleStepContent>{step.content}</CollapsibleStepContent> : step.content}
          </Stack>
        </Box>
      ))}
    </Stack>
  );
}
