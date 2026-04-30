import {
  Dispatch,
  SetStateAction,
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  Service,
  ServiceFilter,
  ServiceFilterBy,
  ServiceTab,
} from "../models/service";
import getServicesApi from "@/api/services/getServicesApi";
import { useLocation } from "react-router-dom";
import { defaultService } from "../components/ServiceForm/utils/initialData";
import useUpdate from "@/hooks/useUpdate";
import getSystemConfigApi from "@/api/config/getSystemConfig";
import { ServiceViewMode } from "../components/Topbar";
import { z } from "zod";

interface ServiceContextType {
  filter: ServiceFilter;
  setFilter: Dispatch<SetStateAction<ServiceFilter>>;

  services: Service[];
  setServices: Dispatch<SetStateAction<Service[]>>;

  formTab: ServiceTab;
  setFormTab: Dispatch<SetStateAction<ServiceTab>>;

  formService: Service;
  setFormService: Dispatch<SetStateAction<Service>>;

  showFDLModal: boolean;
  setShowFDLModal: Dispatch<SetStateAction<boolean>>;

  refreshServices: () => void;
  formMode: ServiceViewMode;

  formFunctions: FormFunctions;
  servocesAreLoading: boolean;
}

type FormFunctions = {
  handleChange: (
    e: React.ChangeEvent<HTMLInputElement>,
    key: SchemaKeys
  ) => void;
  onBlur: (key: SchemaKeys) => void;
  errors: Partial<Record<keyof Service, string>>;
  setErrors: Dispatch<SetStateAction<Partial<Record<keyof Service, string>>>>;
};

export const serviceSchema = z.object({
  name: z.string().min(1, "Service name is required"),
  image: z.string().min(1, "Docker image is required"),
  cpu: z.string().min(1, "CPU cores is required"),
  memory: z.string().min(1, "Memory is required"),
  script: z.string().min(1, "Script is required"),
});

type SchemaKeys = keyof typeof serviceSchema.shape;

export const ServicesContext = createContext({
  services: [] as Service[],
} as ServiceContextType);

export const ServicesProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [services, setServices] = useState([] as Service[]);
  const [servocesAreLoading, setServocesAreLoading] = useState(false);
  const [showFDLModal, setShowFDLModal] = useState(false);
  const location = useLocation();
  const pathnames = location.pathname.split("/").filter((x) => x && x !== "ui");
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_, serviceId] = pathnames;

  // Filter and order TABLE rows
  const [filter, setFilter] = useState({
    value: "",
    type: ServiceFilterBy.Name,
    onlyOwned: false,
  } as ServiceFilter);

  //Active tab in create/update mode
  const [formTab, setFormTab] = useState(ServiceTab.Settings);

  const formMode = useMemo(() => {
    if (!serviceId) {
      return ServiceViewMode.List;
    }

    if (serviceId === "create") {
      return ServiceViewMode.Create;
    }

    return ServiceViewMode.Update;
  }, [pathnames]);

  const [formService, setFormService] = useState({} as Service);

  const [errors, setErrors] = useState<Partial<Record<keyof Service, string>>>(
    {}
  );

  function handleChange(
    e: React.ChangeEvent<HTMLInputElement>,
    key: SchemaKeys
  ) {
    setFormService((service: Service) => {
      return {
        ...service,
        [key]: e.target.value,
      };
    });

    // Validar el campo específico
    try {
      serviceSchema.shape[key].parse(e.target.value);
      setErrors((prevErrors) => ({ ...prevErrors, [key]: undefined }));
    } catch (error) {
      if (error instanceof z.ZodError) {
        setErrors((prevErrors) => ({
          ...prevErrors,
          [key]: error.errors[0].message,
        }));
      }
    }
  }

  function onBlur(key: SchemaKeys) {
    try {
      serviceSchema.shape[key].parse(formService[key]);
      setErrors((prevErrors) => ({ ...prevErrors, [key]: undefined }));
    } catch (error) {
      if (error instanceof z.ZodError) {
        setErrors((prevErrors) => ({
          ...prevErrors,
          [key]: error.errors[0].message,
        }));
      }
    }
  }

  async function handleGetServices() {
    setServocesAreLoading(true);
    const response = await getServicesApi();
    setServocesAreLoading(false);

    setServices(response);

    handleFormService(response);
  }

  async function getDefaultService() {
    const config = await getSystemConfigApi();
    if (!config) return defaultService;

    return {
      ...defaultService,
      storage_providers: {
        minio: {
          default: config.minio_provider,
        },
      },
    } as Service;
  }

  async function handleFormService(services: Service[]) {
    if (!serviceId || serviceId === "create") {
      const defaultService = await getDefaultService();
      setFormService(defaultService);
      return;
    }

    if (!services) return;
    const selectedService = services.find((s) => s.name === serviceId);
    if (!selectedService) return;
    setFormService(selectedService);
  }

  useEffect(() => {
    handleGetServices();
  }, []);

  useUpdate(() => {
    handleFormService(services);
  }, [serviceId]);

  return (
    <ServicesContext.Provider
      value={{
        formMode,
        servocesAreLoading,
        filter,
        setFilter,
        services,
        setServices,
        formTab,
        setFormTab,
        formService,
        setFormService,
        showFDLModal,
        setShowFDLModal,
        refreshServices: handleGetServices,
        formFunctions: {
          handleChange,
          onBlur,
          errors,
          setErrors,
        },
      }}
    >
      {children}
    </ServicesContext.Provider>
  );
};

function useServicesContext() {
  return useContext(ServicesContext);
}

export default useServicesContext;
