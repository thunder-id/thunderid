// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Bot,
  Building2,
  Cloud,
  LayoutGrid,
  Layers,
  Lock,
  Mail,
  Code2,
  Palette,
  Phone,
  Shield,
  User,
  UserCog,
  Users,
  Workflow,
  type LucideProps,
} from '@wso2/oxygen-ui-icons-react';
import type {ComponentType, JSX} from 'react';

/** The icons the explorer's lanes and tiles use, by the names the model gives them. */
const ICONS: Readonly<Record<string, ComponentType<LucideProps>>> = {
  building: Building2,
  userCog: UserCog,
  users: Users,
  user: User,
  shield: Shield,
  bot: Bot,
  app: LayoutGrid,
  cloud: Cloud,
  flow: Workflow,
  layers: Layers,
  palette: Palette,
  github: Code2,
  phone: Phone,
  mail: Mail,
  lock: Lock,
};

export interface LaneIconProps {
  name: string;
  size?: number;
}

export default function LaneIcon({name, size = 24}: LaneIconProps): JSX.Element | null {
  const Icon: ComponentType<LucideProps> | undefined = ICONS[name];
  return Icon ? <Icon size={size} strokeWidth={1.75} aria-hidden /> : null;
}
