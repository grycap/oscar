import Log from "@/pages/ui/services/models/log";
import axios from "axios";

export async function getServiceLogsApi(serviceName: string) {
  const response = await axios.get(`/system/logs/${serviceName}`);

  return response.data as Record<string, Log>;
}
