import OscarColors from "@/styles";
import { RefreshCcwIcon } from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import useServicesContext from "../../../context/ServicesContext";

function ServiceBreadcrumb() {
  const { refreshServices } = useServicesContext();
  const location = useLocation();
  const pathnames = location.pathname.split("/").filter((x) => x && x !== "ui");
  const [_, serviceId] = pathnames;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "row",
        alignItems: "center",
        gap: 9,
      }}
    >
      <Link
        to="/ui/services"
        style={{
          color: OscarColors.DarkGrayText,
          fontSize: 18,
          textDecoration: "none",
        }}
      >{`Services`}</Link>
      
      {location.pathname === "/ui/services" &&
      <Link to=""
        onClick={() => refreshServices()}
      >
        <RefreshCcwIcon size={18} 
          onMouseEnter={(e) => {e.currentTarget.style.transform = 'rotate(90deg)'}}
          onMouseLeave={(e) => {e.currentTarget.style.transform = 'rotate(0deg)'}}
        />
      </Link>
      }
      

      {serviceId === "create" && (
        <>
          <span style={{ color: OscarColors.DarkGrayText, fontSize: 18 }}>
            {` > `}
          </span>
          <Link
            to="/ui/services/create"
            style={{
              color: "black",
              fontSize: 18,
              textDecoration: "none",
            }}
          >{`Creating service`}</Link>
        </>
      )}
    </div>
  );
}

export default ServiceBreadcrumb;
