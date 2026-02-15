import { Box, Card, Group, Stack, Text, ThemeIcon, Title } from "@mantine/core";

interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon: React.ReactNode;
  color?: string;
  trend?: {
    value: number;
    label?: string;
  };
}

export const StatCard = ({
  title,
  value,
  subtitle,
  icon,
  trend,
}: StatCardProps) => (
  <Card>
    <Group justify="space-between" align="start" wrap="nowrap">
      <Stack gap={0}>
        <Text>{title}</Text>
        <Title component="text">{value}</Title>
        {subtitle && <Text>{subtitle}</Text>}
        {trend && (
          <Box>
            <Text c={trend.value > 0 ? "#4caf50" : "#f44336"}>
              {trend.value > 0 ? "+" : ""}
              {trend.value.toFixed(1)}%
            </Text>
            {trend.label && <Text>{trend.label}</Text>}
          </Box>
        )}
      </Stack>

      <ThemeIcon>{icon}</ThemeIcon>
    </Group>
  </Card>
);
