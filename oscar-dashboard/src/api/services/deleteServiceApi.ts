import { Service } from "@/pages/ui/services/models/service";
import axios from "axios";

async function deleteServiceApi(service: Service) {
  const response = await axios.delete("/system/services/" + service.name);

  return response.data;
}

export default deleteServiceApi;
