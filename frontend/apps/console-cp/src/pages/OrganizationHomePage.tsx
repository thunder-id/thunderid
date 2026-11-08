// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries, useQuery} from '@tanstack/react-query';
import {DocumentTitle, QueryErrorNotice} from '@thunderid/components';
import {gatewayColor, usePipeline, useProjects, type PipelineStage, type Project} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {useConfig} from '@thunderid/contexts';
import {User, useThunderID} from '@thunderid/react';
import {
  Box,
  Button,
  ButtonBase,
  Card,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  PageContent,
  Radio,
  Stack,
  Link as TextLink,
  Typography,
  alpha,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Bot,
  ExternalLink,
  Globe,
  LayoutGrid,
  Plus,
  Rocket,
  MoveRight,
  Server,
  SlidersHorizontal,
  Smartphone,
  Users,
} from '@wso2/oxygen-ui-icons-react';
import {Fragment, useEffect, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import RouteConfig from '../configs/RouteConfig';

/** Reads how many of a listed resource there are, from a list of one. */
function useCountReader(): (path: string) => Promise<number> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  return async (path: string): Promise<number> => {
    const separator = path.includes('?') ? '&' : '?';
    const response = (await http.request({
      url: `${getServerUrl()}${path}${separator}limit=1&offset=0`,
      method: 'GET',
    } as unknown as Parameters<typeof http.request>[0])) as {data: {totalResults?: number}};
    return response.data.totalResults ?? 0;
  };
}

/** How many of a listed resource there are. */
function useCount(path: string): number | undefined {
  const read = useCountReader();
  return useQuery<number>({
    queryKey: ['organization-home-count', path],
    queryFn: () => read(path),
    retry: false,
    staleTime: 60_000,
  }).data;
}

const applicationsOf = (project: Project): string => `/applications?projectId=${encodeURIComponent(project.id)}`;
const agentsOf = (project: Project): string => `/agents?projectId=${encodeURIComponent(project.id)}`;

/** How many applications or agents the projects hold together, and how many projects hold any. */
function useAcross(projects: Project[], pathOf: (project: Project) => string): {total?: number; projects: number} {
  const read = useCountReader();
  const results = useQueries({
    queries: projects.map((project: Project) => ({
      queryKey: ['organization-home-count', pathOf(project)],
      queryFn: () => read(pathOf(project)),
      retry: false,
      staleTime: 60_000,
    })),
  });
  if (results.some((result) => result.data === undefined)) return {projects: 0};
  const counts = results.map((result) => result.data ?? 0);
  return {
    total: counts.reduce((sum: number, count: number) => sum + count, 0),
    projects: counts.filter((count: number) => count > 0).length,
  };
}

/** A section of the overview: a heading with what it holds. */
function Section({
  title,
  subtitle = undefined,
  action = undefined,
  children = undefined,
  testId,
}: {
  title: ReactNode;
  subtitle?: string;
  action?: ReactNode;
  children?: ReactNode;
  testId: string;
}): JSX.Element {
  return (
    <Card variant="outlined" sx={{p: 3}} component="section" data-testid={testId}>
      <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: children ? 2 : 0, flexWrap: 'wrap'}}>
        <Box sx={{flex: 1, minWidth: 240}}>
          <Typography variant="h6" component="h2">
            {title}
          </Typography>
          {subtitle && (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        {action}
      </Stack>
      {children}
    </Card>
  );
}

/** How many applications and agents a project holds, as a person reads it. */
function useProjectCounts(project: Project): string {
  const {t} = useTranslation();
  const applications = useCount(applicationsOf(project));
  const agents = useCount(agentsOf(project));
  const countOf = (count: number | undefined, key: string, one: string, other: string, none: string): string => {
    if (count === undefined) return '';
    if (count === 0) return t(`home:organization.${key}None`, none);
    return t(`home:organization.${key}`, {count, defaultValue_one: one, defaultValue_other: other});
  };
  return [
    countOf(applications, 'applications', '{{count}} application', '{{count}} applications', 'No applications'),
    countOf(agents, 'agents', '{{count}} agent', '{{count}} agents', 'No agents'),
  ]
    .filter(Boolean)
    .join(' · ');
}

