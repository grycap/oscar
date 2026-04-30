import { Service } from "@/pages/ui/services/models/service";
import axios from "axios";

async function updateServiceApi(service: Service) {
  const response = await axios.put("/system/services", service);

  return response.data;
}

export default updateServiceApi;
