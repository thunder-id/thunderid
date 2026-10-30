// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, PageLoadingAnimation, QueryErrorNotice} from '@thunderid/components';
import {Button, Grid, PageContent, PageTitle, SearchBar, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ProjectCard from './ProjectCard';
import ProjectDeleteDialog from './ProjectDeleteDialog';
import type {Project} from './useGetProjects';
import useProjectSelection from './useProjectSelection';
import RouteConfig from '../../configs/RouteConfig';

/**
 * Every project in the deployment, shown once the project selection is cleared. Opening a project
 * selects it and opens its applications.
 */
export default function ProjectsListPage(): ReactNode {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {projects, isLoading, error, refetch, selectProject} = useProjectSelection();
  const [search, setSearch] = useState('');
  const [projectToDelete, setProjectToDelete] = useState<Project | null>(null);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) {
      return projects;
    }

    return projects.filter((project) =>
      [project.displayName, project.handle, project.description ?? ''].some((value) =>
        value.toLowerCase().includes(query),
      ),
    );
  }, [projects, search]);

  const handleOpen = (project: Project): void => {
    selectProject(project.id);
    void navigate(RouteConfig.applications.list());
  };

  const title = t('common:projects.title', 'All projects');

  const renderBody = (): ReactNode => {
    if (isLoading) {
      return <PageLoadingAnimation />;
    }

    if (error) {
      return <QueryErrorNotice error={error} t={t} variant="inline" onRetry={refetch} />;
    }

    if (filtered.length === 0) {
      return (
        <Typography variant="body2" color="text.secondary" sx={{py: 6, textAlign: 'center'}}>
          {projects.length === 0
            ? t('common:projects.empty', 'There are no projects yet.')
            : t('common:projects.noMatches', 'No matching projects')}
        </Typography>
      );
    }

    return (
      <Grid container spacing={2}>
        {filtered.map((project) => (
          <Grid key={project.id} size={{xs: 12, sm: 6, md: 4, xl: 3}}>
            <ProjectCard project={project} onOpen={handleOpen} onDelete={setProjectToDelete} />
          </Grid>
        ))}
      </Grid>
    );
  };

  return (
    <PageContent>
      <DocumentTitle title={title} />
      <PageTitle>
        <PageTitle.Header>{title}</PageTitle.Header>
        <PageTitle.SubHeader>{t('common:projects.subtitle', 'Choose a project to work in.')}</PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => void navigate(RouteConfig.projects.create())}
          >
            {t('common:projects.add', 'Add project')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>
      <Stack spacing={3}>
        <SearchBar
          fullWidth
          size="small"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('common:projects.search', 'Search projects')}
          slotProps={{htmlInput: {'aria-label': t('common:projects.search', 'Search projects')}}}
        />
        {renderBody()}
      </Stack>
      <ProjectDeleteDialog project={projectToDelete} onClose={() => setProjectToDelete(null)} />
    </PageContent>
  );
}
