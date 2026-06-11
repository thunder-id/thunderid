/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Monitor, Server, Smartphone, Code, Settings} from '@wso2/oxygen-ui-icons-react';
import BackendPlatformTemplate from '../data/application-templates/platform-based/backend.json';
import BrowserPlatformTemplate from '../data/application-templates/platform-based/browser.json';
import CustomPlatformTemplate from '../data/application-templates/platform-based/custom.json';
import FullStackPlatformTemplate from '../data/application-templates/platform-based/full-stack.json';
import MobilePlatformTemplate from '../data/application-templates/platform-based/mobile.json';
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
    template: BackendPlatformTemplate as ApplicationTemplate,
    categories: ['backend'],
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
