import { useState, KeyboardEvent } from "react";
import { PlusCircle, Pencil, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import useServicesContext from "@/pages/ui/services/context/ServicesContext";
import {
  StoragePath,
  StorageProviders,
} from "@/pages/ui/services/models/service";
import ServiceFormCell from "../FormCell";
import Divider from "@/components/ui/divider";
import OscarColors, { OscarStyles } from "@/styles";

type IOType = "inputs" | "outputs";

export default function InputOutputEditor() {
  const { formService, setFormService } = useServicesContext();
  const providers = formService.storage_providers;
  const [editingItem, setEditingItem] = useState<{
    item: StoragePath;
    index: number;
  } | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingType, setEditingType] = useState<IOType>("inputs");

  const handleSave = (item: StoragePath) => {
    const updatedService = { ...formService };
    if (editingType === "inputs") {
      if (editingItem !== null) {
        updatedService.input[editingItem.index] = item;
      } else {
        updatedService.input.push(item);
      }
    } else {
      if (editingItem !== null) {
        updatedService.output[editingItem.index] = item;
      } else {
        updatedService.output.push(item);
      }
    }
    setFormService(updatedService);
    setIsModalOpen(false);
    setEditingItem(null);
  };

  const handleDelete = (type: IOType, index: number) => {
    const updatedService = { ...formService };
    if (type === "inputs") {
      updatedService.input.splice(index, 1);
    } else {
      updatedService.output.splice(index, 1);
    }
    setFormService(updatedService);
  };

  const openEditModal = (type: IOType, item: StoragePath, index: number) => {
    setEditingType(type);
    setEditingItem({ item, index });
    setIsModalOpen(true);
  };

  const openCreateModal = (type: IOType) => {
    setEditingType(type);
    setEditingItem(null);
    setIsModalOpen(true);
  };

  const renderIOItems = (type: IOType, items: StoragePath[]) => (
    <>
      {items.length === 0 && (
        <div style={{ textAlign: "center", color: OscarColors.DarkGrayText }}>
          No {type} defined yet.
        </div>
      )}
      {items.map((item, index) => (
        <div
          key={item.storage_provider + '-' + index}
          style={{
            border: OscarStyles.border,
            background: "white",
            height: 74,
            borderRadius: 8,
            display: "flex",
            flexDirection: "row",
            alignItems: "center",
            justifyContent: "flex-start",
            gap: 16,
            padding: "10px 14px",
          }}
        >
          <div
            style={{
              display: "flex",
              height: "100%",
              flexDirection: "column",
              justifyContent: "space-evenly",
            }}
          >
            <h1>{item.storage_provider}</h1>
            <h3 style={{ fontSize: 12, color: OscarColors.DarkGrayText }}>
              {item.path}
            </h3>
          </div>
          <div style={{ display: "flex", flexDirection: "column", rowGap: 8 }}>
            <div className="flex flex-row flex-wrap gap-2 items-baseline">
              <h3 style={{ fontSize: 12, color: OscarColors.DarkGrayText }}>
                Prefixes:
              </h3>
              {!item.prefix?.length && (
                <h3 style={{ fontSize: 12, color: OscarColors.DarkGrayText }}>
                  None
                </h3>
              )}
              {item.prefix?.slice(0, 3).map((prefix, prefixIndex) => (
                <Badge
                  key={`prefix-${prefixIndex}`}
                  variant="secondary"
                  style={{
                    maxWidth: "100px",
                    overflow: "hidden",
                  }}
                >
                  <span
                    style={{
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      overflow: "hidden",
                    }}
                  >
                    {prefix}
                  </span>
                </Badge>
              ))}
              {item.prefix?.length > 3 && (
                <Badge variant="secondary">+{item.prefix.length - 3}</Badge>
              )}
            </div>
            <div className="flex flex-row flex-wrap gap-2 items-baseline">
              <h3 style={{ fontSize: 12, color: OscarColors.DarkGrayText }}>
                Suffixes:
              </h3>
              {!item.suffix?.length && (
                <h3 style={{ fontSize: 12, color: OscarColors.DarkGrayText }}>
                  None
                </h3>
              )}
              {item.suffix?.slice(0, 3).map((suffix, suffixIndex) => (
                <Badge
                  key={`suffix-${suffixIndex}`}
                  variant="secondary"
                  style={{
                    maxWidth: "100px",
                    overflow: "hidden",
                  }}
                >
                  <span
                    style={{
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      overflow: "hidden",
                    }}
                  >
                    {suffix}
                  </span>
                </Badge>
              ))}
              {item.suffix?.length > 3 && (
                <Badge variant="secondary">+{item.suffix.length - 3}</Badge>
              )}
            </div>
          </div>
          <div style={{ display: "flex", flexDirection: "row" }}>
            <Button
              id={`edit-input-output-button-${index}`}
              style={{
                minWidth: 40,
                height: 40,
              }}
              size="icon"
              variant={"ghost"}
              onClick={() => openEditModal(type, item, index)}
            >
              <Pencil />
            </Button>
            <Button
              id={`delete-input-output-button-${index}`}
              style={{
                minWidth: 40,
                height: 40,
              }}
              size="icon"
              variant={"ghost"}
              onClick={() => handleDelete(type, index)}
            >
              <Trash2 color={OscarColors.Red} />
            </Button>
          </div>
        </div>
      ))}
    </>
  );

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "row",
        flexGrow: 1,
      }}
    >
      <div style={{ flexGrow: 1 }}>
        <ServiceFormCell
          title="Inputs"
          button={
            <Button
              id="add-input-button"
              variant="default"
              onClick={() => openCreateModal("inputs")}
            >
              <PlusCircle className="mr-2 h-4 w-4" />
              Add Input
            </Button>
          }
        >
          <div
            style={{
              display: "flex",
              flexDirection: "row",
              gap: 16,
              flexWrap: "wrap",
            }}
          >
            {renderIOItems("inputs", formService.input)}
          </div>
        </ServiceFormCell>
      </div>

      <Divider orientation="vertical" />

      <div style={{ flexGrow: 1 }}>
        <ServiceFormCell
          title="Outputs"
          button={
            <Button
              id="add-output-button"
              variant="default"
              onClick={() => openCreateModal("outputs")}
            >
              <PlusCircle className="mr-2 h-4 w-4" />
              Add Output
            </Button>
          }
        >
          <div
            style={{
              display: "flex",
              flexGrow: 1,
              flexDirection: "row",
              gap: 16,
              flexWrap: "wrap",
            }}
          >
            {renderIOItems("outputs", formService.output)}
          </div>
        </ServiceFormCell>
      </div>

      <Dialog open={isModalOpen} onOpenChange={setIsModalOpen}>
        <DialogContent style={{ width: "500px" }}>
          <DialogHeader>
            <DialogTitle>
              {editingItem ? "Edit" : "Create"}{" "}
              {editingType === "inputs" ? "Input" : "Output"}
            </DialogTitle>
          </DialogHeader>
          <EditModal
            providers={providers}
            item={
              editingItem
                ? editingItem.item
                : { storage_provider: "", path: "", prefix: [], suffix: [] }
            }
            onSave={handleSave}
          />
        </DialogContent>
      </Dialog>
    </div>
  );
}

