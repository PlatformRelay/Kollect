import * as Dialog from "@radix-ui/react-dialog";
import { Copy, X } from "lucide-react";
import { useState, type RefObject } from "react";
import type { Condition } from "@/api/k8s-status";
import { ConditionsTable } from "@/components/status/ConditionsTable";
import { isGenerationStale } from "@/components/status/health";

export type DetailDrawerProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocusRef?: RefObject<HTMLElement | null>;
  title: string;
  subtitle?: string;
  conditions?: Condition[];
  generation?: number;
  observedGeneration?: number;
  yamlSnippet: string;
};

export function DetailDrawer({
  open,
  onOpenChange,
  returnFocusRef,
  title,
  subtitle,
  conditions,
  generation,
  observedGeneration,
  yamlSnippet,
}: DetailDrawerProps) {
  const [copied, setCopied] = useState(false);
  const generationStale = isGenerationStale(generation, observedGeneration);

  async function handleCopyYaml() {
    await navigator.clipboard.writeText(yamlSnippet);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-kollect-navy/30" />
        <Dialog.Content
          className="fixed inset-y-0 right-0 z-50 flex w-full max-w-lg flex-col border-l border-slate-200 bg-white shadow-xl outline-none"
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            returnFocusRef?.current?.focus();
          }}
          aria-describedby="drawer-description"
        >
          <div className="flex items-start justify-between border-b border-slate-200 px-4 py-4">
            <div>
              <Dialog.Title className="text-lg font-semibold text-kollect-navy">
                {title}
              </Dialog.Title>
              {subtitle ? (
                <Dialog.Description
                  id="drawer-description"
                  className="mt-1 text-sm text-slate-600"
                >
                  {subtitle}
                </Dialog.Description>
              ) : (
                <Dialog.Description id="drawer-description" className="sr-only">
                  Resource detail
                </Dialog.Description>
              )}
            </div>
            <Dialog.Close
              type="button"
              aria-label="Close"
              className="rounded-md p-1 text-slate-500 hover:bg-slate-100"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </Dialog.Close>
          </div>

          <div className="flex-1 space-y-6 overflow-y-auto px-4 py-4">
            {(generation !== undefined || observedGeneration !== undefined) && (
              <section aria-label="Generation">
                <h2 className="mb-2 text-sm font-semibold text-kollect-navy">Generation</h2>
                <dl className="grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <dt className="text-slate-500">Generation</dt>
                    <dd className="font-mono">{generation ?? "—"}</dd>
                  </div>
                  <div>
                    <dt className="text-slate-500">Observed generation</dt>
                    <dd className="font-mono">{observedGeneration ?? "—"}</dd>
                  </div>
                </dl>
                {generationStale ? (
                  <p className="mt-2 text-xs text-amber-800" role="status">
                    Status has not caught up to the latest spec generation.
                  </p>
                ) : null}
              </section>
            )}

            <section>
              <h2 className="mb-2 text-sm font-semibold text-kollect-navy">Conditions</h2>
              <ConditionsTable conditions={conditions} />
            </section>

            <section>
              <div className="mb-2 flex items-center justify-between">
                <h2 className="text-sm font-semibold text-kollect-navy">YAML snippet</h2>
                <button
                  type="button"
                  onClick={() => void handleCopyYaml()}
                  className="inline-flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                >
                  <Copy className="h-3.5 w-3.5" aria-hidden="true" />
                  {copied ? "Copied" : "Copy YAML"}
                </button>
              </div>
              <pre
                aria-label="Resource YAML snippet"
                className="overflow-x-auto rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-800"
              >
                {yamlSnippet}
              </pre>
              <p className="mt-2 text-xs text-slate-500">
                Read-only preview — apply changes via GitOps or kubectl.
              </p>
            </section>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
