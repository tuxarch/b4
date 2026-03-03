import { useEffect, useState } from "react";

import {
  Badge,
  Button,
  Group,
  Modal,
  Radio,
  Select,
  Stack,
  Text,
} from "@mantine/core";
import { B4SetConfig, MAIN_SET_ID } from "@models/config";

interface AddSniModalProps {
  opened: boolean;
  onClose: () => void;
  domain: string;
  variants: string[];
  sets: B4SetConfig[];
  onAdd: (domain: string, setId: string, setName?: string) => void;
}

export const AddSniModal = ({
  opened,
  onClose,
  domain,
  variants,
  sets,
  onAdd,
}: AddSniModalProps) => {
  const [selectedSetId, setSelectedSetId] = useState("");
  const [selectedVariant, setSelectedVariant] = useState("");

  useEffect(() => {
    if (opened) {
      setSelectedVariant(variants[0] || domain);
      setSelectedSetId(sets[0]?.id ?? MAIN_SET_ID);
    }
  }, [opened, domain, variants, sets]);

  const handleAdd = async () => {
    await onAdd(selectedVariant, selectedSetId);
    onClose();
  };

  return (
    <Modal opened={opened} onClose={onClose} title="Add Domain" centered>
      <Stack>
        <Text size="sm">
          Original domain: <Badge variant="light">{domain}</Badge>
        </Text>
        {sets.length > 0 && (
          <Select
            label="Add to set"
            data={sets.map((set) => ({ label: set.name, value: set.id }))}
            value={selectedSetId}
            onChange={(value) => value && setSelectedSetId(value)}
          />
        )}
        <Radio.Group
          label="Select variant to add"
          value={selectedVariant}
          onChange={setSelectedVariant}
        >
          <Stack gap="xs" mt="xs">
            {variants.map((variant) => (
              <Radio key={variant} label={variant} value={variant} />
            ))}
          </Stack>
        </Radio.Group>
        <Group justify="flex-end" mt="md">
          <Button onClick={handleAdd}>Add</Button>
        </Group>
      </Stack>
    </Modal>
  );
};
