// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Accordion, AccordionDetails, AccordionSummary, Box, Typography} from '@wso2/oxygen-ui';
import {ChevronDown} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';

export interface PanelSectionProps {
  title: string;
  /** A one-line summary of the section, shown while it is collapsed. */
  summary?: string;
  defaultExpanded?: boolean;
  children: ReactNode;
}

/** A collapsible section of a composer panel, separated from the next one by a divider. */
export default function PanelSection({
  title,
  summary = undefined,
  defaultExpanded = false,
  children,
}: PanelSectionProps): JSX.Element {
  const [expanded, setExpanded] = useState(defaultExpanded);

  return (
    <Accordion
      expanded={expanded}
      onChange={(_event, next: boolean) => setExpanded(next)}
      disableGutters
      square
      elevation={0}
      slotProps={{transition: {unmountOnExit: true}}}
      sx={{
        bgcolor: 'transparent',
        '&::before': {display: 'none'},
        '&:not(:last-of-type)': {borderBottom: '1px solid', borderColor: 'divider'},
      }}
    >
      <AccordionSummary
        expandIcon={<ChevronDown size={16} />}
        sx={{px: 3, minHeight: 56, '& .MuiAccordionSummary-content': {minWidth: 0, mr: 1}}}
      >
        <Box sx={{minWidth: 0, py: 0.5}}>
          <Typography variant="body2" sx={{fontWeight: 600}}>
            {title}
          </Typography>
          {!expanded && summary && (
            <Typography
              variant="caption"
              color="text.secondary"
              component="div"
              sx={{display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden'}}
            >
              {summary}
            </Typography>
          )}
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{px: 3, pt: 0, pb: 3}}>{children}</AccordionDetails>
    </Accordion>
  );
}
