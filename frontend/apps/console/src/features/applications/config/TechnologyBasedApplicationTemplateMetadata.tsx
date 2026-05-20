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

import ExpressTemplate from '../data/application-templates/technology-based/express.json';
import NextJSTemplate from '../data/application-templates/technology-based/nextjs.json';
import ReactTemplate from '../data/application-templates/technology-based/react.json';
import VanillaJSTemplate from '../data/application-templates/technology-based/vanilla-js.json';
import type {ApplicationTemplate, ApplicationTemplateMetadata} from '../models/application-templates';
import {TechnologyApplicationTemplate} from '../models/application-templates';

const TechnologyBasedApplicationTemplateMetadata: ApplicationTemplateMetadata<TechnologyApplicationTemplate>[] = [
  {
    value: TechnologyApplicationTemplate.REACT,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 16 16" fill="#149ECA">
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M2.769 5.92C1.414 6.53.762 7.295.762 7.999c0 .703.652 1.467 2.007 2.077 1.319.593 3.168.97 5.231.97 2.063 0 3.912-.377 5.231-.97 1.355-.61 2.007-1.374 2.007-2.077 0-.704-.652-1.468-2.007-2.077C11.912 5.327 10.063 4.95 8 4.95c-2.063 0-3.912.377-5.231.97Zm-.313-.694C3.895 4.579 5.855 4.188 8 4.188c2.145 0 4.105.39 5.544 1.038C14.946 5.857 16 6.808 16 7.998c0 1.19-1.054 2.14-2.456 2.771-1.439.648-3.399 1.038-5.544 1.038-2.145 0-4.105-.39-5.544-1.038C1.054 10.14 0 9.188 0 7.998s1.054-2.141 2.456-2.772Z"
        />
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M7.183 2.429c-1.205-.869-2.193-1.052-2.802-.7-.61.352-.945 1.298-.795 2.777.145 1.439.743 3.229 1.775 5.015 1.031 1.787 2.282 3.2 3.456 4.045 1.205.869 2.193 1.052 2.802.7.61-.352.945-1.298.795-2.777-.145-1.439-.743-3.229-1.775-5.015-1.031-1.787-2.282-3.2-3.456-4.045Zm.445-.618c1.28.922 2.598 2.424 3.671 4.282 1.073 1.857 1.715 3.75 1.873 5.32.155 1.53-.142 2.918-1.172 3.513-1.03.595-2.38.158-3.629-.741-1.28-.923-2.598-2.425-3.67-4.283-1.073-1.857-1.715-3.75-1.873-5.32C2.673 3.052 2.969 1.664 4 1.07c1.03-.595 2.38-.157 3.628.742Z"
        />
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M12.414 4.506c.15-1.478-.186-2.425-.795-2.777-.61-.352-1.597-.169-2.802.7-1.174.845-2.425 2.258-3.456 4.045-1.032 1.786-1.63 3.576-1.775 5.015-.15 1.479.186 2.425.795 2.777.61.352 1.597.169 2.802-.7 1.174-.845 2.425-2.258 3.456-4.045 1.032-1.786 1.63-3.576 1.775-5.015Zm.758.076c-.158 1.57-.8 3.463-1.873 5.32-1.072 1.858-2.39 3.36-3.67 4.283-1.248.899-2.598 1.336-3.629.74-1.03-.594-1.327-1.982-1.172-3.512.158-1.57.8-3.462 1.873-5.32 1.072-1.858 2.39-3.36 3.67-4.282C9.62.91 10.97.474 12 1.069c1.03.595 1.327 1.983 1.172 3.513Z"
        />
        <path d="M8 9.521a1.524 1.524 0 1 0 0-3.047A1.524 1.524 0 0 0 8 9.52Z" />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.react.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.react.description',
    template: ReactTemplate as ApplicationTemplate,
  },
  {
    value: TechnologyApplicationTemplate.EXPRESS,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 16 16" fill="none">
        <rect width="16" height="16" rx="4" fill="#111827" />
        <path d="M4.25 5.25h7.5v1h-6.25v1.75h5.75v1h-5.75v1.75h6.25v1h-7.5v-6.5Z" fill="#fff" />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.express.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.express.description',
    template: ExpressTemplate as ApplicationTemplate,
  },
  {
    value: TechnologyApplicationTemplate.NEXTJS,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 16 16" fill="none">
        <path d="M8 15.733A7.733 7.733 0 1 0 8 .267a7.733 7.733 0 0 0 0 15.466Z" fill="#000" />
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M8 .533a7.467 7.467 0 1 0 0 14.934A7.467 7.467 0 0 0 8 .533ZM0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Z"
          fill="#fff"
        />
        <path
          d="M13.29 14.002 6.146 4.8H4.8v6.397h1.077v-5.03l6.567 8.486c.297-.198.58-.416.846-.651Z"
          fill="url(#b)"
        />
        <path d="M11.289 4.8h-1.067v6.4h1.067V4.8Z" fill="url(#c)" />
        <defs>
          <linearGradient id="b" x1="9.689" y1="10.355" x2="12.845" y2="14.267" gradientUnits="userSpaceOnUse">
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#fff" stopOpacity="0" />
          </linearGradient>
          <linearGradient id="c" x1="10.755" y1="4.8" x2="10.738" y2="9.5" gradientUnits="userSpaceOnUse">
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#fff" stopOpacity="0" />
          </linearGradient>
        </defs>
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.nextjs.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.nextjs.description',
    template: NextJSTemplate as ApplicationTemplate,
    disabled: true,
  },
  {
    value: TechnologyApplicationTemplate.VANILLA_JS,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 256 256">
        <rect width="256" height="256" fill="#F7DF1E" />
        <path d="M67.312 213.932l19.59-11.856c3.78 6.701 7.218 12.371 15.465 12.371 7.905 0 12.89-3.092 12.89-15.12v-81.798h24.057v82.138c0 24.917-14.606 36.259-35.916 36.259-19.245 0-30.416-9.967-36.087-21.996M152.381 211.354l19.588-11.341c5.157 8.421 11.859 14.607 23.715 14.607 9.969 0 16.325-4.984 16.325-11.858 0-8.248-6.53-11.17-17.528-15.98l-6.013-2.58c-17.357-7.387-28.87-16.667-28.87-36.257 0-18.044 13.747-31.792 35.228-31.792 15.294 0 26.292 5.328 34.196 19.247l-18.732 12.03c-4.125-7.389-8.591-10.31-15.465-10.31-7.046 0-11.514 4.468-11.514 10.31 0 7.217 4.468 10.14 14.778 14.608l6.014 2.577c20.45 8.765 31.963 17.7 31.963 37.804 0 21.654-17.012 33.51-39.867 33.51-22.339 0-36.774-10.654-43.819-24.574" />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.vanillaJs.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.vanillaJs.description',
    template: VanillaJSTemplate as ApplicationTemplate,
  },
];

export default TechnologyBasedApplicationTemplateMetadata;
