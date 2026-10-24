// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Pages
export {default as HomePage} from './pages/HomePage';
export {default as WelcomePage} from './pages/WelcomePage';
export {default as GetStartedPage} from './pages/GetStartedPage';
export {default as CreateProjectPage} from './pages/CreateProjectPage';
export {default as TryoutSecuringApplicationPage} from './pages/TryoutSecuringApplicationPage';
export {default as TryoutSecuringAIAgentsPage} from './pages/TryoutSecuringAIAgentsPage';
export {default as TryoutSecuringMCPPage} from './pages/TryoutSecuringMCPPage';

// Components
export {default as WelcomeRedirect} from './components/welcome/WelcomeRedirect';

// Hooks
export {default as useWelcomeClose} from './hooks/useWelcomeClose';

// Sample bundles
export {default as SampleBundlesContext} from './contexts/SampleBundlesContext';
export {default as SampleBundlesProvider, type SampleBundlesProviderProps} from './contexts/SampleBundlesProvider';
export {useGetSampleBundles, useGetSampleBundle} from './api/useGetSampleBundles';
export {buildRegistry, bundleKeyFromPath} from './utils/sampleBundles';
export type {SampleBundle, SampleBundleConfigs} from './utils/sampleBundles';

// Routes
export type {OnboardingRoutePaths} from './hooks/useOnboardingRoutes';
export {defaultOnboardingRoutePaths, default as useOnboardingRoutes} from './hooks/useOnboardingRoutes';
