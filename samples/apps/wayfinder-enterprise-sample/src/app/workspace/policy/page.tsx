// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import PolicyEditor from '@/components/PolicyEditor';
import requireAdmin from '@/lib/requireAdmin';

export default async function PolicyPage() {
  await requireAdmin();

  return (
    <>
      <div className="page-heading">
        <p className="eyebrow">Travel policy</p>
        <h1>Set what your team can book</h1>
      </div>
      <PolicyEditor />
    </>
  );
}
