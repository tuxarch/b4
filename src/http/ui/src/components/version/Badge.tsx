import { Badge, Box, Text, Tooltip } from "@mantine/core";

interface VersionBadgeProps {
  version: string;
  hasUpdate?: boolean;
  isLoading?: boolean;
  onClick?: () => void;
}

export const VersionBadge = ({
  version,
  hasUpdate = false,
  isLoading = false,
  onClick,
}: VersionBadgeProps) => {
  if (isLoading) {
    return (
      <Box>
        <Text>Checking for updates...</Text>
      </Box>
    );
  }

  return (
    <Box onClick={onClick}>
      {hasUpdate ?
        <Tooltip
          position="right"
          label="New version available! Click to view details"
        >
          <Badge>{`v${version}`}</Badge>
        </Tooltip>
      : <Text>{`v${version}`}</Text>}
    </Box>
  );
};
