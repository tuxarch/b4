import { B4SetConfig } from "@models/config";

import { Badge, Box, Group, Text } from "@mantine/core";
import { useNavigate } from "react-router";

interface ActiveSetsProps {
  sets: B4SetConfig[];
}

export const ActiveSets = ({ sets }: ActiveSetsProps) => {
  const navigate = useNavigate();

  if (sets.length === 0) return null;

  return (
    <Box>
      <Text>Active Sets</Text>
      <Group>
        {sets.map((set) => {
          const domainCount =
            (set.targets.sni_domains?.length || 0) +
            (set.targets.geosite_categories?.length || 0);
          const ipCount =
            (set.targets.ip?.length || 0) +
            (set.targets.geoip_categories?.length || 0);
          const totalTargets = domainCount + ipCount;

          return (
            <Badge
              key={set.id}
              onClick={() => {
                navigate(`/sets/${set.id}`)?.catch(() => {});
              }}
            >
              {`${set.name}: ${totalTargets} targets`}
            </Badge>
          );
        })}
      </Group>
    </Box>
  );
};
