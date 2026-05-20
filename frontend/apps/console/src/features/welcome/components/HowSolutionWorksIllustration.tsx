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

import {useConfig} from '@thunderid/contexts';
import React from 'react';
import {useTranslation} from 'react-i18next';

const HowSolutionWorksIllustration: React.FC = () => {
  const {config} = useConfig();
  const {t} = useTranslation();

  const {brand} = config;
  const {product_name: productName} = brand || {};

  return (
    <svg width="1237" height="350" viewBox="0 0 1237 350" fill="none" xmlns="http://www.w3.org/2000/svg">
      <g clipPath="url(#clip0_26_106)">
        <path
          d="M276.25 109.5V144M207.25 144H379.75M241.75 109.5V144M224.5 109.5H362.5C372.027 109.5 379.75 117.223 379.75 126.75V230.25C379.75 239.777 372.027 247.5 362.5 247.5H224.5C214.973 247.5 207.25 239.777 207.25 230.25V126.75C207.25 117.223 214.973 109.5 224.5 109.5Z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M285.025 195.75L290.175 200.9L300.475 190.6M318.5 195.75C318.5 209.971 306.971 221.5 292.75 221.5C278.529 221.5 267 209.971 267 195.75C267 181.529 278.529 170 292.75 170C306.971 170 318.5 181.529 318.5 195.75Z"
          stroke="primary"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <text fill="muted" xmlSpace="preserve" fontSize="16" fontWeight="500" letterSpacing="0em">
          <tspan x="235.211" y="83.3182">
            {t('howSolutionWorksIllustration:validateTest')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="12" letterSpacing="0em" textAnchor="middle">
          <tspan x="291.027" y="275.864">
            {t('howSolutionWorksIllustration:runtimeLocal', {productName})}
          </tspan>
        </text>
        <path
          d="M86.25 109.5V144M17.25 144H189.75M51.75 109.5V144M34.5 109.5H172.5C182.027 109.5 189.75 117.223 189.75 126.75V230.25C189.75 239.777 182.027 247.5 172.5 247.5H34.5C24.9731 247.5 17.25 239.777 17.25 230.25V126.75C17.25 117.223 24.9731 109.5 34.5 109.5Z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M96.8041 171.112C96.9625 169.445 97.7366 167.897 98.975 166.771C100.214 165.645 101.827 165.021 103.501 165.021C105.175 165.021 106.789 165.645 108.028 166.771C109.266 167.897 110.04 169.445 110.199 171.112C110.294 172.188 110.647 173.226 111.228 174.137C111.81 175.048 112.602 175.806 113.538 176.346C114.474 176.886 115.527 177.193 116.606 177.24C117.686 177.287 118.761 177.074 119.741 176.617C121.262 175.927 122.986 175.827 124.577 176.337C126.167 176.847 127.512 177.931 128.347 179.378C129.183 180.824 129.451 182.53 129.098 184.163C128.745 185.796 127.798 187.239 126.44 188.212C125.555 188.833 124.833 189.657 124.335 190.616C123.836 191.574 123.576 192.639 123.576 193.719C123.576 194.8 123.836 195.864 124.335 196.823C124.833 197.781 125.555 198.606 126.44 199.226C127.798 200.199 128.745 201.642 129.098 203.275C129.451 204.908 129.183 206.614 128.347 208.061C127.512 209.507 126.167 210.591 124.577 211.101C122.986 211.612 121.262 211.512 119.741 210.821C118.761 210.365 117.686 210.151 116.606 210.198C115.527 210.246 114.474 210.552 113.538 211.092C112.602 211.632 111.81 212.39 111.228 213.301C110.647 214.212 110.294 215.25 110.199 216.327C110.04 217.993 109.266 219.541 108.028 220.667C106.789 221.793 105.175 222.418 103.501 222.418C101.827 222.418 100.214 221.793 98.975 220.667C97.7366 219.541 96.9625 217.993 96.8041 216.327C96.7091 215.25 96.3559 214.212 95.7745 213.3C95.1931 212.389 94.4005 211.631 93.464 211.091C92.5275 210.55 91.4746 210.244 90.3945 210.197C89.3144 210.15 88.2389 210.364 87.2591 210.821C85.7379 211.512 84.0142 211.612 82.4234 211.101C80.8326 210.591 79.4885 209.507 78.6528 208.061C77.817 206.614 77.5494 204.908 77.902 203.275C78.2546 201.642 79.2022 200.199 80.5604 199.226C81.4448 198.606 82.1667 197.781 82.6651 196.823C83.1635 195.864 83.4237 194.8 83.4237 193.719C83.4237 192.639 83.1635 191.574 82.6651 190.616C82.1667 189.657 81.4448 188.833 80.5604 188.212C79.2041 187.239 78.2582 185.796 77.9065 184.164C77.5547 182.532 77.8222 180.828 78.6571 179.382C79.4919 177.937 80.8344 176.853 82.4236 176.342C84.0129 175.831 85.7353 175.929 87.2562 176.617C88.2359 177.074 89.3111 177.287 90.3909 177.24C91.4706 177.193 92.5231 176.886 93.4592 176.346C94.3953 175.806 95.1875 175.048 95.7687 174.137C96.35 173.226 96.7032 172.188 96.7984 171.112M112.125 193.721C112.125 198.484 108.263 202.346 103.5 202.346C98.7365 202.346 94.875 198.484 94.875 193.721C94.875 188.957 98.7365 185.096 103.5 185.096C108.263 185.096 112.125 188.957 112.125 193.721Z"
          stroke="primary"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <text fill="muted" xmlSpace="preserve" fontSize="16" fontWeight="500" letterSpacing="0em">
          <tspan x="32.1328" y="83.3182">
            {t('howSolutionWorksIllustration:configureProject')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="12" letterSpacing="0em" textAnchor="middle">
          <tspan x="103.027" y="275.864">
            {t('howSolutionWorksIllustration:console', {productName})}
          </tspan>
        </text>
        <path
          d="M963.824 161.608H948.432C944.35 161.608 940.435 159.986 937.548 157.1C934.662 154.213 933.04 150.298 933.04 146.216V115.432C933.04 111.35 934.662 107.435 937.548 104.548C940.435 101.662 944.35 100.04 948.432 100.04H1071.57C1075.65 100.04 1079.57 101.662 1082.45 104.548C1085.34 107.435 1086.96 111.35 1086.96 115.432V146.216C1086.96 150.298 1085.34 154.213 1082.45 157.1C1079.57 159.986 1075.65 161.608 1071.57 161.608H1056.18M963.824 192.392H948.432C944.35 192.392 940.435 194.014 937.548 196.9C934.662 199.787 933.04 203.702 933.04 207.784V238.568C933.04 242.65 934.662 246.565 937.548 249.452C940.435 252.338 944.35 253.96 948.432 253.96H1071.57C1075.65 253.96 1079.57 252.338 1082.45 249.452C1085.34 246.565 1086.96 242.65 1086.96 238.568V207.784C1086.96 203.702 1085.34 199.787 1082.45 196.9C1079.57 194.014 1075.65 192.392 1071.57 192.392H1056.18M963.824 130.824H963.901M963.824 223.176H963.901"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M1017.7 130.824L986.912 177H1033.09L1002.3 223.176"
          stroke="primary"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <text fill="muted" xmlSpace="preserve" fontSize="16" fontWeight="500" letterSpacing="0em">
          <tspan x="998.05" y="78.3182">
            {t('howSolutionWorksIllustration:run')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="12" letterSpacing="0em" textAnchor="middle">
          <tspan x="660" y="275.864">
            {t('howSolutionWorksIllustration:projectEnvConfigs')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="12" letterSpacing="0em" textAnchor="middle">
          <tspan x="1010" y="275.864">
            {t('howSolutionWorksIllustration:runtimeHosted', {productName})}
          </tspan>
        </text>
        <path
          d="M577.707 179.707C578.098 179.317 578.098 178.683 577.707 178.293L571.343 171.929C570.953 171.538 570.319 171.538 569.929 171.929C569.538 172.319 569.538 172.953 569.929 173.343L575.586 179L569.929 184.657C569.538 185.047 569.538 185.681 569.929 186.071C570.319 186.462 570.953 186.462 571.343 186.071L577.707 179.707ZM400 179V180H577V179V178H400V179Z"
          fill="currentColor"
        />
        <path
          d="M919.707 179.707C920.098 179.317 920.098 178.683 919.707 178.293L913.343 171.929C912.953 171.538 912.319 171.538 911.929 171.929C911.538 172.319 911.538 172.953 911.929 173.343L917.586 179L911.929 184.657C911.538 185.047 911.538 185.681 911.929 186.071C912.319 186.462 912.953 186.462 913.343 186.071L919.707 179.707ZM742 179V180H919V179V178H742V179Z"
          fill="currentColor"
        />
        <path
          d="M679.479 125.979H629.479C626.164 125.979 622.985 127.285 620.641 129.61C618.296 131.936 616.979 135.09 616.979 138.379V237.579C616.979 240.867 618.296 244.021 620.641 246.347C622.985 248.672 626.164 249.979 629.479 249.979H704.479C707.795 249.979 710.974 248.672 713.318 246.347C715.663 244.021 716.979 240.867 716.979 237.579V163.179M679.479 125.979C681.458 125.975 683.417 126.36 685.245 127.112C687.073 127.863 688.733 128.965 690.129 130.356L712.554 152.601C713.96 153.987 715.074 155.635 715.834 157.451C716.593 159.266 716.983 161.213 716.979 163.179M679.479 125.979V156.979C679.479 158.623 680.138 160.2 681.31 161.363C682.482 162.525 684.072 163.179 685.729 163.179L716.979 163.179M641.979 187.979H691.979M654.479 181.779V194.179M641.979 218.979H691.979M679.479 212.779V225.179"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M659.884 106.302C661.993 106.299 664.083 106.712 666.032 107.52C667.981 108.327 669.751 109.512 671.24 111.006L681.397 121.163H678.568L669.824 112.419C668.521 111.111 666.972 110.074 665.267 109.367C663.869 108.788 662.389 108.44 660.884 108.335V121.163H658.884V108.301H609.89C606.84 108.301 603.915 109.513 601.759 111.669C599.602 113.825 598.391 116.75 598.391 119.8V219.789C598.391 222.839 599.603 225.764 601.759 227.92C603.646 229.806 606.121 230.969 608.753 231.23V233.237C605.589 232.97 602.607 231.596 600.345 229.334C597.813 226.803 596.391 223.369 596.391 219.789V119.8C596.391 116.22 597.813 112.786 600.345 110.255C602.876 107.723 606.31 106.301 609.89 106.301H659.884V106.302Z"
          fill="currentColor"
        />
        <text fill="currentColor" xmlSpace="preserve" fontSize="14" letterSpacing="0em">
          <tspan x="439.187" y="163.591">
            {t('howSolutionWorksIllustration:saveExport')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="14" letterSpacing="0em">
          <tspan x="806.234" y="166.591">
            {t('howSolutionWorksIllustration:import')}
          </tspan>
        </text>
        <text
          fill="currentColor"
          xmlSpace="preserve"
          fontSize="18"
          fontWeight="600"
          letterSpacing="0em"
          textAnchor="middle"
        >
          <tspan x="1019.384" y="17.5455">
            {t('howSolutionWorksIllustration:runInProduction', {productName})}
          </tspan>
        </text>
        <text
          fill="currentColor"
          xmlSpace="preserve"
          fontSize="14"
          fontStyle="italic"
          fontWeight="500"
          letterSpacing="0em"
          textAnchor="middle"
        >
          <tspan x="1019.384" y="34.5455">
            {t('howSolutionWorksIllustration:runtimeComponentsOnly')}
          </tspan>
        </text>
        <text
          fill="currentColor"
          xmlSpace="preserve"
          fontSize="18"
          fontWeight="600"
          letterSpacing="0em"
          textAnchor="middle"
        >
          <tspan x="195.2969" y="21.5455">
            {t('howSolutionWorksIllustration:designConfigure', {productName})}
          </tspan>
        </text>
        <text
          fill="currentColor"
          xmlSpace="preserve"
          fontSize="14"
          fontStyle="italic"
          fontWeight="500"
          letterSpacing="0em"
        >
          <tspan x="106.377" y="38.5455">
            {t('howSolutionWorksIllustration:designComponents')}
          </tspan>
        </text>
        <rect x="859" y="306" width="301" height="44" rx="10" fill="black" />
        <text fill="white" xmlSpace="preserve" fontSize="12" letterSpacing="0em">
          <tspan x="877" y="332.364">
            {t('howSolutionWorksIllustration:commandProduction')}
          </tspan>
        </text>
        <path
          d="M1201.67 186.667V197.333M1180.33 197.333H1233.67M1191 186.667V197.333M1185.67 186.667H1228.33C1231.28 186.667 1233.67 189.054 1233.67 192V224C1233.67 226.946 1231.28 229.333 1228.33 229.333H1185.67C1182.72 229.333 1180.33 226.946 1180.33 224V192C1180.33 189.054 1182.72 186.667 1185.67 186.667Z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M1200.67 122.667V133.333M1179.33 133.333H1232.67M1190 122.667V133.333M1184.67 122.667H1227.33C1230.28 122.667 1232.67 125.054 1232.67 128V160C1232.67 162.946 1230.28 165.333 1227.33 165.333H1184.67C1181.72 165.333 1179.33 162.946 1179.33 160V128C1179.33 125.054 1181.72 122.667 1184.67 122.667Z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <line x1="1093" y1="175" x2="1135" y2="175" stroke="currentColor" strokeWidth="2" />
        <line x1="1136" y1="144" x2="1136" y2="208" stroke="currentColor" strokeWidth="2" />
        <path
          d="M1168.71 145.707C1169.1 145.317 1169.1 144.683 1168.71 144.293L1162.34 137.929C1161.95 137.538 1161.32 137.538 1160.93 137.929C1160.54 138.319 1160.54 138.953 1160.93 139.343L1166.59 145L1160.93 150.657C1160.54 151.047 1160.54 151.681 1160.93 152.071C1161.32 152.462 1161.95 152.462 1162.34 152.071L1168.71 145.707ZM1136 145V146H1168V145V144H1136V145Z"
          fill="currentColor"
        />
        <path
          d="M1168.71 207.707C1169.1 207.317 1169.1 206.683 1168.71 206.293L1162.34 199.929C1161.95 199.538 1161.32 199.538 1160.93 199.929C1160.54 200.319 1160.54 200.953 1160.93 201.343L1166.59 207L1160.93 212.657C1160.54 213.047 1160.54 213.681 1160.93 214.071C1161.32 214.462 1161.95 214.462 1162.34 214.071L1168.71 207.707ZM1136 207V208H1168V207V206H1136V207Z"
          fill="currentColor"
        />
        <text fill="currentColor" xmlSpace="preserve" fontSize="11" letterSpacing="0em">
          <tspan x="1180" y="249.5">
            {t('howSolutionWorksIllustration:adminApp')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="11" letterSpacing="0em">
          <tspan x="1178" y="107.5">
            {t('howSolutionWorksIllustration:loginApp')}
          </tspan>
        </text>
        <rect x="119" y="306" width="161" height="44" rx="10" fill="black" />
        <text fill="white" xmlSpace="preserve" fontSize="12" letterSpacing="0em">
          <tspan x="174" y="332.364">
            {t('howSolutionWorksIllustration:commandStart')}
          </tspan>
        </text>
      </g>
    </svg>
  );
};

HowSolutionWorksIllustration.displayName = 'HowSolutionWorksIllustration';

export default HowSolutionWorksIllustration;
