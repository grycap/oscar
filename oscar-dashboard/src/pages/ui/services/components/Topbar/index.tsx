import { OscarStyles } from "@/styles";
import ServiceBreadcrumb from "./components/Breadcrumbs";
import { useEffect } from "react";
import ServicesFilterBy from "./components/FilterBy";
import AddServiceButton from "./components/CreateServiceButton";
import CreateUpdateServiceTabs from "./components/CreateUpdateServiceTabs";
import UserInfo from "@/components/UserInfo";
import useServicesContext from "../../context/ServicesContext";

export enum ServiceViewMode {
  List = "List",
  Create = "Create",
  Update = "Update",
}

function ServicesTopbar() {
  const { formMode } = useServicesContext();

  useEffect(() => {
    document.title = "OSCAR - Services";
  }, []);

  return (
    <div
      style={{
        borderBottom: OscarStyles.border,
      }}
      className="grid grid-cols-[1fr_auto] h-[64px]"
    >
      <div
        style={{
          padding: "0 16px",
        }}
        className={"grid items-center justify-between gap-2 " + (formMode === ServiceViewMode.List ? "grid-cols-[auto_auto_auto]" : "grid-cols-[auto_1fr]")}
      >
        <ServiceBreadcrumb />

        {formMode === ServiceViewMode.List ? (
          <>
          <ServicesFilterBy />
          <AddServiceButton />
          </>
        ) : (
          <CreateUpdateServiceTabs mode={formMode} />
        )}
      </div>
      <UserInfo />
    </div>
  );
}

export default ServicesTopbar;
