import { useMemo, useState } from "react";

import { IconAdd } from "@b4.icons";
import { setsApi } from "@b4.sets";
import {
  Accordion,
  ActionIcon,
  Box,
  Group,
  Menu,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import { B4SetConfig } from "@models/config";
import { formatNumber } from "@utils";

interface UnmatchedDomainsProps {
  topDomains: Record<string, number>;
  sets: B4SetConfig[];
  targetedDomains: Set<string>;
  onRefreshSets: () => void;
}

export const UnmatchedDomains = ({
  topDomains,
  sets,
  targetedDomains,
  onRefreshSets,
}: UnmatchedDomainsProps) => {
  const isDomainTargeted = (domain: string): boolean => {
    if (targetedDomains.has(domain)) return true;
    const parts = domain.split(".");
    for (let i = 1; i < parts.length; i++) {
      if (targetedDomains.has(parts.slice(i).join("."))) return true;
    }
    return false;
  };

  const unmatched = useMemo(() => {
    return Object.entries(topDomains)
      .filter(([domain]) => !isDomainTargeted(domain))
      .sort((a, b) => b[1] - a[1])
      .slice(0, 15);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topDomains, targetedDomains]);

  if (unmatched.length === 0) return null;

  return (
    <Accordion variant="contained" chevronPosition="left">
      <Accordion.Item value="unmatched">
        <Accordion.Control>
          <Text>Domains Not In Any Set</Text>
        </Accordion.Control>
        <Accordion.Panel>
          <Stack gap="xs">
            {unmatched.map(([domain, count]) => (
              <UnmatchedRow
                key={domain}
                domain={domain}
                count={count}
                sets={sets}
                onAdded={onRefreshSets}
              />
            ))}
          </Stack>
        </Accordion.Panel>
      </Accordion.Item>
    </Accordion>
  );
};

interface UnmatchedRowProps {
  domain: string;
  count: number;
  sets: B4SetConfig[];
  onAdded: () => void;
}

const UnmatchedRow = ({ domain, count, sets, onAdded }: UnmatchedRowProps) => {
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

  return (
    <Box>
      <Group justify="space-between" wrap="nowrap">
        <Group>
          <Text>{domain}</Text>
          <Text>{formatNumber(count)}</Text>
        </Group>

        <Menu>
          <Menu.Target>
            <Tooltip label="Add to set" position="left">
              <ActionIcon variant="subtle" disabled={adding}>
                <IconAdd size={16} />
              </ActionIcon>
            </Tooltip>
          </Menu.Target>
          <Menu.Dropdown>
            {sets
              .filter((s) => s.enabled)
              .map((set) => (
                <Menu.Item key={set.id} onClick={() => void handleAdd(set.id)}>
                  {set.name}
                </Menu.Item>
              ))}
          </Menu.Dropdown>
        </Menu>
      </Group>
    </Box>
  );
};
