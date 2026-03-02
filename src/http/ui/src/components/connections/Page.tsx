import { devicesApi } from "@b4.devices";
import {
  useAddDomain,
  useEnrichedLogs,
  useParsedLogs,
} from "@hooks/useDomainActions";
import { useAddIp } from "@hooks/useIpActions";
import { useHotkeys } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { B4Config, B4SetConfig } from "@models/config";
import { generateDomainVariants, generateIpVariants } from "@utils";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useWebSocket } from "../../context/B4WsProvider";
import { AddIpModal } from "./AddIpModal";
import { AddSniModal } from "./AddSniModal";
import { TableSort } from "./Table";

const MAX_RAW_ROWS = 1000;
const MAX_DISPLAY_ROWS = 500;

export function ConnectionsPage() {
  const {
    domains,
    pauseDomains,
    showAll,
    setShowAll,
    setPauseDomains,
    clearDomains,
    resetDomainsBadge,
  } = useWebSocket();

  const { addDomain } = useAddDomain();
  const { addIp } = useAddIp();
  const [domainModal, setDomainModal] = useState<{
    domain: string;
    variants: string[];
  } | null>(null);
  const [ipModal, setIpModal] = useState<{
    ip: string;
    variants: string[];
  } | null>(null);

  const showSuccess = useCallback((message: string) => {
    notifications.show({ title: "Success", message });
  }, []);

  const [availableSets, setAvailableSets] = useState<B4SetConfig[]>([]);
  const [ipInfoToken, setIpInfoToken] = useState<string>("");
  const [devicesEnabled, setDevicesEnabled] = useState<boolean>(false);
  const [deviceMap, setDeviceMap] = useState<Record<string, string>>({});

  // Limit displayed rows for performance
  const recentDomains = useMemo(() => domains.slice(-MAX_RAW_ROWS), [domains]);
  const parsedLogs = useParsedLogs(recentDomains, showAll);
  const displayedLogs = useMemo(
    () => parsedLogs.slice(-MAX_DISPLAY_ROWS),
    [parsedLogs],
  );
  const enrichedLogs = useEnrichedLogs(displayedLogs, deviceMap);

  useEffect(() => {
    if (!devicesEnabled) {
      setDeviceMap({});
      return;
    }
    devicesApi
      .list()
      .then((data) => {
        const map: Record<string, string> = {};
        for (const d of data.devices || []) {
          const normalized = d.mac.toUpperCase().replace(/-/g, ":");
          map[normalized] = d.alias || d.vendor || "";
        }
        setDeviceMap(map);
      })
      .catch(() => {});
  }, [devicesEnabled]);

  const fetchSets = useCallback(async (signal?: AbortSignal) => {
    try {
      const response = await fetch("/api/config", { signal });
      if (response.ok) {
        const data = (await response.json()) as B4Config;
        if (data.sets && Array.isArray(data.sets)) {
          setAvailableSets(data.sets);
        }
        if (data.system?.api?.ipinfo_token) {
          setIpInfoToken(data.system.api.ipinfo_token);
        }
        setDevicesEnabled(
          data.queue?.devices?.enabled ||
            data.queue?.devices?.vendor_lookup ||
            false,
        );
      }
    } catch (error) {
      if ((error as Error).name !== "AbortError") {
        console.error("Failed to fetch sets:", error);
      }
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void fetchSets(controller.signal);
    return () => {
      controller.abort();
    };
  }, [fetchSets]);

  useHotkeys([
    [
      "mod+x",
      () => {
        clearDomains();
        resetDomainsBadge();
        showSuccess("Cleared all domains");
      },
    ],
    [
      "Delete",
      () => {
        clearDomains();
        resetDomainsBadge();
        showSuccess("Cleared all domains");
      },
    ],
    [
      "p",
      () => {
        setPauseDomains(!pauseDomains);
        showSuccess(`Domains ${pauseDomains ? "resumed" : "paused"}`);
      },
    ],
  ]);

  return (
    <>
      <TableSort
        logs={enrichedLogs}
        onDomainClick={(domain) => {
          const v = generateDomainVariants(domain);
          setDomainModal({ domain, variants: v.length ? v : [domain] });
        }}
        onIpClick={(ip) => {
          const v = generateIpVariants(ip);
          setIpModal({
            ip: ip.split(":")[0].replaceAll(/[[\]]/g, ""),
            variants: v.length ? v : [ip],
          });
        }}
      />

      <AddSniModal
        opened={!!domainModal}
        onClose={() => setDomainModal(null)}
        domain={domainModal?.domain ?? ""}
        variants={domainModal?.variants ?? []}
        sets={availableSets}
        onAdd={async (domain, setId, setName) => {
          await addDomain(domain, setId, setName);
          await fetchSets();
        }}
      />

      <AddIpModal
        opened={!!ipModal}
        onClose={() => setIpModal(null)}
        ip={ipModal?.ip ?? ""}
        variants={ipModal?.variants ?? []}
        sets={availableSets}
        onAdd={async (entries, setId, setName) => {
          await addIp(entries, setId, setName);
          await fetchSets();
        }}
        onAddHostname={(hostname) => {
          const v = generateDomainVariants(hostname);
          setIpModal(null);
          setDomainModal({
            domain: hostname,
            variants: v.length ? v : [hostname],
          });
        }}
      />
    </>
  );
}
