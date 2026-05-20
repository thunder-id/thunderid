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

const HowProductRunInHostedIllustration: React.FC = () => {
  const {config} = useConfig();
  const {t} = useTranslation();

  const {brand} = config;
  const {product_name: productName} = brand || {};

  return (
    <svg width="100%" height="100%" viewBox="0 0 548 279" fill="none" xmlns="http://www.w3.org/2000/svg">
      <g clipPath="url(#clip0_39_258)">
        <path
          d="M268.824 161.608H253.432C249.35 161.608 245.435 159.986 242.548 157.1C239.662 154.213 238.04 150.298 238.04 146.216V115.432C238.04 111.35 239.662 107.435 242.548 104.548C245.435 101.662 249.35 100.04 253.432 100.04H376.568C380.65 100.04 384.565 101.662 387.452 104.548C390.338 107.435 391.96 111.35 391.96 115.432V146.216C391.96 150.298 390.338 154.213 387.452 157.1C384.565 159.986 380.65 161.608 376.568 161.608H361.176M268.824 192.392H253.432C249.35 192.392 245.435 194.014 242.548 196.9C239.662 199.787 238.04 203.702 238.04 207.784V238.568C238.04 242.65 239.662 246.565 242.548 249.452C245.435 252.338 249.35 253.96 253.432 253.96H376.568C380.65 253.96 384.565 252.338 387.452 249.452C390.338 246.565 391.96 242.65 391.96 238.568V207.784C391.96 203.702 390.338 199.787 387.452 196.9C384.565 194.014 380.65 192.392 376.568 192.392H361.176M268.824 130.824H268.901M268.824 223.176H268.901"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M322.696 130.824L291.912 177H338.088L307.304 223.176"
          stroke="primary"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <text fill="muted" xmlSpace="preserve" fontSize="16" fontWeight="500" letterSpacing="0em">
          <tspan x="300.055" y="78.3182">
            {t('howSolutionWorksIllustration:run')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="12" letterSpacing="0em">
          <tspan x="0" y="275.864">
            {t('howSolutionWorksIllustration:projectEnvConfigs')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="12" letterSpacing="0em" textAnchor="middle">
          <tspan x="315" y="275.864">
            {t('howSolutionWorksIllustration:runtimeHosted', {productName})}
          </tspan>
        </text>
        <path
          d="M224.707 179.707C225.098 179.317 225.098 178.683 224.707 178.293L218.343 171.929C217.953 171.538 217.319 171.538 216.929 171.929C216.538 172.319 216.538 172.953 216.929 173.343L222.586 179L216.929 184.657C216.538 185.047 216.538 185.681 216.929 186.071C217.319 186.462 217.953 186.462 218.343 186.071L224.707 179.707ZM141 179L141 180L224 180L224 179L224 178L141 178L141 179Z"
          fill="currentColor"
        />
        <path
          d="M83.4795 125.979H33.4795C30.1643 125.979 26.9849 127.285 24.6407 129.61C22.2965 131.936 20.9795 135.09 20.9795 138.379V237.579C20.9795 240.867 22.2965 244.021 24.6407 246.347C26.9849 248.672 30.1643 249.979 33.4795 249.979H108.479C111.795 249.979 114.974 248.672 117.318 246.347C119.663 244.021 120.979 240.867 120.979 237.579V163.179M83.4795 125.979C85.4579 125.975 87.4175 126.36 89.2453 127.112C91.0731 127.863 92.733 128.965 94.1295 130.356L116.554 152.601C117.96 153.987 119.074 155.635 119.834 157.451C120.593 159.266 120.983 161.213 120.979 163.179M83.4795 125.979V156.979C83.4795 158.623 84.138 160.2 85.3101 161.363C86.4822 162.525 88.0719 163.179 89.7295 163.179L120.979 163.179M45.9795 187.979H95.9795M58.4795 181.779V194.179M45.9795 218.979H95.9795M83.4795 212.779V225.179"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M63.8838 106.302C65.9934 106.299 68.0833 106.712 70.0322 107.52C71.981 108.327 73.7512 109.512 75.2402 111.006L85.3975 121.163H82.5684L73.8242 112.419C72.5211 111.111 70.9722 110.074 69.2666 109.367C67.869 108.788 66.3885 108.44 64.8838 108.335V121.163H62.8838V108.301H13.8896C10.8401 108.301 7.91517 109.513 5.75879 111.669C3.60242 113.825 2.39064 116.75 2.39062 119.8V219.789C2.39072 222.839 3.60252 225.764 5.75879 227.92C7.64551 229.806 10.1209 230.969 12.7529 231.23V233.237C9.58948 232.97 6.60678 231.596 4.34473 229.334C1.81338 226.803 0.390716 223.369 0.390625 219.789V119.8C0.390636 116.22 1.81328 112.786 4.34473 110.255C6.87618 107.723 10.3096 106.301 13.8896 106.301H63.8838V106.302Z"
          fill="currentColor"
        />
        <text fill="currentColor" xmlSpace="preserve" fontSize="14" letterSpacing="0em">
          <tspan x="160.234" y="166.591">
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
          <tspan x="306.967" y="17.5455">
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
        >
          <tspan x="168.384" y="34.5455">
            {t('howSolutionWorksIllustration:runtimeComponentsOnly')}
          </tspan>
        </text>
        <path
          d="M512.667 186.667V197.333M491.333 197.333H544.667M502 186.667V197.333M496.667 186.667H539.333C542.279 186.667 544.667 189.055 544.667 192V224C544.667 226.946 542.279 229.333 539.333 229.333H496.667C493.721 229.333 491.333 226.946 491.333 224V192C491.333 189.055 493.721 186.667 496.667 186.667Z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M511.667 122.667V133.333M490.333 133.333H543.667M501 122.667V133.333M495.667 122.667H538.333C541.279 122.667 543.667 125.055 543.667 128V160C543.667 162.946 541.279 165.333 538.333 165.333H495.667C492.721 165.333 490.333 162.946 490.333 160V128C490.333 125.055 492.721 122.667 495.667 122.667Z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <line x1="398" y1="175" x2="440" y2="175" stroke="currentColor" strokeWidth="2" />
        <line x1="441" y1="144" x2="441" y2="208" stroke="currentColor" strokeWidth="2" />
        <path
          d="M473.707 145.707C474.098 145.317 474.098 144.683 473.707 144.293L467.343 137.929C466.953 137.538 466.319 137.538 465.929 137.929C465.538 138.319 465.538 138.953 465.929 139.343L471.586 145L465.929 150.657C465.538 151.047 465.538 151.681 465.929 152.071C466.319 152.462 466.953 152.462 467.343 152.071L473.707 145.707ZM441 145V146H473V145V144H441V145Z"
          fill="currentColor"
        />
        <path
          d="M473.707 207.707C474.098 207.317 474.098 206.683 473.707 206.293L467.343 199.929C466.953 199.538 466.319 199.538 465.929 199.929C465.538 200.319 465.538 200.953 465.929 201.343L471.586 207L465.929 212.657C465.538 213.047 465.538 213.681 465.929 214.071C466.319 214.462 466.953 214.462 467.343 214.071L473.707 207.707ZM441 207V208H473V207V206H441V207Z"
          fill="currentColor"
        />
        <text fill="currentColor" xmlSpace="preserve" fontSize="11" letterSpacing="0em">
          <tspan x="491" y="249.5">
            {t('howSolutionWorksIllustration:adminApp')}
          </tspan>
        </text>
        <text fill="currentColor" xmlSpace="preserve" fontSize="11" letterSpacing="0em">
          <tspan x="489" y="107.5">
            {t('howSolutionWorksIllustration:loginApp')}
          </tspan>
        </text>
      </g>
    </svg>
  );
};

HowProductRunInHostedIllustration.displayName = 'HowProductRunInHostedIllustration';

export default HowProductRunInHostedIllustration;
