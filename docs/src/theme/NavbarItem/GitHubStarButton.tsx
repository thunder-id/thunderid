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

import BrowserOnly from '@docusaurus/BrowserOnly';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import React from 'react';

interface ProductCustomFields {
  project: {
    source: {
      github: {
        fullName: string;
        url: string;
      };
    };
  };
}

interface Props {
  mobile?: boolean;
}

export default function GitHubStarButton({mobile = false}: Props): React.ReactElement {
  const {siteConfig} = useDocusaurusContext();
  const {project} = siteConfig.customFields?.product as ProductCustomFields;
  const {fullName, url} = project.source.github;

  const button = (
    <BrowserOnly>
      {() => {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const GitHubButton = require('react-github-btn').default;
        return (
          <GitHubButton
            href={url}
            data-color-scheme="no-preference: light; light: light; dark: dark;"
            data-size="large"
            data-icon="octicon-star"
            aria-label={`Star ${fullName} on GitHub`}
          >
            Star
          </GitHubButton>
        );
      }}
    </BrowserOnly>
  );

  if (mobile) {
    return (
      <li className="menu__list-item">
        <div className="navbar__github-star navbar__github-star--mobile">{button}</div>
      </li>
    );
  }

  return <div className="navbar__github-star">{button}</div>;
}
