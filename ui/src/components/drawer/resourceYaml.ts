// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

type ResourceYamlInput = {
  apiVersion: string;
  kind: string;
  name: string;
  namespace: string;
  generation?: number;
};

export type SinkFamilyKind =
  | "KollectSnapshotSink"
  | "KollectDatabaseSink"
  | "KollectEventSink";

type SinkYamlInput = {
  name: string;
  namespace: string;
  status?: string;
  message?: string;
  family?: SinkFamilyKind;
};

export function inferSinkFamily(sinkName: string): SinkFamilyKind {
  const normalized = sinkName.toLowerCase();

  if (
    normalized.includes("postgres") ||
    normalized.includes("database") ||
    normalized.endsWith("-pg") ||
    normalized.startsWith("pg-")
  ) {
    return "KollectDatabaseSink";
  }

  if (
    normalized.includes("kafka") ||
    normalized.includes("nats") ||
    normalized.includes("prometheus") ||
    normalized.includes("event")
  ) {
    return "KollectEventSink";
  }

  return "KollectSnapshotSink";
}

function snapshotTypeForName(name: string): string {
  const normalized = name.toLowerCase();
  if (normalized.includes("s3")) {
    return "s3";
  }
  if (normalized.includes("gitlab")) {
    return "gitlab";
  }
  return "git";
}

function buildSinkSpecSnippet(family: SinkFamilyKind, name: string): string[] {
  switch (family) {
    case "KollectDatabaseSink":
      return ["spec:", "  type: postgres", "  connectionTest: true"];
    case "KollectEventSink":
      return ["spec:", "  type: kafka", "  connectionTest: true"];
    default: {
      const snapType = snapshotTypeForName(name);
      const lines = ["spec:", `  type: ${snapType}`, "  connectionTest: true"];
      if (snapType === "git" || snapType === "gitlab") {
        lines.push("  git:", "    branch: main", "    pushPolicy: Commit");
      }
      return lines;
    }
  }
}

export function buildResourceYamlSnippet({
  apiVersion,
  kind,
  name,
  namespace,
  generation,
}: ResourceYamlInput): string {
  const lines = [
    `apiVersion: ${apiVersion}`,
    `kind: ${kind}`,
    "metadata:",
    `  name: ${name}`,
    `  namespace: ${namespace}`,
  ];

  if (generation !== undefined) {
    lines.push(`  generation: ${generation}`);
  }

  lines.push("spec: {}");

  return lines.join("\n");
}

type InventoryItemYamlInput = {
  group?: string;
  version: string;
  kind: string;
  name: string;
  namespace: string;
  uid: string;
  targetNamespace: string;
  targetName: string;
};

export function buildInventoryItemYamlSnippet(item: InventoryItemYamlInput): string {
  const apiVersion = item.group ? `${item.group}/${item.version}` : item.version;
  const lines = [
    `apiVersion: ${apiVersion}`,
    `kind: ${item.kind}`,
    "metadata:",
    `  name: ${item.name}`,
    `  namespace: ${item.namespace}`,
    `  uid: ${item.uid}`,
    "collectedBy:",
    `  target: ${item.targetNamespace}/${item.targetName}`,
  ];

  return lines.join("\n");
}

export function buildSinkYamlSnippet({
  name,
  namespace,
  status,
  message,
  family,
}: SinkYamlInput): string {
  const sinkFamily = family ?? inferSinkFamily(name);
  const lines = [
    "apiVersion: kollect.dev/v1alpha1",
    `kind: ${sinkFamily}`,
    "metadata:",
    `  name: ${name}`,
    `  namespace: ${namespace}`,
    ...buildSinkSpecSnippet(sinkFamily, name),
  ];

  if (status || message) {
    lines.push("status:");
    if (status) {
      lines.push(`  exportStatus: ${status}`);
    }
    if (message) {
      lines.push(`  message: ${JSON.stringify(message)}`);
    }
  }

  return lines.join("\n");
}
