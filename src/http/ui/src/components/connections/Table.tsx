import {
  memo,
  useEffect,
  useMemo,
  useRef,
  useState,
  useTransition,
} from "react";

import { ParsedLog } from "@b4.connections";
import { IconAdd } from "@b4.icons";
import { useFilteredLogs } from "@hooks/useDomainActions";

import { Badge, Card, Center, Group, Switch, TextInput } from "@mantine/core";
import { useElementSize, useLocalStorage } from "@mantine/hooks";
import { DataTable, DataTableSortStatus } from "mantine-datatable";

import { sortBy } from "lodash";

export type SortColumn =
  | "timestamp"
  | "set"
  | "protocol"
  | "domain"
  | "source"
  | "destination";

export interface TableSortProps {
  logs: ParsedLog[];
  onDomainClick: (domain: string) => void;
  onIpClick: (ip: string) => void;
  showAll: boolean;
  setShowAll: (showAll: boolean) => void;
}

export const TableSort = memo(function TableSort({
  logs,
  onDomainClick,
  onIpClick,
  showAll,
  setShowAll,
}: TableSortProps) {
  const [search, setSearch] = useLocalStorage({
    key: "b4_connections_filter",
    defaultValue: "",
  });

  const [sortStatus, setSortStatus] = useLocalStorage<
    DataTableSortStatus<ParsedLog>
  >({
    key: "b4_connections_sort_status",
    defaultValue: { columnAccessor: "timestamp", direction: "desc" },
  });

  const [loading, startTransition] = useTransition();
  const [OptimisticShowAll, setOptimisticShowAll] = useState(showAll);
  const filteredLogs = useFilteredLogs(logs, search);

  const records = useMemo(() => {
    const data = sortBy(filteredLogs, sortStatus.columnAccessor) as ParsedLog[];
    return sortStatus.direction === "desc" ? data.reverse() : data;
  }, [filteredLogs, sortStatus]);

  const handleSearchChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setSearch(event.currentTarget.value);
  };

  /*
const TableRowMemo = memo<{
  log: ParsedLog;
  onDomainClick: (domain: string) => void;
  onIpClick: (ip: string) => void;
}>(
  ({ log, onDomainClick, onIpClick }) => {
    const asnName = useMemo(() => {
      if (!log.destination) return null;
      const asn = asnStorage.findAsnForIp(log.destination);
      return asn?.name || null;
    }, [log.destination]);
*/
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
  }, [records]);

  const scrollToBottom = () =>
    viewport.current!.scrollTo({
      top: viewport.current!.scrollHeight,
    });

  return (
    <>
      <Card ref={ref}>
        <Group justify="space-between">
          <TextInput
            placeholder="Search (combine with +, exclude with !, e.g. tcp+!domain:google.com)"
            value={search}
            onChange={handleSearchChange}
            ref={ref}
            flex={1}
          />
          <Switch
            label={OptimisticShowAll ? "All packets" : "Domains only"}
            checked={OptimisticShowAll}
            onChange={(event) => {
              // бесит меня блять жду виртуализацию
              setOptimisticShowAll(event.currentTarget.checked);
              if (event.currentTarget.checked) {
                startTransition(() => setShowAll(event.currentTarget.checked));
              } else {
                setShowAll(false);
              }
            }}
          />
        </Group>
      </Card>
      <DataTable
        columns={[
          {
            accessor: "timestamp",
            title: "Time",
            sortable: true,
            render: (record) => {
              const t = record.timestamp;
              return t?.includes(" ") ? t.split(" ")[1] : (t ?? "");
            },
          },
          {
            accessor: "protocol",
            title: "Type",
            sortable: true,
            render: (record) => (
              <Center>
                <Badge variant="light">{record.protocol}</Badge>
              </Center>
            ),
          },
          { accessor: "ipSet", title: "Set", sortable: true },
          {
            accessor: "domain",
            title: "Domain",
            sortable: true,
            render: (record) => (
              <Group justify="space-between" wrap="nowrap">
                {record.domain}
                {record.domain && !record.hostSet && <IconAdd />}
              </Group>
            ),
          },
          { accessor: "source", title: "Source", sortable: true },
          {
            accessor: "destination",
            title: "Destination",
            sortable: true,
            render: (record) => (
              <Group justify="space-between" wrap="nowrap">
                {record.destination} {!record.hostSet && <IconAdd />}
              </Group>
            ),
          },
        ]}
        records={records}
        noRecordsText="No connections yet..."
        scrollViewportRef={viewport}
        sortStatus={sortStatus}
        onSortStatusChange={setSortStatus}
        fetching={loading}
        onCellClick={({ record, column }) => {
          if (!record.hostSet) {
            if (column.accessor === "domain" && record.domain)
              onDomainClick(record.domain);
            if (column.accessor === "destination")
              onIpClick(record.destination);
          }
        }}
        textSelectionDisabled
        highlightOnHover
        withTableBorder
        withColumnBorders
        height={`calc(100dvh - var(--app-shell-header-height) - 2 * var(--mantine-spacing-md) - ${height}px)`}
      />
    </>
  );
});
