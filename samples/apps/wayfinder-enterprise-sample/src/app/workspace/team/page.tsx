// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import TeamManagement from '@/components/TeamManagement';
import config from '@/lib/config';
import requireAdmin from '@/lib/requireAdmin';

export default async function TeamPage() {
  const user = await requireAdmin();

  return (
    <>
      <div className="page-heading">
        <p className="eyebrow">Team and roles</p>
        <h1>Manage who can use Wayfinder</h1>
      </div>
      <TeamManagement ouId={user.ouId} adminRoleId={config.adminRoleId} memberRoleId={config.memberRoleId} />
    </>
  );
}
