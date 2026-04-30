import axios from "axios";

type GetInfoProps = {
  endpoint: string;
  username: string;
  password: string;
  token: string | undefined;
};

export async function getInfoApi({
  endpoint,
  username,
  token,
  password,
}: GetInfoProps) {
  let config={}
  if ( token === undefined) {
    config = {
      baseURL: endpoint,
      auth: {
        username,
        password,
      },
    }
  }else{
    config = {
      baseURL: endpoint,
      headers: { Authorization: "Bearer "+token},
      auth: undefined,
    }
  }

  const response = await axios.get("/system/info", config);

  return response.data;
}


