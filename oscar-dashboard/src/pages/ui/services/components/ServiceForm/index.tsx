import ServiceGeneralTab from "./components/GeneralTab";
import useServicesContext from "../../context/ServicesContext";

function ServiceForm() {
  const { formService } = useServicesContext();

  //const [formTab, setFormTab] = useState(ServiceFormTab.General);

  if (Object.keys(formService).length === 0) return null;

  return (
    <>
      {/*<ServiceFormTabs tab={formTab} setTab={setFormTab} />*/}
      <div
        style={{
          display: "flex",
          flexGrow: 1,
          flexBasis: 0,
          overflow: "auto",
        }}
      >
        <ServiceGeneralTab />
        {/*formTab === ServiceFormTab.Storage && <ServicesStorageTab />*/}
      </div>
    </>
  );
}

export default ServiceForm;
