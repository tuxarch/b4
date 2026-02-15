import { IconGitHub } from "@b4.icons";
import { GitHubRelease, compareVersions } from "@hooks/useGitHubRelease";
import { useSystemUpdate } from "@hooks/useSystemUpdate";
import {
  Alert,
  Badge,
  Button,
  Center,
  Divider,
  Group,
  Modal,
  ScrollArea,
  Select,
  Stack,
  Switch,
  Text,
} from "@mantine/core";
import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import { Link } from "react-router";

interface UpdateModalProps {
  open: boolean;
  onClose: () => void;
  onDismiss: () => void;
  currentVersion: string;
  releases: GitHubRelease[];
  includePrerelease: boolean;
  onTogglePrerelease: (include: boolean) => void;
}

export const UpdateModal = ({
  open,
  onClose,
  onDismiss,
  currentVersion,
  releases,
  includePrerelease,
  onTogglePrerelease,
}: UpdateModalProps) => {
  const { performUpdate, waitForReconnection } = useSystemUpdate();
  const [status, setStatus] = useState<
    "idle" | "updating" | "reconnecting" | "success" | "error"
  >("idle");
  const [message, setMessage] = useState("");
  const [selectedVersion, setSelectedVersion] = useState(
    releases[0]?.tag_name || "",
  );

  useEffect(() => {
    if (releases.length > 0 && !selectedVersion) {
      setSelectedVersion(releases[0].tag_name);
    }
  }, [releases, selectedVersion]);

  useEffect(() => {
    if (!open) {
      setStatus("idle");
      setMessage("");
    }
  }, [open]);

  const selectedRelease =
    releases.find((r) => r.tag_name === selectedVersion) || releases[0];
  const isDowngrade =
    selectedVersion &&
    compareVersions(`v${currentVersion}`, selectedVersion) > 0;
  const isCurrent = selectedVersion === `v${currentVersion}`;
  const isUpdating = status === "updating" || status === "reconnecting";

  const handleUpdate = async () => {
    setStatus("updating");
    setMessage("Initiating update...");
    const result = await performUpdate(selectedVersion);
    if (!result?.success) {
      setStatus("error");
      setMessage(result?.message || "Failed to initiate update.");
      return;
    }
    setMessage("Update in progress. Waiting for service to restart...");
    setStatus("reconnecting");
    const reconnected = await waitForReconnection();
    if (reconnected) {
      setStatus("success");
      setMessage("Update completed successfully! Refreshing...");
      setTimeout(() => globalThis.window.location.reload(), 5000);
    } else {
      setStatus("error");
      setMessage(
        "Update may have completed but service did not restart. Please check manually.",
      );
    }
  };

  const titles = {
    idle: "Version Management",
    updating: "Updating B4",
    reconnecting: "Updating B4",
    success: "Update Successful",
    error: "Update Failed",
  };

  return (
    <Modal
      title={titles[status]}
      opened={open}
      size="auto"
      onClose={isUpdating ? () => {} : onClose}
      closeOnClickOutside={!isUpdating}
    >
      <Stack gap="md">
        {message &&
          (status === "success" || status === "error" ?
            <Alert color={status === "success" ? "green" : "red"}>
              {message}
            </Alert>
          : <Text>{message}</Text>)}

        {status === "idle" && (
          <>
            <Group>
              <Select
                value={selectedVersion}
                onChange={(value) => value && setSelectedVersion(value)}
                data={releases.map((r) => {
                  const prerelease = r.prerelease ? " (pre-release)" : "";
                  const currentVersionTag = `v${currentVersion}`;
                  const current =
                    r.tag_name === currentVersionTag ? " (current)" : "";
                  return {
                    value: r.tag_name,
                    label: `${r.tag_name}${prerelease}${current}`,
                  };
                })}
              />
              <Switch
                checked={includePrerelease}
                onChange={(e) => onTogglePrerelease(e.target.checked)}
                label="Include pre-releases"
              />
            </Group>
            <Group>
              <Badge>Current: v{currentVersion}</Badge>
              {!isCurrent && (
                <Badge>{isDowngrade ? "Downgrade" : "Upgrade"}</Badge>
              )}
              {selectedRelease?.prerelease && <Badge>Pre-release</Badge>}
            </Group>
          </>
        )}
        <Divider />
        {selectedRelease && status === "idle" && (
          <ScrollArea h={300}>
            <ReactMarkdown>
              {selectedRelease.body || "No release notes available."}
            </ReactMarkdown>
          </ScrollArea>
        )}

        <Center>
          <Group>
            <Button
              component={Link}
              variant="subtle"
              leftSection={<IconGitHub />}
              to="https://github.com/DanielLavrushin/b4/blob/main/changelog.md"
              target="_blank"
            >
              Full Changelog
            </Button>
            {selectedRelease && (
              <Button
                component={Link}
                variant="subtle"
                leftSection={<IconGitHub />}
                to={selectedRelease.html_url}
                target="_blank"
              >
                View on GitHub
              </Button>
            )}
          </Group>
        </Center>

        <Group justify="flex-end">
          <Button onClick={onDismiss} variant="subtle" disabled={isUpdating}>
            Don't Show Again
          </Button>
          {status === "idle" && (
            <Button
              onClick={() => void handleUpdate()}
              disabled={isUpdating || isCurrent}
            >
              {isDowngrade ? "Downgrade" : "Update"}
            </Button>
          )}
          {status === "success" && (
            <Button onClick={() => globalThis.window.location.reload()}>
              Reload Page
            </Button>
          )}
        </Group>
      </Stack>
    </Modal>
  );
};
