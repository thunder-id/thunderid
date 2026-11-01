// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import EnterpriseSso from '@/components/EnterpriseSso';
import config from '@/lib/config';
import requireAdmin from '@/lib/requireAdmin';

export default async function EnterpriseSsoPage() {
  const user = await requireAdmin();

  return (
    <>
      <div className="page-heading">
        <p className="eyebrow">Enterprise SSO</p>
        <h1>Sign in with your company’s identity provider</h1>
      </div>
      <EnterpriseSso
        ouId={user.ouId}
        ouHandle={user.ouHandle}
        ouName={user.ouName}
        signInFlowId={config.signInFlowId}
        callbackUrl={`${config.thunderIdUrl}/gate/callback`}
      />
    </>
  );
}
