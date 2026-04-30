import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  MinioStorageProvider,
  StorageProvider,
} from "@/pages/ui/services/models/service";
import { Dispatch, SetStateAction } from "react";

interface Props {
  selectedProvider: StorageProvider;
  setSelectedProvider: Dispatch<SetStateAction<StorageProvider | null>>;
}

export default function MinioForm({
  selectedProvider,
  setSelectedProvider,
}: Props) {
  const minioProvider = selectedProvider as MinioStorageProvider;

  const updateProvider = (
    key: keyof MinioStorageProvider | keyof StorageProvider,
    value: string | boolean
  ) => {
    setSelectedProvider((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        [key]: value,
      } as StorageProvider;
    });
  };

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 10,
      }}
    >
      <Input
        id="id-input"
        value={selectedProvider.id}
        autoFocus={false}
        placeholder="ID"
        label="ID"
        onChange={(e) => updateProvider("id", e.target.value)}
      />
      <Input
        id="endpoint-input"
        value={minioProvider.endpoint}
        placeholder="Endpoint"
        label="Endpoint"
        onChange={(e) => updateProvider("endpoint", e.target.value)}
      />
      <Input
        id="region-input"
        value={minioProvider.region}
        placeholder="Region"
        label="Region"
        onChange={(e) => updateProvider("region", e.target.value)}
      />
      <Input
        id="access-key-input"
        value={minioProvider.access_key}
        placeholder="Access key"
        title="Access Key"
        label="Access key"
        type="password"
        onChange={(e) => updateProvider("access_key", e.target.value)}
      />
      <Input
        id="secret-key-input"
        value={minioProvider.secret_key}
        placeholder="Secret access key"
        title="Secret Access Key"
        label="Secret access key"
        type="password"
        onChange={(e) => updateProvider("secret_key", e.target.value)}
      />
      <div className="flex items-center space-x-2">
        <Switch
          id="verify-switch"
          checked={minioProvider.verify}
          onCheckedChange={(checked) => updateProvider("verify", checked)}
        />
        <Label>Verify</Label>
      </div>
    </div>
  );
}
