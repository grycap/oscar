import axios from "axios";

async function getServiceApi(serviceName:string) {
  const response = await axios.get("/system/services/"+serviceName);

  return response;
}

export default getServiceApi;
