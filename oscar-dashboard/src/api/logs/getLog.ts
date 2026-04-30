import axios from "axios";

export async function getLogApi(serviceName: string, logName: string) {
  const response = await axios.get(`/system/logs/${serviceName}/${logName}`);

  return response.data as string;
}
