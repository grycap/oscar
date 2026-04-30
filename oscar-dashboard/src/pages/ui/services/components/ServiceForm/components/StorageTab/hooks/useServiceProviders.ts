import useUpdate from "@/hooks/useUpdate";
import useServicesContext from "@/pages/ui/services/context/ServicesContext";
import {
  StorageProvider,
  StorageProviders,
  StorageProviderType,
} from "@/pages/ui/services/models/service";
import { useMemo, useState } from "react";

function useServiceProviders() {
  const { formService, setFormService } = useServicesContext();

  //This function transforms from {s3: {...}, minio: {...}} to an array with all providers
  const initialProviders = useMemo(() => {
    const storageProviders = formService.storage_providers;
    const list: StorageProvider[] = [];

    Object.keys(storageProviders).forEach((key) => {
      const item = storageProviders[key as StorageProviderType] as Record<
        string,
        StorageProvider
      >;

      if (!item) return;

      Object.entries(item).forEach(([id, provider]) => {
        list.push({
          ...provider,
          id,
          type: key as StorageProviderType,
        });
      });
    });

    return list;
  }, []);

  const [providers, setProviders] = useState(initialProviders);

  //Updates the service object with the correct model
  async function updateServiceProviders() {
    let newServiceProviders: StorageProviders = {};
    providers.forEach((providerItem) => {
      const { type, ...providerWithoutType } = providerItem;
      newServiceProviders = {
        ...newServiceProviders,
        [providerItem.type]: {
          ...newServiceProviders[providerItem.type],
          [providerItem.id]: providerWithoutType,
        },
      };
    });

    setFormService((old) => {
      return {
        ...old,
        storage_providers: newServiceProviders,
      };
    });
  }

  useUpdate(() => {
    updateServiceProviders();
  }, [providers]);

  return {
    providers,
    setProviders,
  };
}

export default useServiceProviders;
