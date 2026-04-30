import axios from "axios";

export default async function deleteServiceLogsApi(serviceName: string) {
  const response = await axios.delete(`/system/logs/${serviceName}`);

  return response.data;
}
