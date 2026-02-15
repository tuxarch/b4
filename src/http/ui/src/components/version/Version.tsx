import { IconGitHub } from "@b4.icons";
import { dismissVersion, useGitHubRelease } from "@hooks/useGitHubRelease";
import { Button, Stack } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { Link } from "react-router";
import { VersionBadge } from "./Badge";
import { UpdateModal } from "./UpdateModal";
export default function Version() {
  const [opened, { open, close }] = useDisclosure(false);
  const {
    releases,
    latestRelease,
    isNewVersionAvailable,
    isLoading,
    currentVersion,
    includePrerelease,
    setIncludePrerelease,
  } = useGitHubRelease();

  const handleDismissUpdate = () => {
    if (latestRelease) {
      dismissVersion(latestRelease.tag_name);
    }
    close();
  };

  return (
    <Stack align="center">
      <Button
        component={Link}
        variant="subtle"
        leftSection={<IconGitHub />}
        to="https://github.com/daniellavrushin/b4"
        target="_blank"
      >
        DanielLavrushin/b4
      </Button>
      <VersionBadge
        version={currentVersion}
        hasUpdate={isNewVersionAvailable}
        isLoading={isLoading}
        onClick={open}
      />

      <UpdateModal
        open={opened}
        onClose={close}
        onDismiss={handleDismissUpdate}
        currentVersion={currentVersion}
        releases={releases}
        includePrerelease={includePrerelease}
        onTogglePrerelease={setIncludePrerelease}
      />
    </Stack>
  );
}
