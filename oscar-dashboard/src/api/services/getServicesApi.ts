import { Service } from "@/pages/ui/services/models/service";
import axios from "axios";

async function getServicesApi() {
  const response = await axios.get("/system/services");

  return response.data as Service[];
}

export default getServicesApi;
