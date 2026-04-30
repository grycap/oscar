import { useState } from "react";
import ServiceFormCell from "../FormCell";
import AddProviderButton from "./components/AddProviderButton";
import useServiceProviders from "./hooks/useServiceProviders";
import { StorageProvider } from "@/pages/ui/services/models/service";
import ProvidersListItem from "./components/ProvidersListItem";
import CreateUpdateStorageProviderModal from "./components/StorageProviderModal";
import DeleteDialog from "@/components/DeleteDialog";

function ServicesStorageTab() {
  const { providers, setProviders } = useServiceProviders();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedProvider, setSelectedProvider] =
    useState<StorageProvider | null>(null);

  function handleUpdate() {
    setProviders((prev) => {
      const isNew = !selectedId;
      if (isNew) {
        prev.push(selectedProvider as StorageProvider);
      }

      const newProviders = prev.map((provider) => {
        if (provider.id === selectedId) {
          return selectedProvider as StorageProvider;
        }
        return provider;
      });

      return newProviders;
    });
  }

  function handleDelete(id: string) {
    setProviders((prev) => prev.filter((provider) => provider.id !== id));
    setItemToDelete(null);
  }

  function onClose() {
    setSelectedId(null);
    setSelectedProvider(null);
  }

  const [itemToDelete, setItemToDelete] = useState<string | null>(null);
  const isDeleteDialogOpen = itemToDelete !== null;

  return (
    <>
      {isDeleteDialogOpen && (
        <DeleteDialog
          isOpen={isDeleteDialogOpen}
          onClose={() => setItemToDelete(null)}
          onDelete={() => handleDelete(itemToDelete)}
          itemNames={itemToDelete}
        />
      )}
      {selectedProvider && (
        <CreateUpdateStorageProviderModal
          selectedProvider={selectedProvider}
          onClose={onClose}
          onUpdate={handleUpdate}
          setSelectedProvider={setSelectedProvider}
        />
      )}
      <div
        style={{
          flexGrow: 1,
          flexBasis: 0,
          display: "flex",
          flexDirection: "column",
        }}
      >
        <ServiceFormCell
          title="Storage configuration"
          button={
            <AddProviderButton setSelectedProvider={setSelectedProvider} />
          }
        >
          <div
            style={{
              display: "flex",
              flexDirection: "row",
              flexWrap: "wrap",
              width: "100%",
              gap: "10px",
            }}
          >
            {providers.map((item, index) => (
              <ProvidersListItem
                key={index}
                provider={item}
                setSelectedProvider={setSelectedProvider}
                setSelectedId={setSelectedId}
                onDelete={() => {
                  setItemToDelete(item.id);
                }}
              />
            ))}
          </div>
        </ServiceFormCell>
      </div>
    </>
  );
}

export default ServicesStorageTab;
