import { useEffect, useMemo, useState } from "react";

import { IconAdd, IconCheck } from "@b4.icons";
import { setsApi } from "@b4.sets";
import {
  Accordion,
  ActionIcon,
  Badge,
  Box,
  Group,
  Menu,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import { B4SetConfig } from "@models/config";
import { formatNumber } from "@utils";

interface DeviceInfo {
  mac: string;
  ip: string;
  hostname: string;
  vendor: string;
  alias?: string;
}

interface DeviceActivityProps {
  deviceDomains: Record<string, Record<string, number>>;
  sets: B4SetConfig[];
  targetedDomains: Set<string>;
  onRefreshSets: () => void;
}

export const DeviceActivity = ({
  deviceDomains,
  sets,
  targetedDomains,
  onRefreshSets,
}: DeviceActivityProps) => {
  const [devices, setDevices] = useState<DeviceInfo[]>([]);

  useEffect(() => {
    fetch("/api/devices")
      .then((r) => r.json())
      .then((data: { devices?: DeviceInfo[] }) => {
        if (data?.devices) setDevices(data.devices);
      })
      .catch(() => {});
  }, []);

  const isDomainTargeted = (domain: string): boolean => {
    if (targetedDomains.has(domain)) return true;
    const parts = domain.split(".");
    for (let i = 1; i < parts.length; i++) {
      if (targetedDomains.has(parts.slice(i).join("."))) return true;
    }
    return false;
  };

  const deviceMap = useMemo(() => {
    const map: Record<string, DeviceInfo> = {};
    for (const d of devices) {
      map[d.mac] = d;
    }
    return map;
  }, [devices]);

  // Sort devices by total domain count descending
  const sortedDevices = useMemo(() => {
    return Object.entries(deviceDomains)
      .map(([mac, domains]) => ({
        mac,
        domains,
        total: Object.values(domains).reduce((s, c) => s + c, 0),
        domainCount: Object.keys(domains).length,
      }))
      .sort((a, b) => b.total - a.total);
  }, [deviceDomains]);

  const getDeviceName = (mac: string): string => {
    const dev = deviceMap[mac];
    if (dev?.alias) return dev.alias;
    if (dev?.hostname) return dev.hostname;
    if (dev?.vendor && dev.vendor !== "Private")
      return `${dev.vendor} (${mac})`;
    return mac;
  };

  const getDeviceSubtitle = (mac: string): string => {
    const dev = deviceMap[mac];
    if (!dev) return "";
    const parts: string[] = [];
    if (dev.ip) parts.push(dev.ip);
    if (dev.vendor && dev.vendor !== "Private") parts.push(dev.vendor);
    return parts.join(" - ");
  };

  if (sortedDevices.length === 0) return null;

  return (
    <Box>
      <Text>Device Activity</Text>
      <Accordion multiple variant="contained" chevronPosition="left">
        {sortedDevices.map(({ mac, domains, total, domainCount }) => {
          const sortedDomains = Object.entries(domains).sort(
            (a, b) => b[1] - a[1],
          );

          return (
            <Accordion.Item key={mac} value={mac}>
              <Accordion.Control
                icon={
                  <Group>
                    <Badge variant="light">{`${domainCount} domains`}</Badge>
                    <Badge variant="light">{`${formatNumber(total)} conn`}</Badge>
                  </Group>
                }
              >
                <Group justify="space-between" wrap="nowrap">
                  <Group>
                    <Text>{getDeviceName(mac)}</Text>
                    {getDeviceSubtitle(mac) && (
                      <Text size="sm" c="dimmed">
                        {getDeviceSubtitle(mac)}
                      </Text>
                    )}
                  </Group>
                </Group>
              </Accordion.Control>
              <Accordion.Panel>
                <Stack gap="xs">
                  {sortedDomains.map(([domain, count]) => (
                    <DomainRow
                      key={domain}
                      domain={domain}
                      count={count}
                      isTargeted={isDomainTargeted(domain)}
                      sets={sets}
                      onAdded={onRefreshSets}
                    />
                  ))}
                </Stack>
              </Accordion.Panel>
            </Accordion.Item>
          );
        })}
      </Accordion>
    </Box>
  );
};

interface DomainRowProps {
  domain: string;
  count: number;
  isTargeted: boolean;
  sets: B4SetConfig[];
  onAdded: () => void;
}

const DomainRow = ({
  domain,
  count,
  isTargeted,
  sets,
  onAdded,
}: DomainRowProps) => {
  const [adding, setAdding] = useState(false);

  const handleAdd = async (setId: string) => {
    setAdding(true);
    try {
      await setsApi.addDomainToSet(setId, domain);
      onAdded();
    } catch (e) {
      console.error("Failed to add domain:", e);
    } finally {
      setAdding(false);
    }
  };

  const enabledSets = sets.filter((s) => s.enabled);

  return (
    <Box>
      <Group justify="space-between" wrap="nowrap">
        <Group>
          <Text>{domain}</Text>
          <Text>{formatNumber(count)}</Text>
        </Group>

        {isTargeted ?
          <Tooltip label="Already in a set" position="left">
            <ActionIcon variant="subtle" disabled>
              <IconCheck size={16} />
            </ActionIcon>
          </Tooltip>
        : enabledSets.length > 0 && (
            <Menu>
              <Menu.Target>
                <Tooltip label="Add to set" position="left">
                  <ActionIcon variant="subtle" disabled={adding}>
                    <IconAdd size={16} />
                  </ActionIcon>
                </Tooltip>
              </Menu.Target>
              <Menu.Dropdown>
                {enabledSets.map((set) => (
                  <Menu.Item
                    key={set.id}
                    onClick={() => void handleAdd(set.id)}
                  >
                    {set.name}
                  </Menu.Item>
                ))}
              </Menu.Dropdown>
            </Menu>
          )
        }
      </Group>
    </Box>
  );
};
