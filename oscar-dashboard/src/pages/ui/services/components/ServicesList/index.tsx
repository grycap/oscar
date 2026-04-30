import { useMemo, useRef, useState } from "react";
import useServicesContext from "../../context/ServicesContext";
import getServicesApi from "@/api/services/getServicesApi";
import deleteServiceApi from "@/api/services/deleteServiceApi";
import { alert } from "@/lib/alert";
import DeleteDialog from "@/components/DeleteDialog";
import { Service } from "../../models/service";
import { Button } from "@/components/ui/button";
import { LoaderPinwheel, Pencil, Terminal, Trash2 } from "lucide-react";
import OscarColors from "@/styles";
import { Link, useNavigate } from "react-router-dom";
import GenericTable from "@/components/Table";
import { InvokePopover } from "../InvokePopover";
import { handleFilterServices } from "./domain/filterUtils";
import { useAuth } from "@/contexts/AuthContext";
import MoreActionsPopover from "./components/MoreActionsPopover";

function ServicesList() {
  const { services, servocesAreLoading,setServices, setFormService, filter } =
    useServicesContext();
  const { authData } = useAuth();
  const [servicesToDelete, setServicesToDelete] = useState<Service[]>([]);
  const navigate = useNavigate();
  const buttonRef = useRef<Map<String, HTMLButtonElement>>(new Map())

  async function handleGetServices() {
    try {
      const response = await getServicesApi();
      setServices(response);
    } catch (error) {
      alert.error("Error getting services");
      console.error(error);
    }
  }

  async function handleDeleteService() {
    if (servicesToDelete.length > 0) {
      const deletePromises = servicesToDelete.map((service) =>
        deleteServiceApi(service).then(
          () => ({ status: "fulfilled", service }),
          (error) => ({ status: "rejected", service, error })
        )
      );
      
      const results = await Promise.all(deletePromises);

      const succeededServices = results
        .filter((result) => result.status === "fulfilled")
        .map((result) => result.service);

      const failedServices = results
        .filter((result) => result.status === "rejected")
        .map((result) => (result as {
          status: string;
          service: Service;
          error: any;
        }).service);
      
      await handleGetServices();

      if (succeededServices.length > 0) {
        alert.success("Services deleted successfully");
      }

      if (failedServices.length > 0) {
        alert.error(
          `Error deleting the following services: ${failedServices
            .map((service) => service.name)
            .join(", ")}`
        );
      }

      if (succeededServices.length === 0 && failedServices.length > 1) {
        alert.error("Error deleting all services");
      }

      setServicesToDelete([]);
    }
  }

  const filteredServices = useMemo(() => {
    const filteredServices = handleFilterServices({
      filter,
      services,
      authData,
    });
    return filteredServices;
  }, [services, filter, authData?.user]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        flexGrow: 1,
        flexBasis: 0,
        overflow: "hidden",
      }}
    >
      {servocesAreLoading === true ?
        <div className="flex items-center justify-center h-screen">
            <LoaderPinwheel className="animate-spin" size={60} color={OscarColors.Green3} />
        </div>
      :
        <>
          <GenericTable<Service>
            data={filteredServices}
            idKey="name"
            onRowClick={(item) => {
              setFormService(item);
              navigate(`/ui/services/${item.name}/settings`);
            }}
            columns={[
              { header: "Name", accessor: "name", sortBy: "name" },
              { header: "Image", accessor: "image", sortBy: "image" },
              { header: "CPU", accessor: "cpu", sortBy: "cpu" },
              { header: "Memory", accessor: "memory", sortBy: "memory" },
            ]}
            actions={[
              {
                button: (item) => (
                  <MoreActionsPopover
                    service={item}
                    handleDeleteService={() => setServicesToDelete([item])}
                    handleEditService={() => {
                      setFormService(item);
                      navigate(`/ui/services/${item.name}/settings`);
                    }}
                    handleInvokeService={() => {
                      setFormService(item);
                      buttonRef.current?.get(item.name)?.click();
                    }}
                    handleLogs={() => {
                      setFormService(item);
                      navigate(`/ui/services/${item.name}/logs`);
                    }}
                  />
                ),
              },
              {
                button: (item) => (
                  <InvokePopover
                    service={item}
                    triggerRenderer={
                      <Button variant={"link"} ref={(elem) => {buttonRef.current?.set(item.name, elem!)}} size="icon" tooltipLabel="Invoke">
                        <Terminal />
                      </Button>
                    }
                  />
                ),
              },
              {
                button: (item) => (
                  <Link
                    to={`/ui/services/${item.name}/settings`}
                    replace
                    onClick={() => {
                      setFormService(item);
                    }}
                  >
                    <Button variant={"link"} size="icon" tooltipLabel="Edit">
                      <Pencil />
                    </Button>
                  </Link>
                ),
              },
              {
                button: (item) => (
                  <Button
                    variant={"link"}
                    size="icon"
                    onClick={() => setServicesToDelete([item])}
                    tooltipLabel="Delete"
                  >
                    <Trash2 color={OscarColors.Red} />
                  </Button>
                ),
              },
            ]}
            bulkActions={[
              {
                button: (items) => {
                  return (
                    <div>
                      <Button
                        variant={"destructive"}
                        style={{
                          display: "flex",
                          flexDirection: "row",
                          gap: 8,
                        }}
                        onClick={() => setServicesToDelete(items)}
                      >
                        <Trash2 className="h-5 w-5" />
                        Delete services
                      </Button>
                    </div>
                  );
                },
              },
            ]}
          />
          <DeleteDialog
            isOpen={servicesToDelete.length > 0}
            onClose={() => setServicesToDelete([])}
            onDelete={handleDeleteService}
            itemNames={servicesToDelete.map((service) => service.name)}
          />
        </>
      }
    </div>
  );
}

export default ServicesList;
