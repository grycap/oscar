import axios from "axios";
import { AuthData } from "@/contexts/AuthContext";

async function junoServiceRunning({
  endpoint,
  user,
  token,
  password,
}: AuthData,juno_name: string) {
  let config={}
  if ( token === undefined) {
    config = {
      baseURL: endpoint,
      auth: {
        user,
        password,
      },
    }
  }else{
    config = {
      baseURL: endpoint,
      headers: { Authorization: "Bearer "+token,
                "Access-Control-Allow-Origin": "*"},
      auth: undefined,
    }
  }
  const url= "/system/services/" + juno_name + "/exposed/lab"
  const response = await axios.get(url,config);
  console.log(response.status)
  if(response.status === 200) return true
  else return false
}

export default junoServiceRunning;
