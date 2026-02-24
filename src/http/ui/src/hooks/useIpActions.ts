import { notifications } from "@mantine/notifications";
import { useCallback } from "react";

export function useAddIp() {
  const addIp = useCallback(
    async (entries: string[], setId: string, setName?: string) => {
      if (!entries.length) return;
      try {
        const res = await fetch("/api/geoip", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            cidr: entries,
            set_id: setId,
            set_name: setName,
          }),
        });
        if (res.ok) {
          notifications.show({
            title: "Success",
            message: entries.length > 1 ? `${entries.length} entries added` : `${entries[0]} added`,
          });
        } else {
          const { message } = (await res.json()) as { message: string };
          notifications.show({ title: "Error", message: `Failed: ${message}` });
        }
      } catch (e) {
        notifications.show({ title: "Error", message: String(e) });
      }
    },
    [],
  );
  return { addIp };
}