function EditModal({
  item,
  onSave,
}: {
  item: StoragePath;
  onSave: (item: StoragePath) => void;
  providers: StorageProviders;
}) {
  const [editedItem, setEditedItem] = useState<StoragePath>({
    ...item,
    storage_provider: item.storage_provider || "minio.default",
    prefix: item.prefix || [],
    suffix: item.suffix || [],
  });
  const [newPrefix, setNewPrefix] = useState("");
  const [newSuffix, setNewSuffix] = useState("");

  const handleChange = (field: keyof StoragePath, value: string) => {
    setEditedItem((prev) => ({ ...prev, [field]: value }));
  };

  const handleAddPrefix = () => {
    if (newPrefix.trim()) {
      setEditedItem((prev) => ({
        ...prev,
        prefix: [...prev.prefix, newPrefix.trim()],
      }));
      setNewPrefix("");
    }
  };

  const handleAddSuffix = () => {
    if (newSuffix.trim()) {
      setEditedItem((prev) => ({
        ...prev,
        suffix: [...prev.suffix, newSuffix.trim()],
      }));
      setNewSuffix("");
    }
  };

  const handleRemovePrefix = (index: number) => {
    setEditedItem((prev) => ({
      ...prev,
      prefix: prev.prefix.filter((_, i) => i !== index),
    }));
  };

  const handleRemoveSuffix = (index: number) => {
    setEditedItem((prev) => ({
      ...prev,
      suffix: prev.suffix.filter((_, i) => i !== index),
    }));
  };

  const handleKeyPress = (
    e: KeyboardEvent<HTMLInputElement>,
    type: "prefix" | "suffix"
  ) => {
    if (e.key === "Enter") {
      if (type === "prefix") {
        handleAddPrefix();
      } else {
        handleAddSuffix();
      }
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="provider">Provider</Label>
        <Select
          value={editedItem.storage_provider}
          onValueChange={(value) => handleChange("storage_provider", value)}
        >
          <SelectTrigger id="provider">
            <SelectValue placeholder="Select a provider" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem key={"minio"} value={"minio.default"}>
              {"minio.default"}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="path">Path</Label>
        <Input
          id="path"
          value={editedItem.path}
          autoFocus
          onChange={(e) => handleChange("path", e.target.value)}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="prefix">Prefixes</Label>
        <div className="flex flex-wrap gap-2 mb-2">
          {editedItem.prefix?.map((prefix, index) => (
            <Badge key={index} variant="secondary">
              {prefix}
              <Button
                variant="ghost"
                size="sm"
                className="ml-1 h-4 w-4 p-0"
                onClick={() => handleRemovePrefix(index)}
              >
                <X className="h-3 w-3" />
                <span className="sr-only">Remove prefix</span>
              </Button>
            </Badge>
          ))}
        </div>
        <div className="flex gap-2">
          <Input
            id="new-prefix"
            value={newPrefix}
            onChange={(e) => setNewPrefix(e.target.value)}
            onKeyPress={(e) => handleKeyPress(e, "prefix")}
            placeholder="Add new prefix"
          />
          <Button onClick={handleAddPrefix}>Add</Button>
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="suffix">Suffixes</Label>
        <div className="flex flex-wrap gap-2 mb-2">
          {editedItem.suffix?.map((suffix, index) => (
            <Badge key={index} variant="outline">
              {suffix}
              <Button
                id={`remove-suffix-button-${index}`}
                variant="ghost"
                size="sm"
                className="ml-1 h-4 w-4 p-0"
                onClick={() => handleRemoveSuffix(index)}
              >
                <X className="h-3 w-3" />
                <span className="sr-only">Remove suffix</span>
              </Button>
            </Badge>
          ))}
        </div>
        <div className="flex gap-2">
          <Input
            id="new-suffix"
            value={newSuffix}
            onChange={(e) => setNewSuffix(e.target.value)}
            onKeyPress={(e) => handleKeyPress(e, "suffix")}
            placeholder="Add new suffix"
          />
          <Button onClick={handleAddSuffix}>Add</Button>
        </div>
      </div>
      <Button onClick={() => onSave(editedItem)}>Save</Button>
    </div>
  );
}
