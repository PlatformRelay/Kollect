/**
 * CRD status client — primary path uses Read API proxy endpoints (B3).
 * UI may call Kubernetes API directly for conditions in hybrid mode (OQ-3).
 */

export type Condition = {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
};

export type ResourceStatus = {
  name: string;
  namespace: string;
  generation?: number;
  observedGeneration?: number;
  itemCount?: number;
  lastExportTime?: string;
  conditions?: Condition[];
};

export type StatusListResponse = {
  schemaVersion: string;
  items: ResourceStatus[];
};

const readApiBase = import.meta.env.VITE_READ_API_URL ?? "";
const k8sApiBase = import.meta.env.VITE_K8S_API_URL ?? "/api/k8s";

async function fetchStatus(path: string, namespace?: string): Promise<StatusListResponse> {
  const url = new URL(path, readApiBase || window.location.origin);
  if (namespace) {
    url.searchParams.set("namespace", namespace);
  }

  const response = await fetch(url.toString());
  if (!response.ok) {
    throw new Error(`Status API ${response.status}: ${response.statusText}`);
  }

  return response.json() as Promise<StatusListResponse>;
}

export function fetchInventoryStatus(namespace?: string) {
  return fetchStatus("/v1alpha1/status/inventories", namespace);
}

export function fetchTargetStatus(namespace?: string) {
  return fetchStatus("/v1alpha1/status/targets", namespace);
}

/** Stub for direct Kubernetes API access (hybrid mode — not used in MVP dev mocks). */
export async function fetchTargetStatusFromK8s(namespace?: string): Promise<StatusListResponse> {
  const group = "kollect.dev";
  const version = "v1alpha1";
  const resource = "kollecttargets";
  const url = new URL(
    `${k8sApiBase}/apis/${group}/${version}/namespaces/${namespace ?? "default"}/${resource}`,
  );

  const response = await fetch(url.toString());
  if (!response.ok) {
    throw new Error(`Kubernetes API ${response.status}`);
  }

  const body = (await response.json()) as {
    items: Array<{ metadata: { name: string; namespace: string }; status?: { conditions?: Condition[] } }>;
  };

  return {
    schemaVersion: "kollect.dev/v1alpha1",
    items: body.items.map((item) => ({
      name: item.metadata.name,
      namespace: item.metadata.namespace,
      conditions: item.status?.conditions,
    })),
  };
}
