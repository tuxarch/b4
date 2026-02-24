import { useCallback, useEffect, useState } from "react";

import { clearAsnLookupCache } from "@hooks/useDomainActions";
import {
  Badge,
  Button,
  Group,
  Modal,
  Radio,
  ScrollArea,
  Select,
  Stack,
  Text,
} from "@mantine/core";
import { B4SetConfig, MAIN_SET_ID } from "@models/config";
import { asnStorage } from "@utils";

interface AddIpModalProps {
  opened: boolean;
  onClose: () => void;
  ip: string;
  variants: string[];
  sets: B4SetConfig[];
  onAdd: (entries: string[], setId: string, setName?: string) => void;
  onAddHostname?: (hostname: string) => void;
}

export const AddIpModal = ({
  opened,
  onClose,
  ip,
  variants,
  sets,
  onAdd,
  onAddHostname,
}: AddIpModalProps) => {
  const [selectedSetId, setSelectedSetId] = useState("");
  const [selectedVariant, setSelectedVariant] = useState("");
  const [prefixes, setPrefixes] = useState<string[]>([]);
  const [addMode, setAddMode] = useState<"single" | "all">("single");
  const [ipInfo, setIpInfo] = useState<{
    hostname?: string;
    org?: string;
  } | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (opened) {
      setSelectedVariant(variants[0] || ip);
      setPrefixes([]);
      setIpInfo(null);
      setAddMode("single");
      setSelectedSetId(sets[0]?.id ?? MAIN_SET_ID);
    }
  }, [opened, ip, variants, sets]);

  const loadRipe = useCallback(async () => {
    const cleanIp = ip.split(":")[0].replaceAll(/[[\]]/g, "");
    setLoading(true);
    try {
      const [netRes, asnRes] = await Promise.all([
        fetch(`/api/integration/ripestat?ip=${encodeURIComponent(cleanIp)}`),
        fetch(
          `/api/integration/ipinfo?ip=${encodeURIComponent(cleanIp)}`,
        ).catch(() => null),
      ]);
      if (netRes.ok) {
        const { data } = (await netRes.json()) as { data: { asns?: string[] } };
        const asn = data?.asns?.[0];
        if (asn) {
          const prefixRes = await fetch(
            `/api/integration/ripestat/asn?asn=${encodeURIComponent(asn)}`,
          );
          if (prefixRes.ok) {
            const { data: p } = (await prefixRes.json()) as {
              data: { prefixes: { prefix: string }[] };
            };
            const loaded = p.prefixes.map((x) => x.prefix);
            setPrefixes(loaded);
            setAddMode("all");
            asnStorage.addAsn(asn, `AS${asn}`, loaded);
            clearAsnLookupCache();
          }
        }
      }
      if (asnRes?.ok) {
        const info = (await asnRes.json()) as {
          hostname?: string;
          org?: string;
        };
        if (info.hostname || info.org) setIpInfo(info);
      }
    } finally {
      setLoading(false);
    }
  }, [ip]);

  const entries =
    prefixes.length && addMode === "all" ?
      prefixes
    : [selectedVariant || variants[0] || ip];
  const handleAdd = async () => {
    await onAdd(entries, selectedSetId);
    onClose();
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Add IP/CIDR"
      centered
      size="md"
    >
      <Stack>
        <Group justify="space-between">
          <Text size="sm">
            IP: <Badge variant="light">{ip}</Badge>
          </Text>
          <Button
            variant="light"
            size="xs"
            onClick={() => void loadRipe()}
            loading={loading}
          >
            Load Network Info
          </Button>
        </Group>

        {ipInfo?.hostname && onAddHostname && (
          <Button
            variant="subtle"
            size="xs"
            onClick={() => {
              onAddHostname(ipInfo.hostname!);
              onClose();
            }}
          >
            Add as domain: {ipInfo.hostname}
          </Button>
        )}

        {sets.length > 0 && (
          <Select
            label="Add to set"
            data={sets.map((s) => ({ label: s.name, value: s.id }))}
            value={selectedSetId}
            onChange={(v) => v && setSelectedSetId(v)}
          />
        )}

        {prefixes.length > 0 ?
          <Radio.Group
            label="Add"
            value={addMode}
            onChange={(v) => setAddMode(v as "single" | "all")}
          >
            <Stack gap="xs">
              <Radio value="single" label={`${ip} only`} />
              <Radio value="all" label={`All ${prefixes.length} prefixes`} />
            </Stack>
          </Radio.Group>
        : <Radio.Group
            label="CIDR variant"
            value={selectedVariant}
            onChange={setSelectedVariant}
          >
            <ScrollArea h={200}>
              <Stack gap="xs">
                {variants.map((v) => (
                  <Radio key={v} value={v} label={v} />
                ))}
              </Stack>
            </ScrollArea>
          </Radio.Group>
        }

        <Group justify="flex-end" mt="md">
          <Button onClick={handleAdd}>Add</Button>
        </Group>
      </Stack>
    </Modal>
  );
};
