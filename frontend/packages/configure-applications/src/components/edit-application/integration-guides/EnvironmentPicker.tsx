// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {RuntimeEnvironment} from '@thunderid/contexts';
import {Box, IconButton, ToggleButton, ToggleButtonGroup, Tooltip} from '@wso2/oxygen-ui';
import {ChevronLeft, ChevronRight} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/** How far in from an edge the environments fade out where more of them lie beyond it. */
const FADE_WIDTH = 48;
/** How much of that is clear of them altogether, for the arrow that sits there. */
const ARROW_WIDTH = 20;

/**
 * Props for the {@link EnvironmentPicker} component.
 */
interface EnvironmentPickerProps {
  /** The environments to choose from, in the order a change moves through them. */
  environments: RuntimeEnvironment[];
  /** The environment whose values are shown. */
  selectedId: string;
  onSelect: (environmentId: string) => void;
  /** The environments the application is not deployed to yet, which are marked as such. */
  notDeployedIds?: string[];
}

/**
 * Chooses the environment whose client ID and endpoints an application's overview shows. Each environment
 * is marked with the color the rest of the console gives it, beside its name. Environments that do not fit
 * side by side stay on one row that scrolls. An edge that has more beyond it fades out under an arrow that
 * scrolls that way.
 */
export default function EnvironmentPicker({
  environments,
  selectedId,
  onSelect,
  notDeployedIds = [],
}: EnvironmentPickerProps): JSX.Element {
  const {t} = useTranslation();
  const scrollerRef = useRef<HTMLDivElement>(null);
  const [hidden, setHidden] = useState({start: false, end: false});

  const measure = (): void => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    const start = scroller.scrollLeft > 1;
    const end = scroller.scrollLeft + scroller.clientWidth < scroller.scrollWidth - 1;
    setHidden((current) => (current.start === start && current.end === end ? current : {start, end}));
  };

  // Which edges have more beyond them changes with the room the picker has and with its environments.
  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(scroller);
    if (scroller.firstElementChild) observer.observe(scroller.firstElementChild);
    const frame = requestAnimationFrame(measure);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [environments]);

  // The chosen environment is brought into view, as when it is chosen while partly scrolled out.
  useEffect(() => {
    scrollerRef.current
      ?.querySelector('[aria-pressed="true"]')
      ?.scrollIntoView({block: 'nearest', inline: 'nearest', behavior: 'smooth'});
  }, [selectedId]);

  const scrollBy = (direction: 1 | -1): void => {
    const scroller = scrollerRef.current;
    scroller?.scrollBy({left: direction * scroller.clientWidth * 0.8, behavior: 'smooth'});
  };
  const arrow = (direction: 1 | -1): JSX.Element => {
    const label =
      direction < 0
        ? t('applications:edit.overview.environments.scrollPrevious', 'Show earlier environments')
        : t('applications:edit.overview.environments.scrollNext', 'Show later environments');
    return (
      <Tooltip title={label}>
        <IconButton
          size="small"
          aria-label={label}
          onClick={() => scrollBy(direction)}
          sx={{
            position: 'absolute',
            top: '50%',
            transform: 'translateY(-50%)',
            p: 0.25,
            ...(direction < 0 ? {left: 0.5} : {right: 0.5}),
          }}
        >
          {direction < 0 ? <ChevronLeft size={16} aria-hidden /> : <ChevronRight size={16} aria-hidden />}
        </IconButton>
      </Tooltip>
    );
  };

  return (
    // A segmented control: one outlined track, with the chosen environment filled inside it.
    <Box sx={{position: 'relative', border: 1, borderColor: 'divider', borderRadius: 1, p: 0.5}}>
      <Box
        ref={scrollerRef}
        onScroll={measure}
        sx={{
          overflowX: 'auto',
          scrollbarWidth: 'none',
          '&::-webkit-scrollbar': {display: 'none'},
          // The mask's black is what stays visible, not a color that is shown.
          maskImage: `linear-gradient(to right, ${hidden.start ? `transparent ${ARROW_WIDTH}px` : 'black'}, black ${FADE_WIDTH}px, black calc(100% - ${FADE_WIDTH}px), ${hidden.end ? `transparent calc(100% - ${ARROW_WIDTH}px)` : 'black'})`,
        }}
      >
        <ToggleButtonGroup
          exclusive
          size="small"
          value={selectedId}
          aria-label={t('applications:edit.overview.environments.pickerLabel', 'Environment to show')}
          sx={{
            display: 'flex',
            width: 'max-content',
            minWidth: '100%',
            gap: 0.5,
            '& .MuiToggleButtonGroup-grouped': {border: 0, borderRadius: 0.5, m: 0},
          }}
          onChange={(_event, environmentId: string | null) => {
            if (environmentId) onSelect(environmentId);
          }}
        >
          {environments.map(({id, name, color}: RuntimeEnvironment, index: number) => {
            const notDeployed = notDeployedIds.includes(id);
            return (
              // An environment the application is not deployed to is told apart before it is chosen: its mark
              // is hollow, its name is dimmed, and the tooltip says so.
              <Tooltip
                key={id}
                describeChild
                title={
                  // The first environment is deployed to; each later one is reached by promoting.
                  !notDeployed
                    ? name
                    : index > 0
                      ? t(
                          'applications:edit.overview.environments.notPromoted',
                          'Not promoted to {{environment}} yet.',
                          {
                            environment: name,
                          },
                        )
                      : t(
                          'applications:edit.overview.environments.notDeployed',
                          'Not deployed to {{environment}} yet.',
                          {
                            environment: name,
                          },
                        )
                }
              >
                <ToggleButton
                  value={id}
                  sx={{
                    // Environment names read as short codes, set like the identifiers below them.
                    typography: 'caption',
                    fontFamily: 'monospace',
                    fontWeight: 'fontWeightBold',
                    letterSpacing: '0.05em',
                    textTransform: 'uppercase',
                    gap: 0.75,
                    px: 1,
                    py: 0.75,
                    flex: '1 0 auto',
                    width: 'auto',
                    // Brought into view clear of the fade.
                    scrollMarginInline: `${FADE_WIDTH}px`,
                  }}
                >
                  {color && (
                    <Box
                      aria-hidden
                      data-testid={notDeployed ? `environment-not-deployed-${id}` : undefined}
                      sx={(theme) => ({
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        flexShrink: 0,
                        border: 1.5,
                        borderColor: color.light,
                        bgcolor: notDeployed ? 'transparent' : color.light,
                        ...theme.applyStyles('dark', {
                          borderColor: color.dark,
                          bgcolor: notDeployed ? 'transparent' : color.dark,
                        }),
                      })}
                    />
                  )}
                  <Box component="span" sx={{whiteSpace: 'nowrap', color: notDeployed ? 'text.disabled' : undefined}}>
                    {name}
                  </Box>
                </ToggleButton>
              </Tooltip>
            );
          })}
        </ToggleButtonGroup>
      </Box>
      {hidden.start && arrow(-1)}
      {hidden.end && arrow(1)}
    </Box>
  );
}
