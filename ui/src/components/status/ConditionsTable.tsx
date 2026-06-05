import type { Condition } from "@/api/k8s-status";

type ConditionsTableProps = {
  conditions?: Condition[];
};

export function ConditionsTable({ conditions = [] }: ConditionsTableProps) {
  if (!conditions.length) {
    return (
      <p className="text-sm text-slate-500" role="status">
        No conditions reported.
      </p>
    );
  }

  return (
    <table aria-label="Conditions" className="min-w-full text-left text-sm">
      <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
        <tr>
          <th scope="col" className="px-3 py-2">
            Type
          </th>
          <th scope="col" className="px-3 py-2">
            Status
          </th>
          <th scope="col" className="px-3 py-2">
            Reason
          </th>
          <th scope="col" className="px-3 py-2">
            Message
          </th>
          <th scope="col" className="px-3 py-2">
            Last transition
          </th>
        </tr>
      </thead>
      <tbody>
        {conditions.map((condition) => (
          <tr
            key={`${condition.type}-${condition.status}-${condition.reason ?? ""}`}
            className="border-b border-slate-100 last:border-0"
          >
            <td className="px-3 py-2 font-mono text-xs">{condition.type}</td>
            <td className="px-3 py-2">{condition.status}</td>
            <td className="px-3 py-2">{condition.reason ?? "—"}</td>
            <td className="px-3 py-2 text-slate-600">{condition.message ?? "—"}</td>
            <td className="px-3 py-2 text-xs text-slate-500">
              {condition.lastTransitionTime ?? "—"}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
