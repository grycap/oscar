import axios, { AxiosInstance } from "axios";

interface InvokeServiceSyncProps {
  serviceName: string;
  file: File;
  token: string;
  endpoint: string;
}

function readFile(file: File): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();

    reader.onload = (e) => {
      if (e.target?.result) {
        const base64Data = e.target.result as string;
        resolve(base64Data);
      }
    };

    reader.onerror = () => {
      reject(new Error("Error reading file."));
    };

    reader.readAsBinaryString(file);
  });
}

export default async function invokeServiceSync({
  serviceName,
  file,
  token,
  endpoint,
}: InvokeServiceSyncProps) {
  const axiosInstance: AxiosInstance = axios.create();

  try {
    const fileData: string = await readFile(file);

    const base64 = btoa(fileData);

    const response = await axiosInstance({
      method: "post",
      url: endpoint + "/run/" + serviceName,
      headers: { Authorization: `Bearer ${token}` },
      data: base64,
    });

    return response.data;
  } catch (error) {
    console.error("Error invoking service:", error);

    throw new Error("Error invoking service");
  }
}
