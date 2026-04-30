import axios from "axios";

export async function deleteLogApi(serviceName: string, logName: string) {
  const response = await axios.delete(`/system/logs/${serviceName}/${logName}`);

  return response.data;
}
