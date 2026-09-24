// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Heading from '@theme/Heading';
import type {Props as HeadingProps} from '@theme/Heading';
import MDXComponents from '@theme-original/MDXComponents';
import {AndroidLogo, FlutterLogo} from '@thunderid/components';
import {
  Box,
  Card,
  CardContent,
  Typography,
  ColorSchemeSVG,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from '@wso2/oxygen-ui';
import type {ComponentProps} from 'react';
import {AgentAuthorityDiagram} from '@site/src/components/AgentAuthorityDiagram';
import {DelegationMethodSelector, DelegationContent} from '@site/src/components/AgentDelegationMode';
import {AgentInteractionsDiagram} from '@site/src/components/AgentInteractionsDiagram';
import {LangTabs, Lang} from '@site/src/components/AgentLang';
import {AgentModeSelector, Mode} from '@site/src/components/AgentMode';
import {AgentOwnTokenFlow, AgentOboFlow} from '@site/src/components/AgentQuickstartFlow';
import {SignInMethodSelector, SignInMethodContent} from '@site/src/components/AgentSignInMethod';
import {AgentSolutionArchitectureDiagram} from '@site/src/components/AgentSolutionArchitectureDiagram';
import {
  AIAgentIdentityRoadmap as AIAgentIdentityExplorer,
  AIAgentSolutionPatternsRoadmap,
} from '@site/src/components/AIAgentIdentityJourney';
import ApiReference from '@site/src/components/ApiReference';
import ApiVersionReference from '@site/src/components/ApiVersionReference';
import {
  B2CIdentityJourneyExplorer,
  B2CArchitectureDecisions,
  B2CSolutionPatternsExplorer,
  B2CNextSteps,
} from '@site/src/components/B2CIdentityJourney';
import CodeBlock from '@site/src/components/CodeBlock';
import CodeGroup from '@site/src/components/CodeGroup';
import ColorSchemeImage from '@site/src/components/ColorSchemeImage';
import DeploymentCards from '@site/src/components/DeploymentCards';
import DeveloperShortcut from '@site/src/components/DeveloperShortcut';
import DocsGetStarted from '@site/src/components/DocsGetStarted';
import FloatingLogosBackground from '@site/src/components/FloatingLogosBackground';
import {BuildAFlowDiagram, FlowNodeTypesRoadmap, FlowBuildingBlocksRoadmap} from '@site/src/components/FlowConcepts';
import GettingStartedJourney from '@site/src/components/GettingStartedJourney';
import AngularLogo from '@site/src/components/icons/AngularLogo';
import BrowserLogo from '@site/src/components/icons/BrowserLogo';
import ClaudeLogo from '@site/src/components/icons/ClaudeLogo';
import CliLogo from '@site/src/components/icons/CliLogo';
import CodexLogo from '@site/src/components/icons/CodexLogo';
import DockerLogo from '@site/src/components/icons/DockerLogo';
import ExpressLogo from '@site/src/components/icons/ExpressLogo';
import GoLogo from '@site/src/components/icons/GoLogo';
import Html5Logo from '@site/src/components/icons/Html5Logo';
import IOSLogo from '@site/src/components/icons/IOSLogo';
import JavaScriptLogo from '@site/src/components/icons/JavaScriptLogo';
import NextLogo from '@site/src/components/icons/NextLogo';
import NodeLogo from '@site/src/components/icons/NodeLogo';
import NuxtLogo from '@site/src/components/icons/NuxtLogo';
import PythonLogo from '@site/src/components/icons/PythonLogo';
import ReactLogo from '@site/src/components/icons/ReactLogo';
import ReactRouterLogo from '@site/src/components/icons/ReactRouterLogo';
import SkillsLogo from '@site/src/components/icons/SkillsLogo';
import TanStackLogo from '@site/src/components/icons/TanStackLogo';
import VueLogo from '@site/src/components/icons/VueLogo';
import {InfographicTimeline, InfographicStep} from '@site/src/components/InfographicTimeline';
import IntegrationTypePicker from '@site/src/components/IntegrationTypePicker';
import {K8sArchDiagram} from '@site/src/components/K8sArchDiagram';
import {ConsoleUrl, WayFinderSampleUrl, WayFinderMailUrl} from '@site/src/components/LocalUrls';
import {McpOAuthFlowDiagram} from '@site/src/components/McpQuickstartFlow';
import {NextSteps, NextStepsCard} from '@site/src/components/NextSteps';
import {Pattern, PatternPicker} from '@site/src/components/PatternPicker';
import ProductName from '@site/src/components/ProductName';
import RepoLink from '@site/src/components/RepoLink';
import RunThunderID from '@site/src/components/RunThunderID';
import SampleDownload from '@site/src/components/SampleDownload';
import SDKCard from '@site/src/components/SDKCard';
import SdkQuickstartDownload from '@site/src/components/SdkQuickstartDownload';
import {SolutionArchitectureDiagram} from '@site/src/components/SolutionArchitectureDiagram';
import Stepper from '@site/src/components/Stepper';
import TutorialHero, {TutorialHeroItem} from '@site/src/components/TutorialHero';
import UseCaseBranchCards from '@site/src/components/UseCaseBranchCards';
import {UseCaseStepper, UseCaseStepperCard} from '@site/src/components/UseCaseStepper';
import {UseCaseVerticalCards, UseCaseVerticalCard, UseCaseCardSection} from '@site/src/components/UseCaseVerticalCards';
import {
  WayfinderCast,
  WayfinderVcCast,
  WayfinderArchitecture,
  WayfinderVcArchitecture,
  WayfinderAgentOrganization,
  WayfinderAgentArchitecture,
  WayfinderMcpOrganization,
  WayfinderMcpArchitecture,
} from '@site/src/components/WayfinderDiagrams';

type HeadingLevel = HeadingProps['as'];

/**
 * Renders a heading level as an Oxygen UI `Typography` instead of a bare host
 * tag, while still going through `@theme/Heading` underneath: that is what
 * gives a heading its `id`, its `anchor` class, and the hover hash-link, and
 * what registers it with Docusaurus's TOC and broken-anchor checker. Swapping
 * those out for a plain `<Typography variant="h1" {...props} />` (as this
 * file used to note as the eventual fix) drops all of that silently, which is
 * the "styling is a bit off" this replaces.
 *
 * Typography's own variant scale is reset back to the CSS custom properties
 * `custom.css` already drives every heading from (`--ifm-h1-font-size` etc.),
 * rather than duplicating those numbers here, so the two never drift apart.
 */
function typographyHeading(level: HeadingLevel) {
  function TypographyHeading({...props}: ComponentProps<'h1'>) {
    return (
      <Typography
        variant={level}
        component={level}
        sx={{
          fontSize: `var(--ifm-${level}-font-size)`,
          fontWeight: 'var(--ifm-heading-font-weight)',
          lineHeight: 'var(--ifm-heading-line-height)',
          letterSpacing: 'normal',
          marginBottom: 'var(--ifm-heading-margin-bottom)',
        }}
        {...props}
      />
    );
  }
  TypographyHeading.displayName = `TypographyHeading(${level})`;

  // `Stepper` and `TutorialHero` group their MDX children into steps by walking
  // `children` and checking `child.type.name === 'h2'`: theme-original's mapping
  // worked because an anonymous arrow assigned to an object property (`h2: (props)
  // => ...`) has its name inferred from that property by the JS engine. A named
  // `function` does not get renamed that way, so the replacement has to pick the
  // name up explicitly. A computed method key does this correctly (`{[level](){}}`
  // produces a function whose `.name` is the string `level` evaluated to).
  const named = {
    [level](props: Omit<HeadingProps, 'as'>) {
      if (level === 'h1') {
        // `@theme/Heading` recognises "no anchor for this one" by comparing `as`
        // against the literal string `'h1'` (h1 doesn't appear in the TOC), a check
        // that never matches once `as` is a component instead of a level string.
        // Render Typography directly for h1 so it stays anchor-less, as before.
        return <TypographyHeading {...props} id={undefined} />;
      }
      return <Heading as={TypographyHeading as unknown as HeadingLevel} {...props} />;
    },
  };
  return named[level];
}

const h1 = typographyHeading('h1');
const h2 = typographyHeading('h2');
const h3 = typographyHeading('h3');
const h4 = typographyHeading('h4');
const h5 = typographyHeading('h5');
const h6 = typographyHeading('h6');

export default {
  ...MDXComponents,
  h1,
  h2,
  h3,
  h4,
  h5,
  h6,
  Box,
  Card,
  CardContent,
  ColorSchemeSVG,
  ColorSchemeImage,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  DocsGetStarted,
  Stepper,
  TutorialHero,
  TutorialHeroItem,
  SDKCard,
  ReactLogo,
  NextLogo,
  VueLogo,
  NuxtLogo,
  AngularLogo,
  BrowserLogo,
  NodeLogo,
  ExpressLogo,
  GoLogo,
  PythonLogo,
  FlutterLogo,
  IOSLogo,
  JavaScriptLogo,
  AndroidLogo,
  ReactRouterLogo,
  TanStackLogo,
  ApiReference,
  CodeBlock,
  CodeGroup,
  FloatingLogosBackground,
  ProductName,
  ConsoleUrl,
  WayFinderSampleUrl,
  WayFinderMailUrl,
  IntegrationTypePicker,
  RepoLink,
  RunThunderID,
  WayfinderCast,
  WayfinderVcCast,
  WayfinderArchitecture,
  WayfinderVcArchitecture,
  WayfinderAgentOrganization,
  WayfinderAgentArchitecture,
  WayfinderMcpOrganization,
  WayfinderMcpArchitecture,
  NextSteps,
  NextStepsCard,
  B2CIdentityJourneyExplorer,
  B2CArchitectureDecisions,
  B2CSolutionPatternsExplorer,
  B2CNextSteps,
  AIAgentIdentityExplorer,
  AIAgentSolutionPatternsRoadmap,
  AgentOwnTokenFlow,
  AgentOboFlow,
  McpOAuthFlowDiagram,
  LangTabs,
  Lang,
  AgentModeSelector,
  Mode,
  DelegationMethodSelector,
  DelegationContent,
  SignInMethodSelector,
  SignInMethodContent,
  AgentInteractionsDiagram,
  BuildAFlowDiagram,
  FlowNodeTypesRoadmap,
  FlowBuildingBlocksRoadmap,
  K8sArchDiagram,
  ApiVersionReference,
  DeploymentCards,
  DeveloperShortcut,
  GettingStartedJourney,
  SampleDownload,
  SdkQuickstartDownload,
  AgentAuthorityDiagram,
  AgentSolutionArchitectureDiagram,
  SolutionArchitectureDiagram,
  UseCaseBranchCards,
  UseCaseStepper,
  UseCaseStepperCard,
  InfographicTimeline,
  InfographicStep,
  UseCaseVerticalCards,
  UseCaseVerticalCard,
  UseCaseCardSection,
  ClaudeLogo,
  CliLogo,
  CodexLogo,
  DockerLogo,
  Html5Logo,
  SkillsLogo,
  Pattern,
  PatternPicker,
};
