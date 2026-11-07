// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {formatRelativeTime} from '@thunderid/utils';
import {ListingCard} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Project} from '../models/project';

export interface ProjectCardProps {
  project: Project;
  /** Whether this is the project the console is working in. */
  selected?: boolean;
  onOpen: (project: Project) => void;
  onDelete: (project: Project) => void;
}

/**
 * A project on the projects page: its name, description (or handle) and when it was created.
 * Opening it enters the project.
 */
export default function ProjectCard({project, selected = false, onOpen, onDelete}: ProjectCardProps): ReactNode {
  const {t, i18n} = useTranslation();
  const created = new Date(project.createdAt);
  const hasCreated = !Number.isNaN(created.getTime());

  return (
    <ListingCard
      title={project.displayName}
      description={project.description ?? project.handle}
      selected={selected}
      onClick={() => onOpen(project)}
      onDelete={() => onDelete(project)}
      deleteLabel={t('projects:delete.action', 'Delete {{name}}', {name: project.displayName})}
      {...(hasCreated && {
        updatedLabel: t('projects:listing.created', 'Created'),
        updatedTime: formatRelativeTime(created, i18n.language),
        updatedDateTime: project.createdAt,
        updatedTooltip: created.toLocaleString(i18n.language),
      })}
      sx={{height: '100%'}}
      data-testid={`project-card-${project.id}`}
    />
  );
}
