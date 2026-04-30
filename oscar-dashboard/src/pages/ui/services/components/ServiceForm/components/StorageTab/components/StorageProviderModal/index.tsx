import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StorageProvider } from "@/pages/ui/services/models/service";
import MinioForm from "./forms/minio";
import { Dispatch, SetStateAction } from "react";
import { Button } from "@/components/ui/button";

interface Props {
  onClose: () => void;
  selectedProvider: StorageProvider;
  setSelectedProvider: Dispatch<SetStateAction<StorageProvider | null>>;
  onUpdate: () => void;
}

function CreateUpdateStorageProviderModal({
  onClose,
  selectedProvider,
  setSelectedProvider,
  onUpdate,
}: Props) {
  return (
    <Dialog defaultOpen onOpenChange={onClose}>
      <DialogContent style={{ paddingBottom: "1rem", width: "500px" }}>
        <DialogHeader>
          <DialogTitle>
            {selectedProvider.type.charAt(0).toUpperCase() +
              selectedProvider.type.slice(1)}{" "}
            {" provider configuration"}
          </DialogTitle>
        </DialogHeader>
        {selectedProvider.type === "minio" && (
          <MinioForm
            selectedProvider={selectedProvider}
            setSelectedProvider={setSelectedProvider}
          />
        )}
        <div
          style={{ width: "100%", display: "flex", justifyContent: "flex-end" }}
        >
          <Button
            variant={"mainGreen"}
            onClick={() => {
              onUpdate();
              onClose();
            }}
          >
            Save
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export default CreateUpdateStorageProviderModal;
