import { AuthData } from "@/contexts/AuthContext";
import axios from "axios";

export function setAxiosInterceptor(authData: AuthData) {
  const { endpoint, user: username, password, token } = authData;
  if (token === undefined) {
    axios.interceptors.request.use((config) => {
      config.baseURL = endpoint;
      config.auth = {
        username,
        password,
      };
      return config;
    });
  } else {
    axios.interceptors.request.use((config) => {
      config.baseURL = endpoint;
      config.headers.Authorization = "Bearer " + token;
      return config;
    });
  }
}