/** One project, with how many applications and agents it holds; it opens the project. */
function ProjectRow({project, onOpen}: {project: Project; onOpen: () => void}): JSX.Element {
  const counts = useProjectCounts(project);
  return (
    <ButtonBase
      onClick={onOpen}
      sx={{
        display: 'flex',
        justifyContent: 'flex-start',
        gap: 2,
        width: '100%',
        textAlign: 'left',
        px: 2,
        py: 1.5,
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        '&:hover': {bgcolor: 'action.hover'},
      }}
      data-testid={`organization-project-${project.id}`}
    >
      <Typography variant="body1" fontWeight={500} sx={{flex: 1, minWidth: 0}} noWrap>
        {project.displayName}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {counts}
      </Typography>
      <ArrowRight size={16} />
    </ButtonBase>
  );
}

/** One project to choose in the picker, with what it holds. */
function PickerOption({
  project,
  checked,
  onChoose,
}: {
  project: Project;
  checked: boolean;
  onChoose: () => void;
}): JSX.Element {
  const counts = useProjectCounts(project);
  return (
    <ButtonBase
      onClick={onChoose}
      role="radio"
      aria-checked={checked}
      sx={{
        display: 'flex',
        justifyContent: 'flex-start',
        gap: 1,
        width: '100%',
        textAlign: 'left',
        px: 1,
        py: 1,
        border: 1,
        borderColor: checked ? 'primary.main' : 'divider',
        borderRadius: 1,
      }}
      data-testid={`project-picker-${project.id}`}
    >
      <Radio checked={checked} size="small" tabIndex={-1} />
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="body2" fontWeight={500} noWrap>
          {project.displayName}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {counts}
        </Typography>
      </Box>
    </ButtonBase>
  );
}

/** What the project picker was opened for: a deploy, or a new application or agent. */
type Pick = 'deploy' | 'application' | 'agent';

