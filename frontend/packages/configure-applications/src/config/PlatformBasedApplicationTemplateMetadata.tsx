// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Monitor, Server, Smartphone, Code, Settings, Wallet} from '@wso2/oxygen-ui-icons-react';
import BackendPlatformTemplate from '../data/application-templates/platform-based/backend.json';
import BrowserPlatformTemplate from '../data/application-templates/platform-based/browser.json';
import CustomPlatformTemplate from '../data/application-templates/platform-based/custom.json';
import FullStackPlatformTemplate from '../data/application-templates/platform-based/full-stack.json';
import MobilePlatformTemplate from '../data/application-templates/platform-based/mobile.json';
import WalletPlatformTemplate from '../data/application-templates/platform-based/wallet.json';
import type {ApplicationTemplate, ApplicationTemplateMetadata} from '../models/application-templates';
import {PlatformApplicationTemplate} from '../models/application-templates';

const PlatformBasedApplicationTemplateMetadata: ApplicationTemplateMetadata<PlatformApplicationTemplate>[] = [
  {
    value: PlatformApplicationTemplate.BROWSER,
    icon: <Monitor size={32} />,
    titleKey: 'applications:onboarding.configure.stack.platform.browser.title',
    descriptionKey: 'applications:onboarding.configure.stack.platform.browser.description',
    template: BrowserPlatformTemplate as ApplicationTemplate,
    categories: ['web'],
  },
  {
    value: PlatformApplicationTemplate.FULL_STACK,
    icon: <Server size={32} />,
    titleKey: 'applications:onboarding.configure.stack.platform.full_stack.title',
    descriptionKey: 'applications:onboarding.configure.stack.platform.full_stack.description',
    template: FullStackPlatformTemplate as ApplicationTemplate,
    categories: ['web', 'backend'],
  },
  {
    value: PlatformApplicationTemplate.MOBILE,
    icon: <Smartphone size={32} />,
    titleKey: 'applications:onboarding.configure.stack.platform.mobile.title',
    descriptionKey: 'applications:onboarding.configure.stack.platform.mobile.description',
    template: MobilePlatformTemplate as ApplicationTemplate,
    categories: ['mobile'],
  },
  {
    value: PlatformApplicationTemplate.BACKEND,
    icon: <Code size={32} />,
    titleKey: 'applications:onboarding.configure.stack.platform.backend.title',
    descriptionKey: 'applications:onboarding.configure.stack.platform.backend.description',
    template: BackendPlatformTemplate as unknown as ApplicationTemplate,
    categories: ['backend'],
  },
  {
    value: PlatformApplicationTemplate.WALLET,
    icon: <Wallet size={32} />,
    titleKey: 'applications:onboarding.configure.stack.platform.wallet.title',
    descriptionKey: 'applications:onboarding.configure.stack.platform.wallet.description',
    template: WalletPlatformTemplate as ApplicationTemplate,
    categories: ['mobile'],
  },
  {
    value: PlatformApplicationTemplate.CUSTOM,
    icon: <Settings size={32} />,
    titleKey: 'applications:onboarding.configure.stack.platform.custom.title',
    descriptionKey: 'applications:onboarding.configure.stack.platform.custom.description',
    template: CustomPlatformTemplate as ApplicationTemplate,
    categories: ['web', 'backend', 'mobile'],
  },
];

export default PlatformBasedApplicationTemplateMetadata;
