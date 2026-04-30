import { AuthData } from "@/contexts/AuthContext";
import {
  Service,
  ServiceFilter,
  ServiceFilterByKey,
} from "../../../models/service";

interface Props {
  services: Service[];
  filter: ServiceFilter;
  authData: AuthData;
}

function handleFilterServices({ services, filter, authData }: Props) {
  return services.filter((service) => {
    if (filter.onlyOwned) {
      const egiUserId = authData.egiSession?.sub;

      if (!egiUserId) {
        return (
          service.allowed_users.includes(authData.user) ||
          service.owner === authData.user
        );
      }

      if (!service.allowed_users.includes(egiUserId)) {
        return false;
      }
    }

    const param = service[ServiceFilterByKey[filter.type]] as string;

    return param.toLowerCase().includes(filter.value.toLowerCase());
  });
}

export { handleFilterServices };
