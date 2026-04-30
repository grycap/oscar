import axios from "axios";

async function getSystemConfigApi() {
  const response = await axios.get("/system/config");

  return response.data;
}

export default getSystemConfigApi;
