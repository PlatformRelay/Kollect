import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

function watchUrl(): string {
  const baseUrl = import.meta.env.VITE_READ_API_URL ?? "";
  return new URL("/v1alpha1/inventory/watch", baseUrl || window.location.origin).toString();
}

export function useInventoryWatch(enabled = true) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!enabled || typeof window === "undefined" || typeof EventSource === "undefined") {
      return;
    }

    const source = new EventSource(watchUrl());

    source.onmessage = () => {
      void queryClient.invalidateQueries({ queryKey: ["inventory"] });
    };

    return () => {
      source.close();
    };
  }, [enabled, queryClient]);
}
