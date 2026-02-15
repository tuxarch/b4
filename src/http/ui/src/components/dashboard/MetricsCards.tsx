import { formatNumber } from "@utils";
import { StatCard } from "./StatCard";

import { IconConnections, IconPackets, IconShield } from "@b4.icons";
import { Group } from "@mantine/core";
import type { Metrics } from "./Page";

interface MetricsCardsProps {
  metrics: Metrics;
}

export const MetricsCards = ({ metrics }: MetricsCardsProps) => {
  const targetRate =
    metrics.total_connections > 0 ?
      (
        (metrics.targeted_connections / metrics.total_connections) *
        100
      ).toFixed(1)
    : "0.0";

  return (
    <Group grow>
      <StatCard
        title="Connections"
        value={formatNumber(metrics.total_connections)}
        subtitle={`${metrics.current_cps.toFixed(1)} conn/s`}
        icon={<IconConnections />}
      />

      <StatCard
        title="Bypassed"
        value={formatNumber(metrics.targeted_connections)}
        subtitle={`${targetRate}%`}
        icon={<IconShield />}
      />

      <StatCard
        title="Packets"
        value={formatNumber(metrics.packets_processed)}
        subtitle={`${metrics.current_pps.toFixed(1)} pkt/s`}
        icon={<IconPackets />}
      />
    </Group>
  );
};
