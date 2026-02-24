import { useSystemRestart } from "@hooks/useSystemRestart";

import { IconRefresh } from "@b4.icons";
import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Group,
  Modal,
  Text,
  Tooltip,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import type { Metrics } from "./Page";

interface HealthBannerProps {
  metrics: Metrics;
  connected: boolean;
  version: string | null;
}

type HealthLevel = "healthy" | "degraded" | "critical";

function deriveHealth(metrics: Metrics, connected: boolean): HealthLevel {
  if (!connected) return "critical";
  if (
    metrics.nfqueue_status === "unknown" ||
    metrics.tables_status === "unknown"
  )
    return "degraded";
  const activeWorkers = metrics.worker_status.filter(
    (w) => w.status === "active",
  ).length;
  if (activeWorkers === 0 && metrics.worker_status.length > 0)
    return "critical";
  if (activeWorkers < metrics.worker_status.length) return "degraded";
  return "healthy";
}

const healthConfig = new Map<HealthLevel, { color: string; label: string }>([
  ["healthy", { color: "green", label: "Running" }],
  ["degraded", { color: "yellow", label: "Degraded" }],
  ["critical", { color: "red", label: "Critical" }],
]);

export const HealthBanner = ({
  metrics,
  connected,
  version,
}: HealthBannerProps) => {
  const {
    restart,
    waitForReconnection,
    loading: restarting,
  } = useSystemRestart();

  const health = deriveHealth(metrics, connected);
  const config = healthConfig.get(health) ?? {
    color: "red",
    label: "Critical",
  };
  const activeWorkers = metrics.worker_status.filter(
    (w) => w.status === "active",
  ).length;
  const totalWorkers = metrics.worker_status.length;

  const handleRestart = async () => {
    close();
    const result = await restart();
    if (result?.success) {
      await waitForReconnection();
    }
  };

  const [opened, { open, close }] = useDisclosure(false);

  return (
    <Card>
      <Group justify="space-between" wrap="nowrap">
        <Group>
          <Badge
            color={config.color}
          >{`NFQueue: ${metrics.nfqueue_status}`}</Badge>

          <Badge
            color={config.color}
          >{`Firewall: ${metrics.tables_status}`}</Badge>

          <Badge
            color={
              activeWorkers === totalWorkers && totalWorkers > 0 ?
                "green"
              : "yellow"
            }
          >{`Workers: ${activeWorkers}/${totalWorkers} active`}</Badge>

          <Text>Uptime: {metrics.uptime}</Text>

          {version && <Text>v{version}</Text>}
        </Group>

        <Tooltip
          label={restarting ? "Restarting..." : "Restart b4"}
          position="left"
        >
          <ActionIcon onClick={open} disabled={restarting} variant="subtle">
            <IconRefresh />
          </ActionIcon>
        </Tooltip>

        <Modal opened={opened} onClose={close} title="Restart b4">
          <Text>
            Are you sure you want to restart b4? Active connections will be
            interrupted.
          </Text>

          <Button mt="lg" fullWidth onClick={() => void handleRestart()}>
            Restart
          </Button>
        </Modal>
      </Group>
    </Card>
  );
};
