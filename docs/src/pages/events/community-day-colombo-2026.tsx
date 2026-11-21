// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Bot, PlugZap} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import CommunityDayPage, {type CommunityDayEvent} from '@site/src/components/CommunityDay';

const LFDT = {
  name: 'LF Decentralized Trust',
  logo: 'lfdt-full',
  logoWidth: 200,
  url: 'https://www.lfdecentralizedtrust.org/',
};

const EVENT: CommunityDayEvent = {
  title: '{{ProductName}} Agentic AI Community Day',
  edition: 'Colombo 2026',
  shortName: 'Community Day Colombo 2026',
  summary: 'Build, secure, and run AI agents and MCP servers with open source. One evening of talks and live demos.',
  description:
    'Join {{ProductName}} Agentic AI Community Day in Colombo on December 10, 2026. Free talks and live demos on building AI agents and MCP servers with open source.',
  startDate: '2026-12-10T16:00:00+05:30',
  endDate: '2026-12-10T20:00:00+05:30',
  dateLabel: 'December 10, 2026',
  dateNote: 'Thursday',
  timeLabel: '4:00 PM to 8:00 PM',
  timeNote: 'Sri Lanka time',
  venue: {
    name: 'Choreo Labs',
    address: '105, Bauddhaloka Mawatha',
    locality: 'Colombo 4',
    country: 'LK',
    note: 'In person only',
  },
  cost: 'Free',
  registrationUrl: 'https://luma.com/2lvipfap',
  heroPartner: LFDT,
  audience: [
    {
      icon: Bot,
      title: "You're building AI agents.",
      description: 'Learn how to give your agents their own identity and control what they can access.',
    },
    {
      icon: PlugZap,
      title: "You're opening your systems to AI agents.",
      description: 'Learn how to let agents in safely through MCP servers and APIs you can trust.',
    },
  ],
  takeaways: [
    'Build AI agents with LangChain and MCP servers with FastMCP',
    'Learn the agentic AI security basics every builder needs',
    'Run {{ProductName}} on your laptop and leave with working code',
    'Meet the maintainers and find your first contribution',
    'Take part in the {{ProductName}} Community Challenge for a chance to win gift vouchers worth up to LKR 25,000',
    'Made for developers building AI agents, teams whose systems AI agents connect to, and students ready to join the industry',
  ],
  // The session abstracts and speaker bios are placeholder copy. Replace them with the final versions.
  sessions: [
    {
      title: 'What It Really Takes to Build and Run AI Agents in Production',
      abstract:
        'Agents are moving from demos to production, and the hard part is everything around them. Ayesha shows what a real production setup looks like: agent identity, agent gateways, and the other pieces you need whether you run agents or open your systems to them. Grounded in how teams run agents today, not in slides.',
      speakers: [
        {
          name: 'Ayesha Dissanayaka',
          github: 'ayshsandu',
          affiliation: 'AAIF Ambassador',
          bio: 'Ayesha Dissanayaka is an AAIF Ambassador who has spoken at MCP Dev Summit events on identity and authorization for AI agents. In this session, Ayesha shares practical patterns for giving agents their own identity and keeping their access tightly scoped.',
        },
      ],
    },
    {
      title: '{{ProductName}}: Open Source Identity for Humans, AI Agents, and Machines',
      abstract:
        'Most open source identity tools were built for human logins, with agents and machines added later. {{ProductName}} was built from the start for all three. See what that changes and how to get it running in minutes.',
      speakers: [
        {
          name: 'Sagara Gunathunga',
          github: 'sagara-gunathunga',
          affiliation: '{{ProductName}} Maintainer',
          bio: 'Sagara Gunathunga is a {{ProductName}} maintainer. In this session, Sagara introduces {{ProductName}} and the thinking behind an identity stack built for humans, AI agents, and machines.',
        },
        {
          name: 'Senthalan Kanagalingam',
          github: 'senthalan',
          affiliation: '{{ProductName}} Maintainer',
          bio: 'Senthalan Kanagalingam is a {{ProductName}} maintainer. In this session, Senthalan shows {{ProductName}} in action, from a fresh install to a working sign-in flow.',
        },
      ],
    },
    {
      title: 'Securing Unified Communications: Deep {{ProductName}} Integration',
      abstract:
        'Building Email 2.0 and integrated chat with {{ProductName}} to enable centralized authentication, role-based access control, mailbox delegation, and group management across platforms.',
      speakers: [
        {
          name: 'Aravinda Kankanamge',
          github: 'Aravinda-HWK',
          affiliation: 'OpenGovMail Lead, Lanka Software Foundation',
        },
      ],
    },
    {
      title: 'One Identity, Many Authorities: OpenNSW on {{ProductName}}',
      abstract:
        'Businesses and government officers use a single identity, while each agency controls what they can do. See how OpenNSW built this on {{ProductName}} and how it can evolve into a federated government identity.',
      speakers: [
        {
          name: 'Thanikan Sivatheepan',
          github: 'sthanikan2000',
          photo: '/assets/images/events/community-day-colombo-2026/speakers/thanikan-sivatheepan.jpg',
          affiliation: 'OpenNSW IAM Lead, Lanka Software Foundation',
        },
      ],
    },
    {
      title: 'Building Secure MCP Servers with FastMCP',
      abstract:
        'MCP servers hand real tools and data to agents. Watch a FastMCP server get built live, then secured with {{ProductName}} so only authorized callers can use its tools.',
      speakers: [
        {
          name: 'Thivaharan Kalyanasundaram',
          github: 'thiva-k',
          affiliation: '{{ProductName}} Maintainer',
          bio: 'Thivaharan Kalyanasundaram is a {{ProductName}} maintainer. In this session, Thivaharan builds a FastMCP server and secures its tools with {{ProductName}}, step by step.',
        },
      ],
    },
    {
      title: 'Building Secure AI Agents with LangChain',
      abstract:
        "Watch a LangChain agent get built live. It signs in, gets its own credentials, and calls protected APIs on a user's behalf, with {{ProductName}} handling identity.",
      speakers: [
        {
          name: 'Nipuni Paaris',
          github: 'NipuniBhagya',
          affiliation: '{{ProductName}} Maintainer',
          bio: "Nipuni Paaris is a {{ProductName}} maintainer. In this session, Nipuni builds a LangChain agent that signs in with {{ProductName}} and calls protected APIs on a user's behalf.",
        },
      ],
    },
  ],
  sessionsIntro: 'Talks and live demos on building and securing AI agents and MCP servers with open source.',
  sessionsNote: 'Doors open at 4:00 PM for refreshments and networking.',
  organizers: [{name: 'LFDT Sri Lanka Meetup Group', url: 'https://www.meetup.com/lfdt-sri-lanka/'}],
  partners: [
    LFDT,
    // {name: 'Agentic AI Foundation (AAIF)'},
    // {name: 'Partner 2 logo'},
  ],
  sponsors: [{name: 'Choreo Labs', logo: 'choreo-labs', logoWidth: 128, url: 'https://choreolabs.com/'}],
};

export default function CommunityDayColombo2026(): JSX.Element {
  return <CommunityDayPage event={EVENT} />;
}
