// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AndroidLogo, AppleIcon, FlutterLogo} from '@thunderid/components';
import {MCP} from '@wso2/oxygen-ui-icons-react';
import AndroidTemplate from '../data/application-templates/technology-based/android.json';
import ExpressTemplate from '../data/application-templates/technology-based/express.json';
import FlutterTemplate from '../data/application-templates/technology-based/flutter.json';
import IOSTemplate from '../data/application-templates/technology-based/ios.json';
import MCPClientTemplate from '../data/application-templates/technology-based/mcp-client.json';
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
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 32 32" fill="none">
        <path
          d="M18.6789 15.9759C18.6789 14.5415 17.4796 13.3785 16 13.3785C14.5206 13.3785 13.3211 14.5415 13.3211 15.9759C13.3211 17.4105 14.5206 18.5734 16 18.5734C17.4796 18.5734 18.6789 17.4105 18.6789 15.9759Z"
          fill="#53C1DE"
        />
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M24.7004 11.1537C25.2661 8.92478 25.9772 4.79148 23.4704 3.39016C20.9753 1.99495 17.7284 4.66843 16.0139 6.27318C14.3044 4.68442 10.9663 2.02237 8.46163 3.42814C5.96751 4.82803 6.73664 8.8928 7.3149 11.1357C4.98831 11.7764 1 13.1564 1 15.9759C1 18.7874 4.98416 20.2888 7.29698 20.9289C6.71658 23.1842 5.98596 27.1909 8.48327 28.5877C10.9973 29.9932 14.325 27.3945 16.0554 25.7722C17.7809 27.3864 20.9966 30.0021 23.4922 28.6014C25.9956 27.1963 25.3436 23.1184 24.7653 20.8625C27.0073 20.221 31 18.7523 31 15.9759C31 13.1835 26.9903 11.7923 24.7004 11.1537ZM24.4162 19.667C24.0365 18.5016 23.524 17.2623 22.8971 15.9821C23.4955 14.7321 23.9881 13.5088 24.3572 12.3509C26.0359 12.8228 29.7185 13.9013 29.7185 15.9759C29.7185 18.07 26.1846 19.1587 24.4162 19.667ZM22.85 27.526C20.988 28.571 18.2221 26.0696 16.9478 24.8809C17.7932 23.9844 18.638 22.9422 19.4625 21.7849C20.9129 21.6602 22.283 21.4562 23.5256 21.1777C23.9326 22.7734 24.7202 26.4763 22.85 27.526ZM9.12362 27.5111C7.26143 26.47 8.11258 22.8946 8.53957 21.2333C9.76834 21.4969 11.1286 21.6865 12.5824 21.8008C13.4123 22.9332 14.2816 23.9741 15.1576 24.8857C14.0753 25.9008 10.9945 28.557 9.12362 27.5111ZM2.28149 15.9759C2.28149 13.874 5.94207 12.8033 7.65904 12.3326C8.03451 13.5165 8.52695 14.7544 9.12123 16.0062C8.51925 17.2766 8.01977 18.5341 7.64085 19.732C6.00369 19.2776 2.28149 18.0791 2.28149 15.9759ZM9.1037 4.50354C10.9735 3.45416 13.8747 6.00983 15.1159 7.16013C14.2444 8.06754 13.3831 9.1006 12.5603 10.2265C11.1494 10.3533 9.79875 10.5569 8.55709 10.8297C8.09125 9.02071 7.23592 5.55179 9.1037 4.50354ZM20.3793 11.5771C21.3365 11.6942 22.2536 11.85 23.1147 12.0406C22.8562 12.844 22.534 13.6841 22.1545 14.5453C21.6044 13.5333 21.0139 12.5416 20.3793 11.5771ZM16.0143 8.0481C16.6054 8.66897 17.1974 9.3623 17.7798 10.1145C16.5985 10.0603 15.4153 10.0601 14.234 10.1137C14.8169 9.36848 15.414 8.67618 16.0143 8.0481ZM9.8565 14.5444C9.48329 13.6862 9.16398 12.8424 8.90322 12.0275C9.75918 11.8418 10.672 11.69 11.623 11.5748C10.9866 12.5372 10.3971 13.5285 9.8565 14.5444ZM11.6503 20.4657C10.6679 20.3594 9.74126 20.2153 8.88556 20.0347C9.15044 19.2055 9.47678 18.3435 9.85796 17.4668C10.406 18.4933 11.0045 19.4942 11.6503 20.4657ZM16.0498 23.9915C15.4424 23.356 14.8365 22.6531 14.2448 21.8971C15.4328 21.9423 16.6231 21.9424 17.811 21.891C17.2268 22.6608 16.6369 23.3647 16.0498 23.9915ZM22.1667 17.4222C22.5677 18.3084 22.9057 19.1657 23.1742 19.9809C22.3043 20.1734 21.3652 20.3284 20.3757 20.4435C21.015 19.4607 21.6149 18.4536 22.1667 17.4222ZM18.7473 20.5941C16.9301 20.72 15.1016 20.7186 13.2838 20.6044C12.2509 19.1415 11.3314 17.603 10.5377 16.0058C11.3276 14.4119 12.2404 12.8764 13.2684 11.4158C15.0875 11.2825 16.9178 11.2821 18.7369 11.4166C19.7561 12.8771 20.6675 14.4086 21.4757 15.9881C20.6771 17.5812 19.7595 19.1198 18.7473 20.5941ZM22.8303 4.4666C24.7006 5.51254 23.8681 9.22726 23.4595 10.8426C22.2149 10.5641 20.8633 10.3569 19.4483 10.2281C18.6239 9.09004 17.7698 8.05518 16.9124 7.15949C18.1695 5.98441 20.9781 3.43089 22.8303 4.4666Z"
          fill="#53C1DE"
        />
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
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 353 258" fill="currentColor">
        <g clipPath="url(#express-clip)">
          <path d="M180.477 197.464h20.699l102.5-137.0211h-20.972z" />
          <path d="M323.863 218.64v.045l-59.307-79.008-10.918 14.231 65.413 89.993H13.8905V13.8905H149.028l74.355 98.8675 10.849-14.1398-62.666-84.7277h.046L161.058 0H0v257.792h352.665z" />
          <path d="M160.424 80.4821c-4.88-7.1722-11.19-12.9599-18.998-17.3858-7.785-4.4259-17.181-6.6275-28.144-6.6275s-20.1322 2.0881-28.008 6.2416-14.2991 9.6462-19.247 16.4553-8.6249 14.5714-11.0307 23.2413c-2.3832 8.671-3.5861 17.522-3.5861 26.556 0 9.737 1.2029 18.997 3.5861 27.758s6.0601 16.455 11.0307 23.106c4.9479 6.65 11.3712 11.87 19.247 15.661 7.8758 3.813 17.204 5.719 28.008 5.719 17.34 0 30.754-4.244 40.242-12.756 9.464-8.488 15.978-20.54 19.519-36.11h-16.728c-2.655 10.622-7.444 19.11-14.344 25.488s-16.455 9.556-28.689 9.556c-7.967 0-14.7757-1.68-20.4499-5.039-5.6743-3.359-10.3498-7.603-14.0721-12.756-3.7223-5.129-6.4233-10.94-8.1028-17.386-1.498-5.719-2.3151-11.257-2.4967-16.636-.0454-1.408-.1135-2.792-.2043-4.199-.295-4.154-.2496-8.217.2043-12.189q1.0554-9.192 4.3578-17.976c2.2016-5.9235 5.1749-11.1438 8.8972-15.6605q5.58345-6.77505 13.5501-10.8945c5.3114-2.7463 11.4164-4.1081 18.3164-4.1081s12.756 1.3618 18.067 4.1081 9.828 6.3778 13.55 10.8945 6.582 9.6916 8.625 15.5245c2.042 5.833 3.132 11.962 3.313 18.317H82.6638l.0681 14.072h90.8331c.363-9.556-.522-18.998-2.655-28.281-2.134-9.2827-5.629-17.5217-10.486-24.6939" />
        </g>
        <defs>
          <clipPath id="express-clip">
            <path d="M0 0h352.665v257.792H0z" />
          </clipPath>
        </defs>
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
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="currentColor">
        <path d="M11.2141 0.00645944C11.1625 0.0111515 10.9982 0.0275738 10.8504 0.039304C7.44164 0.346635 4.24868 2.18593 2.22639 5.01291C1.10029 6.58476 0.380059 8.36775 0.107918 10.2563C0.0117302 10.9156 0 11.1103 0 12.0041C0 12.898 0.0117302 13.0927 0.107918 13.7519C0.760117 18.2587 3.96716 22.0452 8.31672 23.4481C9.0956 23.6991 9.91672 23.8704 10.8504 23.9736C11.2141 24.0135 12.7859 24.0135 13.1496 23.9736C14.7613 23.7953 16.1267 23.3965 17.4733 22.7091C17.6798 22.6035 17.7196 22.5754 17.6915 22.5519C17.6727 22.5378 16.793 21.3578 15.7372 19.9314L13.8182 17.339L11.4135 13.7801C10.0903 11.8235 9.00176 10.2235 8.99238 10.2235C8.98299 10.2211 8.97361 11.8024 8.96891 13.7331C8.96188 17.1138 8.95953 17.2499 8.9173 17.3296C8.85631 17.4446 8.80938 17.4915 8.71085 17.5431C8.63578 17.5807 8.57009 17.5877 8.21584 17.5877H7.80997L7.70205 17.5197C7.63167 17.4751 7.58006 17.4164 7.54487 17.3484L7.4956 17.2428L7.50029 12.539L7.50733 7.83285L7.58006 7.74136C7.6176 7.69209 7.69736 7.62875 7.75367 7.59825C7.84985 7.55133 7.88739 7.54664 8.29325 7.54664C8.77185 7.54664 8.85161 7.5654 8.97595 7.70147C9.01114 7.73901 10.3132 9.7003 11.871 12.0628C13.4287 14.4252 15.5589 17.651 16.6053 19.2346L18.5056 22.1132L18.6018 22.0499C19.4534 21.4962 20.3543 20.7079 21.0674 19.8868C22.5853 18.1437 23.5636 16.0182 23.8921 13.7519C23.9883 13.0927 24 12.898 24 12.0041C24 11.1103 23.9883 10.9156 23.8921 10.2563C23.2399 5.74957 20.0328 1.96306 15.6833 0.560125C14.9161 0.311445 14.0997 0.140184 13.1848 0.036958C12.9595 0.0134976 11.4088 -0.0123089 11.2141 0.00645944ZM16.1267 7.26511C16.2393 7.32142 16.3308 7.42933 16.3636 7.54194C16.3824 7.60294 16.3871 8.90734 16.3824 11.8469L16.3754 16.0651L15.6317 14.9249L14.8856 13.7848V10.7185C14.8856 8.73608 14.895 7.62171 14.9091 7.56775C14.9466 7.43637 15.0287 7.33315 15.1413 7.27215C15.2375 7.22288 15.2727 7.21819 15.6411 7.21819C15.9883 7.21819 16.0493 7.22288 16.1267 7.26511Z" />
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
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="40"
        height="40"
        viewBox="0 0 256 256"
        preserveAspectRatio="xMinYMin meet"
        role="img"
        aria-label="JavaScript logo"
      >
        <path d="M0 0h256v256H0V0z" fill="#F7DF1E" />
        <path d="M67.312 213.932l19.59-11.856c3.78 6.701 7.218 12.371 15.465 12.371 7.905 0 12.89-3.092 12.89-15.12v-81.798h24.057v82.138c0 24.917-14.606 36.259-35.916 36.259-19.245 0-30.416-9.967-36.087-21.996M152.381 211.354l19.588-11.341c5.157 8.421 11.859 14.607 23.715 14.607 9.969 0 16.325-4.984 16.325-11.858 0-8.248-6.53-11.17-17.528-15.98l-6.013-2.58c-17.357-7.387-28.87-16.667-28.87-36.257 0-18.044 13.747-31.792 35.228-31.792 15.294 0 26.292 5.328 34.196 19.247L210.29 147.43c-4.125-7.389-8.591-10.31-15.465-10.31-7.046 0-11.514 4.468-11.514 10.31 0 7.217 4.468 10.14 14.778 14.608l6.014 2.577c20.45 8.765 31.963 17.7 31.963 37.804 0 21.654-17.012 33.51-39.867 33.51-22.339 0-36.774-10.654-43.819-24.574" />
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
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 196.32 170.02">
        <path fill="#42b883" d="M120.83 0L98.16 39.26 75.49 0H0l98.16 170.02L196.32 0h-75.49z" />
        <path fill="#35495e" d="M120.83 0L98.16 39.26 75.49 0H39.26l58.9 102.01L157.06 0h-36.23z" />
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
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 300 300">
        <path
          fill="#00DC82"
          d="M168 250h111c3.542 0 6.932-1.244 10-3 3.068-1.756 6.23-3.959 8-7 1.77-3.041 3.002-6.49 3-10.001-.002-3.511-1.227-6.959-3-9.998L222 91c-1.77-3.04-3.933-5.245-7-7s-7.458-3-11-3-6.933 1.245-10 3-5.23 3.96-7 7l-19 33-38-64.002c-1.772-3.04-3.932-6.243-7-7.998s-6.458-2-10-2-6.932.245-10 2c-3.068 1.755-6.228 4.958-8 7.998L2 220c-1.773 3.039-1.998 6.487-2 9.998-.002 3.511.23 6.96 2 10.001 1.77 3.04 4.932 5.244 8 7 3.068 1.756 6.458 3 10 3h70c27.737 0 47.925-12.442 62-36l34-59 18-31 55 94h-73l-18 32Zm-79-32H40l73-126 37 63-24.509 42.725C116.144 213.01 105.488 218 89 218Z"
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
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 44 50">
        <defs>
          <clipPath id="hexClip">
            <path d="M22.8725 0.4166C22.136 0 21.2616 0 20.5253 0.4166L1.1505 11.6694C0.4142 12.0862 0 12.8733 0 13.707V36.2584C0 37.0921 0.4602 37.8791 1.1505 38.296L20.5253 49.5487C21.2616 49.9653 22.136 49.9653 22.8725 49.5487L42.2471 38.296C42.9836 37.8791 43.3976 37.0921 43.3976 36.2584V13.707C43.3976 12.8733 42.9375 12.0862 42.2471 11.6694L22.8725 0.4166Z" />
          </clipPath>
          <linearGradient id="main" x1="30.33" y1="8.56" x2="14.9" y2="44.7" gradientUnits="userSpaceOnUse">
            <stop stopColor="#3F8B3D" />
            <stop offset="0.64" stopColor="#3F873F" />
            <stop offset="0.93" stopColor="#3DA92E" />
            <stop offset="1" stopColor="#3DAE2B" />
          </linearGradient>
          <linearGradient id="r1" x1="18.8" y1="26.8" x2="68" y2="0.4" gradientUnits="userSpaceOnUse">
            <stop offset="0.14" stopColor="#3F873F" />
            <stop offset="0.4" stopColor="#52A044" />
            <stop offset="0.71" stopColor="#64B749" />
            <stop offset="0.91" stopColor="#6ABF4B" />
          </linearGradient>
          <linearGradient id="r2" x1="0.25" y1="24.5" x2="44" y2="24.5" gradientUnits="userSpaceOnUse">
            <stop offset="0.09" stopColor="#6ABF4B" />
            <stop offset="0.29" stopColor="#64B749" />
            <stop offset="0.6" stopColor="#52A044" />
            <stop offset="0.86" stopColor="#3F873F" />
          </linearGradient>
        </defs>
        <path
          fill="url(#main)"
          d="M22.8725 0.4166C22.136 0 21.2616 0 20.5253 0.4166L1.1505 11.6694C0.4142 12.0862 0 12.8733 0 13.707V36.2584C0 37.0921 0.4602 37.8791 1.1505 38.296L20.5253 49.5487C21.2616 49.9653 22.136 49.9653 22.8725 49.5487L42.2471 38.296C42.9836 37.8791 43.3976 37.0921 43.3976 36.2584V13.707C43.3976 12.8733 42.9375 12.0862 42.2471 11.6694L22.8725 0.4166Z"
        />
        <polygon
          fill="url(#r1)"
          clipPath="url(#hexClip)"
          points="21.698901,-1.046618 43.20532,11.948247 21.698901,51.072715 0.152778,38.055107"
        />
        <polygon
          fill="url(#r2)"
          clipPath="url(#hexClip)"
          points="21.698901,-1.046618 0.152778,11.948247 21.698901,51.072715 43.20532,38.055107"
        />
      </svg>
    ),
    titleKey: 'applications:onboarding.configure.stack.technology.nodejs.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.nodejs.description',
    template: NodeTemplate as ApplicationTemplate,
    categories: ['backend'],
  },
  {
    value: TechnologyApplicationTemplate.IOS,
    icon: <AppleIcon size={40} />,
    titleKey: 'applications:onboarding.configure.stack.technology.ios.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.ios.description',
    template: IOSTemplate as ApplicationTemplate,
    categories: ['mobile'],
  },
  {
    value: TechnologyApplicationTemplate.ANDROID,
    icon: <AndroidLogo size={40} />,
    titleKey: 'applications:onboarding.configure.stack.technology.android.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.android.description',
    template: AndroidTemplate as ApplicationTemplate,
    categories: ['mobile'],
  },
  {
    value: TechnologyApplicationTemplate.FLUTTER,
    icon: <FlutterLogo size={40} />,
    titleKey: 'applications:onboarding.configure.stack.technology.flutter.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.flutter.description',
    template: FlutterTemplate as ApplicationTemplate,
    categories: ['mobile'],
  },
  {
    value: TechnologyApplicationTemplate.MCP_CLIENT,
    icon: <MCP size={40} />,
    titleKey: 'applications:onboarding.configure.stack.technology.mcpClient.title',
    descriptionKey: 'applications:onboarding.configure.stack.technology.mcpClient.description',
    template: MCPClientTemplate as ApplicationTemplate,
    categories: ['ai'],
  },
];

export default TechnologyBasedApplicationTemplateMetadata;
