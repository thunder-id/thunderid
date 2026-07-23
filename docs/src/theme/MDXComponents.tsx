/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import MDXComponents from '@theme-original/MDXComponents';
import {Box, Card, CardContent, Typography, ColorSchemeSVG, Table, TableBody, TableCell, TableContainer, TableHead, TableRow} from '@wso2/oxygen-ui';
import {AgentInteractionsDiagram} from '@site/src/components/AgentInteractionsDiagram';
import {LangTabs, Lang} from '@site/src/components/AgentLang';
import {AgentModeSelector, Mode} from '@site/src/components/AgentMode';
import {AgentOwnTokenFlow, AgentOboFlow} from '@site/src/components/AgentQuickstartFlow';
import {AIAgentIdentityRoadmap as AIAgentIdentityExplorer, AIAgentSolutionPatternsRoadmap} from '@site/src/components/AIAgentIdentityJourney';
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
import AndroidLogo from '@site/src/components/icons/AndroidLogo';
import AngularLogo from '@site/src/components/icons/AngularLogo';
import BrowserLogo from '@site/src/components/icons/BrowserLogo';
import ClaudeLogo from '@site/src/components/icons/ClaudeLogo';
import CliLogo from '@site/src/components/icons/CliLogo';
import CodexLogo from '@site/src/components/icons/CodexLogo';
import DockerLogo from '@site/src/components/icons/DockerLogo';
import ExpressLogo from '@site/src/components/icons/ExpressLogo';
import FlutterLogo from '@site/src/components/icons/FlutterLogo';
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
import {NextSteps, NextStepsCard} from '@site/src/components/NextSteps';
import ProductName from '@site/src/components/ProductName';
import RepoLink from '@site/src/components/RepoLink';
import RunThunderID from '@site/src/components/RunThunderID';
import SampleDownload from '@site/src/components/SampleDownload';
import SDKCard from '@site/src/components/SDKCard';
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

export default {
  ...MDXComponents,
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
  LangTabs,
  Lang,
  AgentModeSelector,
  Mode,
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
  // TODO: Heading styling is a bit off when oxygen-ui Typography is used.
  // After sorting that out, we can switch to using Oxygen UI Typography for headings as well.
  // ex: h1: (props: TypographyProps<'h1'>) => <Typography variant="h1" {...props} />,
};
