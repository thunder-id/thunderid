// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** A user who did something: their id, and the name their token gave them when it carried one. */
export interface Actor {
  id: string;
  name?: string;
}

/** A captured configuration version, as the control plane lists one. */
export interface Version {
  /** What names the version everywhere: its hash on a control plane. */
  version: string;
  /** The version's content hash, when `version` is not it, such as a numbered version kept elsewhere. */
  hash?: string;
  /** The version's number, from 1, which stays the same as older versions are removed. */
  number?: number;
  name?: string;
  note?: string;
  createdAt: string;
  /** Who the version was captured for: who captured it, or whose change it followed. */
  createdBy?: Actor;
  /** Present on a single version read: where it refers to a variable or a secret. */
  references?: ValueReference[];
  /** Present on a single version read: the YAML it holds. */
  resources?: string;
  /**
   * Set on the newest version when it is the current one: the configuration as it stands, which the
   * control plane keeps up to date in place until a gateway runs it, and which stays current until
   * the next change.
   */
  current?: boolean;
}

/** One place a configuration refers to a value a gateway holds. */
export interface ValueReference {
  name: string;
  kind: 'variable' | 'secret';
  resourceType: string;
  resourceId?: string;
  resourceName?: string;
  field?: string;
  /** Whether the reference stands in a list, so the value is a JSON list of strings. */
  list?: boolean;
}

/** Which version a gateway runs. */
export interface AppliedVersion {
  gatewayId: string;
  appliedVersion?: string;
  previousVersion?: string;
  appliedAt?: string;
  /** Who applied the version, or rolled the target back to it. */
  appliedBy?: Actor;
}

/** What a gateway runs: every resource of the version it last applied, as its own read returns it. */
export interface AppliedConfiguration {
  gatewayId: string;
  version?: string;
  appliedAt?: string;
  resources: AppliedResource[];
  skipped?: SkippedResource[];
}

export interface AppliedResource {
  resourceType: string;
  id: string;
  resource: unknown;
  /** Entries a resource lists apart from its own fields, such as a group's members. */
  parts?: Record<string, unknown>;
}

export interface SkippedResource {
  resourceType: string;
  id?: string;
  reason?: string;
}

export type ChangeKind = 'added' | 'updated' | 'deleted' | 'unchanged';

/** One resource an apply would write, delete or leave alone. */
export interface Change {
  key: string;
  resourceType: string;
  id?: string;
  name?: string;
  change: ChangeKind;
  /** The target refused this resource on its last deploy, so it is offered again. */
  failed?: boolean;
  /** On a deploy's answer, the change was sent. */
  selected?: boolean;
  /** Offered as updated because a value it refers to was set on the target again, though no document changed. */
  valueChanged?: boolean;
  /** The keys of the resources this one names by id in the version, which it needs on the target. */
  needs?: string[];
  /** On a deploy's answer, sent because a selected resource needs it; `requiredBy` is that resource's key. */
  required?: boolean;
  requiredBy?: string;
  /**
   * Where the changes come from holds this change back, so it does not run it: a promotion carries only
   * what its source runs, so the change cannot be applied.
   */
  sourceExcluded?: boolean;
  lines?: LineOp[];
}

export interface LineOp {
  kind: ' ' | '+' | '-';
  text: string;
}

export interface DiffSummary {
  added: number;
  updated: number;
  deleted: number;
  unchanged: number;
}

export interface Diff {
  /** The hash of the target's master, what it holds now. */
  fromMaster?: string;
  toVersion: string;
  summary: DiffSummary;
  changes: Change[];
}

/** The values a version refers to that a gateway does not hold. */
export interface MissingValues {
  variables?: string[];
  secrets?: string[];
  references?: ValueReference[];
}

/** The values a version refers to that were set on a gateway after the version it runs was deployed. */
export interface ChangedValues {
  variables?: string[];
  secrets?: string[];
  references?: ValueReference[];
}

export interface ApplyRequest {
  /** A version's hash, or `latest`. */
  version?: string;
  dryRun?: boolean;
  /** The keys of the changes to apply; absent, every change. Nothing about it is kept. */
  selection?: string[];
  /** Widens the selection to what it needs that the target lacks. On unless set to false. */
  withDependencies?: boolean;
  /** Lets the apply remove more resources than the deployment's threshold allows unconfirmed. */
  confirmRemovals?: boolean;
  /** A label for where the version came from, recorded on each resource the apply writes. */
  origin?: string;
}

export interface ApplyResult {
  gatewayId: string;
  dryRun: boolean;
  diff: Diff;
  import?: ImportResult;
  recorded?: boolean;
  /** The hash of the master the target holds after the deploy. */
  master?: string;
  missing?: MissingValues;
  changed?: ChangedValues;
}

export type EntryState = 'deployed' | 'failed' | 'removed';

/** How one resource got onto a target, as its master records it. */
export interface MasterEntry {
  key: string;
  resourceType: string;
  id?: string;
  name?: string;
  /** The hash of the document the target holds; none for a removed resource or one it never accepted. */
  contentHash?: string;
  /** The version the resource was last deployed from. */
  sourceVersion?: string;
  /** The label the deploy gave for where the version came from. */
  origin?: string;
  state: EntryState;
  /** What the target answered when it refused the resource. */
  failure?: {code?: string; message?: string};
  at?: string;
  by?: Actor;
}

export interface MasterSummary {
  deployed: number;
  failed: number;
  removed: number;
}

/** What a target holds, resource by resource: its master. */
export interface Master {
  gatewayId: string;
  /** The master's place in the target's order, from 1; 0 while nothing was deployed. */
  number: number;
  master?: string;
  createdAt?: string;
  createdBy?: Actor;
  /** Every resource the target holds, one document each. A listing leaves it out. */
  resources?: string;
  summary: MasterSummary;
  /** One entry per resource key, sorted by key. A listing leaves it out. */
  entries?: MasterEntry[];
}

/** A resource's entry in one master. */
export interface ResourceHistoryEntry {
  master: string;
  number: number;
  entry: MasterEntry;
}

/** One resource's entry across a target's masters, newest first. */
export interface ResourceHistory {
  gatewayId: string;
  key: string;
  history: ResourceHistoryEntry[];
}

export interface ImportResourceResult {
  resourceType: string;
  resourceId?: string;
  resourceName?: string;
  operation?: string;
  status: string;
  code?: string;
  message?: string;
}

export interface ImportResult {
  summary?: {totalDocuments?: number; imported?: number; deleted?: number; failed?: number};
  results?: ImportResourceResult[];
}

export type ValueKind = 'variable' | 'secret';

/** A value a gateway holds: a variable with its value, or a secret that exists but is never read back. */
export interface StoredValue {
  name: string;
  value?: string;
  exists?: boolean;
  description?: string;
  createdAt?: string;
  updatedAt?: string;
}
