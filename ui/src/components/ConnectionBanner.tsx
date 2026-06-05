import { useConnectionStore } from "@/store/connection";

export function ConnectionBanner() {
  const mockApiEnabled = useConnectionStore((s) => s.mockApiEnabled);
  const readApiBaseUrl = useConnectionStore((s) => s.readApiBaseUrl);
  const showDebugBanner = useConnectionStore((s) => s.showDebugBanner);

  if (!showDebugBanner) {
    return null;
  }

  return (
    <div
      className="border-b border-amber-200 bg-amber-50 px-4 py-1.5 text-center text-xs text-amber-900"
      role="status"
      aria-live="polite"
    >
      {mockApiEnabled ? (
        <span>
          <strong>Mock data</strong> — MSW intercepts <code>/v1alpha1/*</code>
        </span>
      ) : (
        <span>
          <strong>Live Read API</strong> — proxy target{" "}
          <code className="font-mono">{readApiBaseUrl}</code>
        </span>
      )}
    </div>
  );
}
