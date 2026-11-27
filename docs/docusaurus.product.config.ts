// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
  local: {
    consoleUrl: string;
    controlPlaneConsoleUrl: string;
    samples: {
      wayfinderUrl: string;
      wayfinderMailUrl: string;
    };
  };
}

const DocusaurusProductConfig = {
  project: {
    emoji: '⚡',
    name: 'ThunderID',
    description: 'Open Source Auth for Modern Apps and AI Agents',
    source: {
      github: {
        name: 'thunderid',
        fullName: 'thunder-id/thunderid',
        url: 'https://github.com/thunder-id/thunderid',
        discussionsUrl: 'https://github.com/thunder-id/thunderid/discussions',
        issuesUrl: 'https://github.com/thunder-id/thunderid/issues',
        releasesUrl: '/releases',
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
      enabled: true,
    },
    deployment: {
      production: {
        baseUrl: '',
        url: 'https://thunderid.dev',
      },
    },
  },
  local: {
    consoleUrl: 'https://localhost:8090/console',
    controlPlaneConsoleUrl: 'https://localhost:8095/console',
    samples: {
      wayfinderUrl: 'http://localhost:5173',
      wayfinderMailUrl: 'http://localhost:8788',
    },
  },
};

export default DocusaurusProductConfig;
