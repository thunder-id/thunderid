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
import NodeTemplate from '../data/application-templates/technology-based/node.json';
import NuxtTemplate from '../data/application-templates/technology-based/nuxt.json';
import ReactTemplate from '../data/application-templates/technology-based/react.json';
import VanillaJSTemplate from '../data/application-templates/technology-based/vanilla-js.json';
import VueTemplate from '../data/application-templates/technology-based/vue.json';
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
    categories: ['web'],
  },
  {
    value: TechnologyApplicationTemplate.EXPRESS,
    icon: (
      <svg width="40" height="40" viewBox="0 0 20 20" style={{fill: 'white'}}>
        <path d="M6.504 7.181c1.47 0 1.812 1.29 1.812 2.108H4.5c.103-.906.683-2.108 2.004-2.108Z" />
        <path
          fillRule="evenodd"
          d="M10 20a10 10 0 1 0 0-20 10 10 0 0 0 0 20Zm-3.05-7.291c-1.321 0-2.438-.728-2.464-2.492l5.032.013c.04-.2.062-.405.058-.61 0-1.32-.621-3.37-2.955-3.37-2.109 0-3.385 1.737-3.385 3.875 0 2.137 1.328 3.625 3.535 3.625a5.738 5.738 0 0 0 2.39-.475l-.223-.938a4.65 4.65 0 0 1-1.988.372Zm5.833-4.78L11.759 6.4h-1.455l2.437 3.505-2.555 3.666h1.439l1.04-1.604a26.7 26.7 0 0 1 .261-.425c.171-.274.336-.538.494-.837h.031l.023.037c.245.413.479.807.75 1.225l1.067 1.604h1.471L14.238 9.86l2.45-3.46h-1.425l-.995 1.514c-.096.157-.194.312-.293.47-.146.231-.294.465-.435.704h-.03l-.165-.273c-.176-.291-.35-.58-.563-.887Z"
        />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.express.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.express.description',
    template: ExpressTemplate as ApplicationTemplate,
    categories: ['backend'],
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
    categories: ['web', 'backend'],
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
    categories: ['web'],
  },
  {
    value: TechnologyApplicationTemplate.VUE,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 261.76 226.69">
        <path d="M161.096.001l-30.225 52.351L100.647.001H0l130.871 226.688L261.742.001z" fill="#41b883" />
        <path d="M161.096.001l-30.225 52.351L100.647.001H52.346l78.525 136.01L209.398.001z" fill="#34495e" />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.vue.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.vue.description',
    template: VueTemplate as ApplicationTemplate,
    categories: ['web'],
  },
  {
    value: TechnologyApplicationTemplate.NUXT,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 221 120">
        <path
          d="M120.81 120H212.7c1.903 0 3.773-.498 5.408-1.442a10.827 10.827 0 003.977-3.92 10.657 10.657 0 001.458-5.36c0-1.889-.5-3.745-1.458-5.36L166.037 19.2a10.827 10.827 0 00-3.977-3.92 10.978 10.978 0 00-10.816 0 10.827 10.827 0 00-3.977 3.92l-9.684 16.704-18.664-32.28A10.827 10.827 0 00114.942 0a10.978 10.978 0 00-10.816 0 10.827 10.827 0 00-3.977 3.92L1.458 104.008A10.697 10.697 0 000 109.278a10.657 10.657 0 001.458 5.36 10.827 10.827 0 003.977 3.92A10.978 10.978 0 0010.843 120H67.89c21.187 0 36.72-9.152 47.248-26.88L140.47 51.2l12.94 22.4-21.6 37.36C125.097 118.18 113.433 120 120.81 120zm-58.168-21.28l-36.19-.08 72.368-125.2 18.096 31.28-25.936 44.8c-8.784 14.56-18.37 49.2-28.338 49.2z"
          fill="#00dc82"
        />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.nuxt.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.nuxt.description',
    template: NuxtTemplate as ApplicationTemplate,
    categories: ['web', 'backend'],
  },
  {
    value: TechnologyApplicationTemplate.NODEJS,
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 256 289">
        <path
          d="M128 288.774c-3.975 0-7.685-1.06-11.13-2.915l-35.247-20.936c-5.3-2.915-2.65-3.975-1.06-4.505 7.155-2.385 8.48-2.915 15.9-7.155.795-.53 1.856-.265 2.65.265l27.032 16.166c1.06.53 2.385.53 3.18 0l105.74-61.082c1.06-.53 1.59-1.59 1.59-2.915V94.28c0-1.325-.53-2.385-1.59-2.915L128.795 30.55c-1.06-.53-2.385-.53-3.18 0L19.875 91.365c-1.06.53-1.59 1.855-1.59 2.915v122.165c0 1.06.53 2.385 1.59 2.915l28.887 16.696c15.635 7.95 25.442-1.325 25.442-10.6V107.35c0-1.59 1.325-3.18 3.18-3.18h13.515c1.59 0 3.18 1.325 3.18 3.18v118.11c0 20.936-11.395 32.861-31.271 32.861-6.095 0-10.865 0-24.382-6.625L10.07 235.2A22.312 22.312 0 010 216.46V94.28c0-7.685 4.24-14.84 11.13-18.815L116.87 14.648c6.625-3.975 15.635-3.975 22.26 0L244.87 75.465c6.89 3.975 11.13 11.13 11.13 18.815V216.46c0 7.685-4.24 14.84-11.13 18.815L139.13 295.93c-3.445 1.59-7.155 2.384-11.13 2.384v-.265z"
          fill="#539E43"
        />
        <path
          d="M163.573 215.666c-46.217 0-55.757-21.2-55.757-39.22 0-1.59 1.325-3.18 3.18-3.18h13.78c1.59 0 2.915 1.06 3.18 2.65 2.12 14.31 8.48 21.466 35.882 21.466 22.26 0 31.536-5.035 31.536-16.96 0-6.89-2.65-11.925-37.472-15.37-29.152-2.915-47.277-9.275-47.277-32.596 0-21.466 18.12-34.187 48.337-34.187 33.921 0 50.617 11.66 52.737 37.207 0 .795-.265 1.59-.795 2.12-.53.53-1.325.795-2.12.795h-13.78c-1.325 0-2.65-1.06-2.915-2.385-3.18-14.575-11.13-19.345-33.127-19.345-24.382 0-27.297 8.48-27.297 14.84 0 7.685 3.445 10.07 36.412 14.31 32.7 4.24 48.602 10.335 48.602 33.391 0 23.32-19.345 36.572-53.266 36.572l-.62-.109z"
          fill="#539E43"
        />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.nodejs.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.nodejs.description',
    template: NodeTemplate as ApplicationTemplate,
    categories: ['backend'],
  },
];

export default TechnologyBasedApplicationTemplateMetadata;
