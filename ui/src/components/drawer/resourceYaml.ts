type ResourceYamlInput = {
  apiVersion: string;
  kind: string;
  name: string;
  namespace: string;
  generation?: number;
};

type SinkYamlInput = {
  name: string;
  namespace: string;
  status?: string;
  message?: string;
};

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

export function buildSinkYamlSnippet({
  name,
  namespace,
  status,
  message,
}: SinkYamlInput): string {
  const lines = [
    "apiVersion: kollect.dev/v1alpha1",
    "kind: KollectSink",
    "metadata:",
    `  name: ${name}`,
    `  namespace: ${namespace}`,
    "spec: {}",
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