/** The rounded buttons of the overview, as the design draws them. */
const PILL = {minHeight: 40, px: 2, borderRadius: '20px', fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap'};

/** Where configuration enters the first gateway, in that gateway's color. */
function DeployArrow({label, color}: {label: string; color: string}): JSX.Element {
  return (
    <Stack alignItems="center" spacing={0.25} sx={{flex: '0 0 auto', pt: 1.25, color}}>
      <Typography color="inherit" sx={{fontSize: 11, fontWeight: 600, whiteSpace: 'nowrap'}}>
        {label}
      </Typography>
      <MoveRight size={28} />
    </Stack>
  );
}

/** Where promotion starts: a dashed divide, broken where the arrow crosses it. */
function PromoteArrow({label}: {label: string}): JSX.Element {
  const dash = {borderLeft: '1px dashed', borderColor: 'divider'};
  return (
    <Stack alignItems="center" sx={{flex: '0 0 auto', alignSelf: 'stretch', width: 96, color: 'text.secondary'}}>
      <Box aria-hidden sx={{...dash, height: 10}} />
      <Stack alignItems="center" spacing={0.25} sx={{py: 0.5}}>
        <Typography color="inherit" sx={{fontSize: 11, fontWeight: 600, whiteSpace: 'nowrap'}}>
          {label}
        </Typography>
        <MoveRight size={28} />
      </Stack>
      <Box aria-hidden sx={{...dash, flex: 1}} />
    </Stack>
  );
}

/** The dashed lines from this console to the two kinds of resource configured in it. */
function ForkLines(): JSX.Element {
  return (
    <Box component="span" sx={{display: 'block', color: 'primary.main', opacity: 0.55}}>
      <svg
        aria-hidden="true"
        width="160"
        height="22"
        viewBox="0 0 160 22"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeDasharray="3 3"
        strokeLinecap="round"
        style={{display: 'block'}}
      >
        <path d="M80 1 L40 21" />
        <path d="M80 1 L120 21" />
      </svg>
    </Box>
  );
}

/** One step of the path: its mark, its name and what it is for, and what hangs below it. */
function PathStep({
  mark,
  color,
  name,
  role,
  square = false,
  mono = false,
  dashed = false,
  testId = undefined,
  children,
}: {
  mark: ReactNode;
  color: string;
  name: string;
  role: string;
  square?: boolean;
  mono?: boolean;
  dashed?: boolean;
  testId?: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Stack
      alignItems="center"
      spacing={0.75}
      sx={{flex: '0 0 auto', width: 180, textAlign: 'center'}}
      data-testid={testId}
    >
      <Box
        sx={{
          width: 52,
          height: 52,
          borderRadius: square ? '14px' : '50%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color,
          border: dashed ? '1.5px dashed' : '1.5px solid',
          borderColor: color,
          bgcolor: square ? (theme) => alpha(theme.palette.primary.main, 0.12) : 'action.hover',
        }}
      >
        {mark}
      </Box>
      <Stack sx={{lineHeight: 1.3}}>
        <Typography
          sx={{color, fontSize: 13, fontWeight: 600, lineHeight: 1.3, fontFamily: mono ? 'monospace' : undefined}}
        >
          {name}
        </Typography>
        <Typography
          color="text.secondary"
          sx={{fontSize: 12, lineHeight: 1.3, minHeight: '1.3em'}}
          data-testid={testId ? 'organization-gateway-role' : undefined}
        >
          {role}
        </Typography>
      </Stack>
      {children}
    </Stack>
  );
}

/** What a step holds or serves, hanging below it. */
function PathLeaf({icon, label, accent = false}: {icon: ReactNode; label: string; accent?: boolean}): JSX.Element {
  return (
    <Stack alignItems="center" spacing={1} sx={{width: 80}}>
      <Box
        sx={{
          width: 34,
          height: 34,
          borderRadius: '50%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: accent ? 'primary.light' : 'text.secondary',
          border: '1px dashed',
          borderColor: accent ? (theme) => alpha(theme.palette.primary.main, 0.45) : 'text.disabled',
        }}
      >
        {icon}
      </Box>
      <Typography color="text.secondary" sx={{fontSize: 12, lineHeight: 1.3, minHeight: '2.6em'}}>
        {label}
      </Typography>
    </Stack>
  );
}

/** Where a gateway is reached, for a person to tell gateways apart. */
const hostOf = (url: string | undefined): string => {
  if (!url) return '';
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
};

/** A person's first name, as their profile gives it. */
const firstNameOf = (user: unknown): string | undefined => {
  const name = (user as {name?: unknown} | null)?.name;
  return typeof name === 'string' && name ? name.split(' ')[0] : undefined;
};

export interface OrganizationHomePageProps {
  /** Says where the gateways come from, as the deployment has them. */
  gatewaysSource?: string;
  /** Offers to create a project, where the console can. */
  canCreateProject?: boolean;
}

/**
 * The organization's overview: how configuration moves from this console through the gateways, the
 * applications and agents the projects hold, and the projects themselves.
 */
export default function OrganizationHomePage({
  gatewaysSource = undefined,
  canCreateProject = true,
}: OrganizationHomePageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const pipeline = usePipeline();
  const {projects, selectProject: select, clearSelection} = useProjectSelection();
  // The organization's home is the organization level: no project is selected while it is open.
  useEffect(() => clearSelection(), [clearSelection]);
  const projectList = useProjects();
  const applications = useAcross(projects, applicationsOf);
  const agents = useAcross(projects, agentsOf);
  const stages: PipelineStage[] = pipeline.data?.stages ?? [];
  const manageGatewaysUrl = window.__THUNDERID_RUNTIME_CONFIG__?.deploy?.manage_environments_url;
  const [picking, setPicking] = useState<Pick | null>(null);
  const [chosen, setChosen] = useState<string | null>(null);

  const openProject = (id: string): void => {
    select(id);
    void navigate(RouteConfig.projects.home(id));
  };
  const pick = (purpose: Pick): void => {
    setChosen(projects[0]?.id ?? null);
    setPicking(purpose);
  };
  const proceed = (): void => {
    if (!chosen || !picking) return;
    select(chosen);
    setPicking(null);
    if (picking === 'application') void navigate(RouteConfig.applications.types());
    else if (picking === 'agent') void navigate(RouteConfig.agents.create());
    else void navigate(RouteConfig.projects.home(chosen));
  };

  // What a gateway is for: the first is where a deploy enters. Which gateway follows which belongs to each
  // project's pipeline, so the overview names no order for the rest.
  const roleOf = (index: number): string =>
    index === 0 ? t('home:organization.roleDeployFirst', 'Deploy here first') : '';

  const pickerText: Record<Pick, {title: string; subtitle: string; note: string}> = {
    deploy: {
      title: t('home:organization.deploy', 'Deploy'),
      subtitle: t(
        'home:organization.pickDeploySubtitle',
        'Choose the project to deploy. Each project deploys through its own pipeline.',
      ),
      note: t(
        'home:organization.pickDeployNote',
        'Its latest version enters the pipeline at the first gateway, together with shared resources.',
      ),
    },
    application: {
      title: t('home:organization.newApplication', 'New application'),
      subtitle: t(
        'home:organization.pickApplicationSubtitle',
        'Applications belong to a project. Choose where this one lives.',
      ),
      note: t('home:organization.pickCreateNote', "It will be deployed through this project's pipeline."),
    },
    agent: {
      title: t('home:organization.newAgent', 'New agent'),
      subtitle: t('home:organization.pickAgentSubtitle', 'Agents belong to a project. Choose where this one lives.'),
      note: t('home:organization.pickCreateNote', "It will be deployed through this project's pipeline."),
    },
  };

  const acrossTile = (
    kind: 'applications' | 'agents',
    icon: ReactNode,
    across: {total?: number; projects: number},
    lead: string,
    action: string,
    onAction: () => void,
  ): JSX.Element => (
    <Card
      variant="outlined"
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 2,
        p: 2.5,
        borderRadius: '12px',
        bgcolor: (theme) => alpha(theme.palette.primary.main, 0.07),
        border: '1px solid',
        borderColor: (theme) => alpha(theme.palette.primary.main, 0.3),
      }}
      data-testid={`organization-${kind}`}
    >
      <Box
        sx={{
          width: 48,
          height: 48,
          flex: '0 0 auto',
          borderRadius: '12px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          bgcolor: (theme) => alpha(theme.palette.primary.main, 0.14),
          color: 'primary.main',
        }}
      >
        {icon}
      </Box>
      <Stack spacing={0.25} sx={{flex: '1 1 auto', minWidth: 0}}>
        <Stack direction="row" spacing={1.25} alignItems="baseline">
          <Typography component="span" sx={{fontSize: 28, fontWeight: 500, lineHeight: 1}}>
            {across.total ?? '·'}
          </Typography>
          <Typography component="span" sx={{fontSize: 16, fontWeight: 600}}>
            {kind === 'applications'
              ? t('home:organization.applicationsLabel', 'applications')
              : t('home:organization.agentsLabel', 'agents')}
          </Typography>
        </Stack>
        <Typography color="text.secondary" sx={{fontSize: 13}}>
          {lead}
        </Typography>
        <Typography color="text.secondary" sx={{fontSize: 12, opacity: 0.8}}>
          {t('home:organization.acrossProjects', {
            count: across.projects,
            defaultValue_one: 'Across {{count}} project',
            defaultValue_other: 'Across {{count}} projects',
          })}
        </Typography>
      </Stack>
      <Button
        variant="contained"
        startIcon={<Plus size={16} />}
        onClick={onAction}
        disabled={projects.length === 0}
        sx={{...PILL, flex: '0 0 auto'}}
      >
        {action}
      </Button>
    </Card>
  );

  return (
    <PageContent>
      <DocumentTitle title={t('home:organization.title', 'Overview')} />
      <Stack spacing={4}>
        <User>
          {(user) => (
            <Box>
              <Typography variant="h1" sx={{fontSize: {xs: '2rem', md: '2.5rem'}, fontWeight: 500}} gutterBottom>
                {t('home:organization.hello', 'Hello, {{name}}', {
                  name: firstNameOf(user) ?? t('home:organization.there', 'there'),
                })}
              </Typography>
              <Typography variant="body1" color="text.secondary">
                {t(
                  'home:organization.lead',
                  'Shared settings here apply to every project. Each project builds its own applications and agents and runs them through its pipeline.',
                )}
              </Typography>
            </Box>
          )}
        </User>

        <Card
          variant="outlined"
          component="figure"
          aria-labelledby="organization-how-it-works-title"
          sx={{
            m: 0,
            display: 'flex',
            flexDirection: 'column',
            gap: 3.5,
            p: '20px 24px 24px',
            bgcolor: 'background.paper',
            border: 1,
            borderColor: 'divider',
            borderRadius: '14px',
          }}
          data-testid="organization-how-it-works"
        >
          <Stack spacing={0.75}>
            <Typography
              id="organization-how-it-works-title"
              color="text.secondary"
              sx={{fontSize: 12, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase'}}
            >
              {t('home:organization.howItWorks', 'How it works')}
            </Typography>
            <Typography color="text.secondary" sx={{maxWidth: 680}}>
              {stages.length === 1
                ? t(
                    'home:organization.howItWorksLeadSingle',
                    'Configure here, then deploy to your gateway. It has its own users and the apps connected to it.',
                  )
                : t(
                    'home:organization.howItWorksLead',
                    'Configure here, then deploy to your first gateway and promote through the rest. Each gateway has its own users and the apps connected to it.',
                  )}
            </Typography>
          </Stack>
          {pipeline.isLoading && <CircularProgress size={20} />}
          {pipeline.isError && (
            <QueryErrorNotice
              error={pipeline.error}
              t={t}
              variant="inline"
              fallbackKey="home:organization.gatewaysFailed"
              fallbackDefaultValue="The gateways could not be read."
              onRetry={() => void pipeline.refetch()}
            />
          )}
          <Box
            tabIndex={0}
            role="region"
            aria-label={t('home:organization.flowRegion', 'Configuration flow diagram')}
            sx={{overflowX: 'auto'}}
          >
            {/* Centered while it fits, as the design has it; wider than the card, it scrolls from its start. */}
            <Box sx={{display: 'flex', alignItems: 'flex-start', width: 'max-content', mx: 'auto'}}>
              <PathStep
                mark={<SlidersHorizontal size={22} />}
                color="primary.main"
                square
                name={t('home:organization.thisConsole', 'This console')}
                role={t('home:organization.configureHere', 'Configure here')}
              >
                <ForkLines />
                <Stack direction="row">
                  <PathLeaf
                    icon={<LayoutGrid size={18} />}
                    label={t('home:organization.projectManaged', 'Project-managed resources')}
                    accent
                  />
                  <PathLeaf
                    icon={<Globe size={18} />}
                    label={t('home:organization.globalShared', 'Global shared resources')}
                    accent
                  />
                </Stack>
              </PathStep>
              {stages.map((stage: PipelineStage, index: number) => {
                const color = gatewayColor(index);
                const tag = stage.name.toUpperCase();
                const last = index > 0 && index === stages.length - 1;
                return (
                  <Fragment key={stage.gateway.id}>
                    {index === 0 && (
                      <DeployArrow label={t('home:organization.deployJoint', 'Deploy')} color={color.light} />
                    )}
                    {/* One divide, as the design draws it: the first gateway, then everything promoted from it. */}
                    {index === 1 && <PromoteArrow label={t('home:organization.promote', 'Promote')} />}
                    <PathStep
                      mark={<Server size={22} />}
                      color={color.light}
                      name={tag}
                      mono
                      role={roleOf(index)}
                      testId={`organization-gateway-${stage.gateway.id}`}
                    >
                      {/* As tall as the fork under this console, so every step's circles line up. */}
                      <Box aria-hidden sx={{height: 22, borderLeft: '2px dashed', borderColor: 'divider'}} />
                      <Stack direction="row">
                        <PathLeaf
                          icon={<Smartphone size={18} />}
                          label={
                            last
                              ? t('home:organization.liveApps', 'Live apps')
                              : t('home:organization.testApps', 'Apps')
                          }
                        />
                        <PathLeaf
                          icon={<Users size={18} />}
                          label={
                            last
                              ? t('home:organization.liveUsers', 'Customers, employees')
                              : t('home:organization.testUsers', 'Test users')
                          }
                        />
                      </Stack>
                      {/* Its own space above, which the step's spacing would otherwise override. */}
                      <Box sx={{pt: 2}}>
                        <Button
                          component={Link}
                          to={RouteConfig.deploy.gateway(stage.gateway.id)}
                          variant="outlined"
                          color="inherit"
                          endIcon={<ArrowRight size={13} />}
                          aria-label={t('home:organization.openGatewayAt', 'Open {{tag}} ({{host}})', {
                            tag,
                            host: hostOf(stage.gateway.baseUrl),
                          })}
                          sx={{
                            minHeight: 34,
                            px: 1.5,
                            borderRadius: '17px',
                            borderColor: 'divider',
                            fontSize: 12,
                            fontWeight: 500,
                            whiteSpace: 'nowrap',
                            '& .MuiButton-endIcon': {ml: 0.75},
                          }}
                        >
                          {t('home:organization.openGateway', 'Open {{tag}}', {tag})}
                        </Button>
                      </Box>
                    </PathStep>
                  </Fragment>
                );
              })}
              {stages.length === 1 && (
                <>
                  <PromoteArrow label={t('home:organization.promote', 'Promote')} />
                  <PathStep
                    mark={<Plus size={22} />}
                    color="text.secondary"
                    dashed
                    name={t('home:organization.addGateway', 'Add a gateway')}
                    role={t('home:organization.noHigherGateways', 'No higher gateways')}
                    testId="organization-gateway-more"
                  >
                    {manageGatewaysUrl && (
                      <Box sx={{pt: 1}}>
                        <Button
                          component="a"
                          href={manageGatewaysUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          variant="outlined"
                          color="inherit"
                          endIcon={<ExternalLink size={13} />}
                          sx={{
                            minHeight: 34,
                            px: 1.5,
                            borderRadius: '17px',
                            borderColor: 'divider',
                            fontSize: 12,
                            fontWeight: 500,
                            whiteSpace: 'nowrap',
                            '& .MuiButton-endIcon': {ml: 0.75},
                          }}
                          data-testid="organization-manage-gateways"
                        >
                          {t('home:organization.manageGateways', 'Open gateway management')}
                        </Button>
                      </Box>
                    )}
                  </PathStep>
                </>
              )}
            </Box>
          </Box>
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{gap: 1.5, flexWrap: 'wrap', pt: 1.75, borderTop: 1, borderColor: 'divider'}}
          >
            <Typography color="text.secondary" sx={{fontSize: 13}}>
              {gatewaysSource ??
                t('home:organization.gatewaysSource', 'Gateways come from the deployment configuration.')}
            </Typography>
            <Stack direction="row" alignItems="center" sx={{gap: 2, flexWrap: 'wrap'}}>
              {manageGatewaysUrl && (
                <TextLink
                  href={manageGatewaysUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  underline="none"
                  sx={{display: 'inline-flex', alignItems: 'center', gap: 0.75, fontSize: 13, fontWeight: 500}}
                  data-testid="organization-manage-runtime"
                >
                  {t('home:organization.manageRuntimeGateways', 'Manage gateways')}
                  <ExternalLink size={13} />
                </TextLink>
              )}
              {projects.length > 0 && (
                <Button
                  variant="contained"
                  startIcon={<Rocket size={16} />}
                  onClick={() => pick('deploy')}
                  sx={PILL}
                  data-testid="organization-deploy"
                >
                  {t('home:organization.deploy', 'Deploy')}
                </Button>
              )}
            </Stack>
          </Stack>
        </Card>

        <Box
          component="section"
          aria-label={t('home:organization.applicationsAndAgents', 'Applications and agents')}
          sx={{display: 'flex', flexDirection: 'column', gap: 2}}
          data-testid="organization-applications-and-agents"
        >
          <Stack spacing={0.5}>
            <Typography component="h2" sx={{fontSize: 20, fontWeight: 600}}>
              {t('home:organization.applicationsAndAgents', 'Applications and agents')}
            </Typography>
            <Typography color="text.secondary">
              {t(
                'home:organization.applicationsAndAgentsLead',
                'The core of your identity setup. Every application and agent belongs to a project, which configures, deploys and promotes it.',
              )}
            </Typography>
          </Stack>
          <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(380px, 100%), 1fr))', gap: 2}}>
            {acrossTile(
              'applications',
              <LayoutGrid size={24} />,
              applications,
              t('home:organization.applicationsLead', 'Web, mobile and API clients that sign users in'),
              t('home:organization.newApplication', 'New application'),
              () => pick('application'),
            )}
            {acrossTile(
              'agents',
              <Bot size={24} />,
              agents,
              t('home:organization.agentsLead', 'AI agents with their own identity and access'),
              t('home:organization.newAgent', 'New agent'),
              () => pick('agent'),
            )}
          </Box>
        </Box>

        <Section
          title={t('home:organization.projectsCount', 'Projects · {{count}}', {count: projects.length})}
          subtitle={t('home:organization.projectsLead', 'Each project holds its own applications and agents.')}
          action={
            canCreateProject ? (
              <Button variant="outlined" size="small" onClick={() => void navigate(RouteConfig.projects.create())}>
                {t('home:organization.createProject', 'Create project')}
              </Button>
            ) : undefined
          }
          testId="organization-projects"
        >
          {projectList.isLoading && <CircularProgress size={20} />}
          <Stack spacing={1}>
            {projects.map((project: Project) => (
              <ProjectRow key={project.id} project={project} onOpen={() => openProject(project.id)} />
            ))}
          </Stack>
        </Section>
      </Stack>

      <Dialog open={picking !== null} onClose={() => setPicking(null)} fullWidth maxWidth="xs">
        {picking && (
          <>
            <DialogTitle>
              {pickerText[picking].title}
              <Typography variant="body2" color="text.secondary" component="div">
                {pickerText[picking].subtitle}
              </Typography>
            </DialogTitle>
            <DialogContent>
              <Typography variant="overline" color="text.secondary">
                {t('home:organization.pickProject', 'Project')}
              </Typography>
              <Stack spacing={1} role="radiogroup" data-testid="project-picker">
                {projects.map((project: Project) => (
                  <PickerOption
                    key={project.id}
                    project={project}
                    checked={chosen === project.id}
                    onChoose={() => setChosen(project.id)}
                  />
                ))}
              </Stack>
              <Typography variant="caption" color="text.secondary" component="div" sx={{mt: 1.5}}>
                {pickerText[picking].note}{' '}
                {canCreateProject && (
                  <Box
                    component={Link}
                    to={RouteConfig.projects.create()}
                    sx={{color: 'primary.main'}}
                    onClick={() => setPicking(null)}
                  >
                    {t('home:organization.needProject', 'Need a new project? Create one.')}
                  </Box>
                )}
              </Typography>
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setPicking(null)}>{t('home:organization.pickCancel', 'Cancel')}</Button>
              <Button variant="contained" onClick={proceed} disabled={!chosen} data-testid="project-picker-continue">
                {t('home:organization.pickContinue', 'Continue')}
              </Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </PageContent>
  );
}
