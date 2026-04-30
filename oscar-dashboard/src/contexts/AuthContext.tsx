import getSystemConfigApi from "@/api/config/getSystemConfig";
import { getInfoApi } from "@/api/info/getInfoApi";
import { setAxiosInterceptor } from "@/lib/axiosClient";
import { ClusterInfo } from "@/models/clusterInfo";
import { SystemConfig } from "@/models/systemConfig";
import { MinioStorageProvider } from "@/pages/ui/services/models/service";
import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

type EGISessionInfo = {
  eduperson_assurance: string[]; // Lista de URLs para los niveles de garantía
  eduperson_entitlement: string[]; // Lista de valores de derechos
  email: string; // Correo electrónico del usuario
  email_verified: boolean; // Indica si el correo está verificado
  family_name: string; // Apellido del usuario
  given_name: string; // Nombre del usuario
  name: string; // Nombre completo del usuario
  preferred_username: string; // Nombre de usuario preferido
  sub: string; // Identificador único del usuario
  voperson_verified_email: string[]; // Lista de correos electrónicos verificados
};

export type AuthData = {
  user: string;
  password: string;
  endpoint: string;
  token?: string;
  authenticated?: boolean;
  egiSession?: EGISessionInfo;
};

export const AuthContext = createContext({
  authData: {
    user: "",
    password: "",
    endpoint: "",
    authenticated: false,
  } as AuthData,
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  setAuthData: (_: AuthData) => {},
  systemConfig: null as {
    config: SystemConfig;
    minio_provider: MinioStorageProvider;
  } | null,
  clusterInfo: null as ClusterInfo | null,
});

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const initialData = useMemo(() => {
    const storedData = localStorage.getItem("authData");
    if (!storedData) {
      return {
        user: "",
        password: "",
        endpoint: "",
        token: undefined,
        authenticated: false,
      } as AuthData;
    }

    const parsedData = JSON.parse(storedData) as AuthData;

    setAxiosInterceptor(parsedData);

    return parsedData;
  }, []);

  const [authData, setAuthDataState] = useState(initialData);
  const [systemConfig, setSystemConfig] = useState<{
    config: SystemConfig;
    minio_provider: MinioStorageProvider;
  } | null>(null);
  const [clusterInfo, setClusterInfo] = useState<ClusterInfo | null>(null);

  async function handleGetSystemConfig() {
    if (!authData.authenticated) return;

    const response = await getSystemConfigApi();
    setSystemConfig(response);
  }

  useEffect(() => {
    handleGetSystemConfig();
  }, [authData]);

  function setAuthData(data: AuthData) {
    if (data.authenticated) {
      localStorage.setItem("authData", JSON.stringify(data));
    } else {
      localStorage.removeItem("authData");
    }

    setAxiosInterceptor(data);

    setAuthDataState(data);
  }

  async function checkAuth() {
    if (!authData.authenticated) return;
    try {
      setClusterInfo(await getInfoApi({
        endpoint: authData.endpoint,
        username: authData.user,
        password: authData.password,
        token: authData?.token,
      }));
    } catch (error) {
      setAuthData({
        user: "",
        password: "",
        endpoint: "",
        authenticated: false,
        egiSession: undefined,
      });
    }
  }

  useEffect(() => {
    checkAuth();
  }, [initialData]);

  return (
    <AuthContext.Provider
      value={{
        authData,
        setAuthData,
        systemConfig,
        clusterInfo,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);
