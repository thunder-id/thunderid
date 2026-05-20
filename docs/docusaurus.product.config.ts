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

export interface DocusaurusProductConfig {
  project: {
    emoji: string;
    name: string;
    description: string;
    source: {
      github: {
        name: string;
        fullName: string;
        url: string;
        discussionsUrl: string;
        issuesUrl: string;
        releasesUrl: string;
        editUrls: {
          blog: string;
          content: string;
        };
        owner: {
          name: string;
        };
      };
    };
  };
  postman: {
    collection: {
      output: string;
    };
  };
  documentation: {
    versioning: {
      enabled: boolean;
    };
    deployment: {
      production: {
        baseUrl: string;
        url: string;
      };
    };
  };
}

const DocusaurusProductConfig = {
  project: {
    emoji: '⚡',
    name: 'ThunderID',
    description:
      'ThunderID is a modern, open-source identity management service designed for teams building secure, customizable authentication experiences across applications, services, and AI agents.',
    source: {
      github: {
        name: 'thunderid',
        fullName: 'thunder-id/thunderid',
        url: 'https://github.com/thunder-id/thunderid',
        discussionsUrl: 'https://github.com/thunder-id/thunderid/discussions',
        issuesUrl: 'https://github.com/thunder-id/thunderid/issues',
        releasesUrl: '/docs/next/releases',
        editUrls: {
          blog: 'https://github.com/thunder-id/thunderid/tree/main/blog/',
          content: 'https://github.com/thunder-id/thunderid/tree/main/docs/',
        },
        owner: {
          name: 'thunderid',
        },
      },
    },
  },
  postman: {
    collection: {
      output: 'thunderid-api-postman-collection.json',
    },
  },
  documentation: {
    versioning: {
      enabled: false,
    },
    deployment: {
      production: {
        baseUrl: '',
        url: 'https://thunderid.dev',
      },
    },
  },
};

export default DocusaurusProductConfig;
