import { Service } from "@/pages/ui/services/models/service";
import axios from "axios";

async function createServiceApi(service: Service) {
  const response = await axios.post("/system/services", service);

  return response.data;
}

export default createServiceApi;
