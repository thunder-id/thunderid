// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Avatar,
  Card,
  CardActionArea,
  CardContent,
  IconButton,
  RelativeTime,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {BoxIcon, Trash2} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Project} from './useGetProjects';

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

/** How long ago a moment was, "2 minutes ago". */
function relative(date: Date, locale: string): string {
  const seconds = Math.round((date.getTime() - Date.now()) / 1000);
  const format = new Intl.RelativeTimeFormat(locale, {numeric: 'auto'});
  for (const [unit, size] of UNITS) {
    if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
  }
  return format.format(seconds, 'second');
}

export interface ProjectCardProps {
  project: Project;
  onOpen: (project: Project) => void;
  onDelete: (project: Project) => void;
}

const DELETE_BUTTON_CLASS = 'ProjectCard-delete';

/**
 * A project on the projects page: its name, handle and when it was created. Opening it enters the
 * project. The delete button shows while the card is hovered or anything in it has focus, so
 * tabbing onto the button reveals it; on touch screens, which cannot hover, it always shows.
 */
export default function ProjectCard({project, onOpen, onDelete}: ProjectCardProps): ReactNode {
  const {t, i18n} = useTranslation();
  const created = new Date(project.createdAt);
  const hasCreated = !Number.isNaN(created.getTime());
  const deleteLabel = t('common:projects.delete.action', 'Delete {{name}}', {name: project.displayName});

  return (
    <Card
      variant="outlined"
      sx={{
        height: '100%',
        position: 'relative',
        transition: 'border-color 0.15s',
        '&:hover, &:focus-within': {
          borderColor: 'primary.main',
          [`& .${DELETE_BUTTON_CLASS}`]: {opacity: 1},
        },
      }}
      data-testid={`project-card-${project.id}`}
    >
      <CardActionArea onClick={() => onOpen(project)} sx={{height: '100%'}}>
        <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
          <Stack spacing={2}>
            <Stack direction="row" spacing={1.5} alignItems="center" sx={{minWidth: 0}}>
              <Avatar sx={{bgcolor: 'action.hover', color: 'primary.main'}} aria-hidden>
                <BoxIcon size={20} />
              </Avatar>
              <Stack sx={{minWidth: 0}}>
                <Typography variant="subtitle1" noWrap>
                  {project.displayName}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {project.description ?? project.handle}
                </Typography>
              </Stack>
            </Stack>
            {hasCreated && (
              <RelativeTime
                size="small"
                action={t('common:projects.created', 'Created')}
                time={relative(created, i18n.language)}
                dateTime={project.createdAt}
                tooltip={created.toLocaleString(i18n.language)}
              />
            )}
          </Stack>
        </CardContent>
      </CardActionArea>
      <Tooltip title={deleteLabel}>
        <IconButton
          size="small"
          color="error"
          aria-label={deleteLabel}
          className={DELETE_BUTTON_CLASS}
          onClick={() => onDelete(project)}
          sx={{
            position: 'absolute',
            right: (theme) => theme.spacing(1.5),
            bottom: (theme) => theme.spacing(1.5),
            opacity: 0,
            transition: 'opacity 0.15s',
            '@media (hover: none)': {opacity: 1},
          }}
        >
          <Trash2 size={16} />
        </IconButton>
      </Tooltip>
    </Card>
  );
}
