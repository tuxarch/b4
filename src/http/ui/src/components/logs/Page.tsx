import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { IconDelete } from "@b4.icons";
import { useWebSocket } from "@context/B4WsProvider";
import {
  ActionIcon,
  Badge,
  Card,
  Group,
  Switch,
  TextInput,
} from "@mantine/core";
import { useElementSize } from "@mantine/hooks";
import { DataTable } from "mantine-datatable";

export function LogsPage() {
  const [filter, setFilter] = useState("");
  const { logs, pauseLogs, setPauseLogs, clearLogs } = useWebSocket();

  const filtered = useMemo(() => {
    const f = filter.trim().toLowerCase();
    return f ? logs.filter((l) => l.toLowerCase().includes(f)) : logs;
  }, [logs, filter]);

  const handleHotkeysDown = useCallback(
    (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      if ((e.ctrlKey && e.key === "x") || e.key === "Delete") {
        e.preventDefault();
        clearLogs();
      } else if (e.key === "p" || e.key === "Pause") {
        e.preventDefault();
        setPauseLogs(!pauseLogs);
      }
    },
    [clearLogs, pauseLogs, setPauseLogs],
  );

  useEffect(() => {
    globalThis.window.addEventListener("keydown", handleHotkeysDown);
    return () => {
      globalThis.window.removeEventListener("keydown", handleHotkeysDown);
    };
  }, [handleHotkeysDown]);
  const { ref, height } = useElementSize();

  const viewport = useRef<HTMLDivElement>(null);
  const wasAtBottom = useRef(true);
  useEffect(() => {
    const el = viewport.current;
    if (!el) return;
    const handler = () => {
      wasAtBottom.current = el.scrollTop + el.clientHeight >= el.scrollHeight;
    };
    el.addEventListener("scroll", handler);
    return () => el.removeEventListener("scroll", handler);
  }, []);

  useEffect(() => {
    if (wasAtBottom.current) {
      scrollToBottom();
    }
  }, [filtered]);

  const scrollToBottom = () =>
    viewport.current!.scrollTo({
      top: viewport.current!.scrollHeight,
    });
  return (
    <>
      {/* Controls Bar */}
      <Card>
        <Group ref={ref} justify="space-between">
          <TextInput
            placeholder="Filter logs..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            flex={1}
          />
          <Group>
            <Badge>{`${logs.length} lines`}</Badge>
            {filter && <Badge>{`${filtered.length} filtered`}</Badge>}

            <Switch
              label={pauseLogs ? "Paused" : "Streaming"}
              checked={pauseLogs}
              onChange={(event) => setPauseLogs(event.currentTarget.checked)}
            />
            <ActionIcon onClick={clearLogs}>
              <IconDelete />
            </ActionIcon>
          </Group>
        </Group>
      </Card>
      <DataTable
        records={filtered.map((line, i) => ({ logs: line, index: i + 1 }))}
        noRecordsText="No logs yet..."
        scrollViewportRef={viewport}
        columns={[{ accessor: "logs", title: "Logs" }]}
        height={`calc(100dvh - var(--app-shell-header-height) - 2 * var(--mantine-spacing-md) - ${height}px)`}
        withRowBorders={false}
        noHeader
        withTableBorder
        highlightOnHover
      />
    </>
  );
}
