import { MinioStorageProvider } from "@/pages/ui/services/models/service";
import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  S3Client,
  ListBucketsCommand,
  Bucket,
  ListObjectsV2Command,
  ListObjectsV2CommandInput,
  CommonPrefix,
  _Object,
  PutObjectCommand,
  DeleteObjectCommand,
  GetObjectCommand,
} from "@aws-sdk/client-s3";
import getSystemConfigApi from "@/api/config/getSystemConfig";
import { alert } from "@/lib/alert";
import JSZip from "jszip";
import env from "@/env";
import createBucketsApi from "@/api/buckets/createBucketsApi";
import deleteBucketsApi from "@/api/buckets/deleteBucketsApi";

export type MinioProviderData = {
  providerInfo: MinioStorageProvider;
  setProviderInfo: (providerInfo: MinioStorageProvider) => void;
  buckets: Bucket[];
  setBuckets: (buckets: Bucket[]) => void;
  createBucket: (bucketName: string) => Promise<void>;
  updateBuckets: () => Promise<void>;
  getBucketItems: (
    bucketName: string,
    path: string
  ) => Promise<{
    folders: CommonPrefix[];
    items: _Object[];
  }>;
  deleteBucket: (bucketName: string) => Promise<void>;
  createFolder: (bucketName: string, folderName: string) => Promise<void>;
  uploadFile: (bucketName: string, path: string, file: File) => Promise<void>;
  deleteFile: (bucketName: string, path: string) => Promise<void>;
  getFileUrl: (bucketName: string, path: string) => Promise<string | undefined>;
  listObjects: (bucketName: string, path: string) => Promise<_Object[]>;
  downloadAndZipFolders: (
    bucketName: string,
    folders: CommonPrefix[],
    singleFiles: _Object[]
  ) => Promise<Blob | undefined>;
};

function isLocalhostDeployed(endpoint:string){
  if (env.response_default_minio === endpoint){
    return true
  }else return false
}
export const MinioContext = createContext({} as MinioProviderData);

export const MinioProvider = ({ children }: { children: React.ReactNode }) => {
  const [providerInfo, setProviderInfo] = useState({} as MinioStorageProvider);
  const [buckets, setBuckets] = useState<Bucket[]>([]);

  const client = useMemo(() => {
    if (
      !providerInfo.endpoint ||
      !providerInfo.access_key ||
      !providerInfo.secret_key ||
      !providerInfo.region
    )
      return null;
    providerInfo.endpoint =  isLocalhostDeployed(providerInfo.endpoint) ? "http://"+env.minio_local_endpoint+":"+env.minio_local_port : providerInfo.endpoint;
    return new S3Client({
      region: providerInfo.region,
      endpoint: providerInfo.endpoint,
      credentials: {
        accessKeyId: providerInfo.access_key,
        secretAccessKey: providerInfo.secret_key,
      },
      forcePathStyle: true,
    });
  }, [providerInfo]);

  /**
   * Lista las carpetas e ítems en una ruta específica dentro de un bucket de S3.
   * @param bucketName Nombre del bucket de S3.
   * @param path Ruta dentro del bucket. Usa una cadena vacía para la raíz.
   * @returns Un objeto que contiene arrays de carpetas e ítems.
   */
  async function getBucketItems(
    bucketName: string,
    path: string = ""
  ): Promise<{
    folders: CommonPrefix[];
    items: _Object[];
  }> {
    if (!client) return { folders: [], items: [] };

    // Asegura que el prefijo termine con '/' si no está vacío
    const prefix = path ? (path.endsWith("/") ? path : `${path}/`) : "";

    const params: ListObjectsV2CommandInput = {
      Bucket: bucketName,
      Prefix: prefix,
      Delimiter: "/", // Usa '/' como delimitador para agrupar carpetas
    };

    try {
      const command = new ListObjectsV2Command(params);
      const response = await client.send(command);

      // Extrae las carpetas (CommonPrefixes)
      const folders = response.CommonPrefixes ?? [];

      // Extrae los ítems (Contents)
      const items =
        response.Contents?.filter((item) => item.Key !== prefix) ?? [];

      return { folders, items };
    } catch (error) {
      console.error("Error al listar objetos de S3:", error);
      throw error;
    }
  }

  async function updateBuckets() {
    if (!client) return;

    const res = await client.send(new ListBucketsCommand({}));
    const buckets = res?.Buckets;
    if (!buckets) return;

    setBuckets(buckets);
  }

  async function createBucket(bucketName: string) {
    if (!client) return;

    try {
      await createBucketsApi(bucketName,undefined)
      /*const command = new CreateBucketCommand({
        Bucket: bucketName,
      });
      await client.send(command);*/
      alert.success("Bucket created successfully");
    } catch (error) {
      console.error(error);
      alert.error("Error creating bucket");
    }

    updateBuckets();
  }

  async function deleteBucket(bucketName: string) {
    if (!client) return;

    try {
      await deleteBucketsApi(bucketName)
      /*const command = new DeleteBucketCommand({ Bucket: bucketName });
      await client.send(command);*/

      alert.success("Bucket deleted successfully");
    } catch (error) {
      console.error(error);
      alert.error("Error deleting bucket");
    }

    updateBuckets();
  }

  async function createFolder(bucketName: string, folderName: string) {
    if (!client) return;

    const folderKey = folderName.endsWith("/") ? folderName : `${folderName}/`;

    try {
      await client.send(
        new PutObjectCommand({
          Bucket: bucketName,
          Key: folderKey,
        })
      );
      alert.success("Folder created successfully");
    } catch (error) {
      console.error(error);
      alert.error("Error creating folder");
    }

    updateBuckets();
  }

  async function uploadFile(
    bucketName: string,
    path: string,
    file: File
  ): Promise<void> {
    const reader = new FileReader();
    const  fileContent = await new Promise<string | ArrayBuffer | null>((resolve, reject) => {
        reader.onload = () => resolve(reader.result);  // Resolver la promesa cuando se cargue el archivo
        reader.onerror = (error) => reject(error);     // Rechazar la promesa si ocurre un error
        reader.readAsArrayBuffer(file);  // Si el archivo es de texto, usa readAsText. Para binarios usa readAsArrayBuffer o readAsDataURL
      });
   
    if (!client) return;

    const key = path ? `${path}${file.name}` : file.name;
  
    try {
      const command = new PutObjectCommand({
        Bucket: bucketName,
        Key: key,
        // @ts-ignore
        Body: fileContent,
      });
      await client.send(command);
      alert.success("File uploaded successfully");
    } catch (error) {
      console.error(error);
      alert.error("Error uploading file");
    }

    updateBuckets();
  }

  async function deleteFile(bucketName: string, path: string) {
    if (!client) return;

    try {
      const command = new DeleteObjectCommand({
        Bucket: bucketName,
        Key: path,
      });
      await client.send(command);
      alert.success("File deleted successfully");
    } catch (error) {
      console.error(error);
      alert.error("Error deleting file");
    }

    updateBuckets();
  }

  async function getFileUrl(bucketName: string, path: string) {
    if (!client) return;

    const command = new GetObjectCommand({
      Bucket: bucketName,
      Key: path,
    });
    const response = await client.send(command);
    const byteArray = await response.Body?.transformToByteArray();
    if (!byteArray) {
      throw new Error("Failed to transform response body to byte array");
    }
    const url = URL.createObjectURL(new Blob([byteArray]));

    return url;
  }

  async function listObjects(bucketName: string, path: string = "") {
    if (!client) return [];

    let objects: _Object[] = [];
    let continuationToken: string | undefined = undefined;

    do {
      const params: ListObjectsV2CommandInput = {
        Bucket: bucketName,
        Prefix: path,
        ContinuationToken: continuationToken,
      };

      const response = await client.send(new ListObjectsV2Command(params));
      if (response.Contents) {
        objects = objects.concat(response.Contents);
      }
      continuationToken = response.NextContinuationToken;
    } while (continuationToken);

    return objects;
  }

  // Función para descargar un archivo como ArrayBuffer
  async function downloadFile(bucketName: string, key: string) {
    const params = { Bucket: bucketName, Key: key };
    const data = await client?.send(new GetObjectCommand(params));
    if (!data?.Body) return undefined;
    return await data.Body.transformToByteArray();
  }

  async function downloadAndZipFolders(
    bucketName: string,
    folders: CommonPrefix[],
    singleFiles: _Object[]
  ) {
    const zip = new JSZip();

    try {
      for (const folder of folders) {
        const objects = await listObjects(bucketName, folder.Prefix!);

        for (const object of objects) {
          const relativePath = object.Key!.replace(folder.Prefix!, "");
          if (!relativePath) continue; // Ignorar carpetas vacías

          const fileData = await downloadFile(bucketName, object.Key!);

          // Añadir archivo al ZIP
          if (fileData) {
            zip.file(`${folder.Prefix}${relativePath}`, new Blob([fileData]));
          } else {
            throw new Error(`Error al descargar el archivo: ${object.Key}`);
          }
        }
      }

      for (const file of singleFiles) {
        const fileData = await downloadFile(bucketName, file.Key!);
        if (fileData) {
          zip.file(file.Key!, new Blob([fileData]));
        } else {
          throw new Error(`Error al descargar el archivo: ${file.Key}`);
        }
      }

      // Generar el ZIP y descargarlo
      const zipBlob = await zip.generateAsync({ type: "blob" });
      return zipBlob;
    } catch (err) {
      alert.error(
        err instanceof Error ? err.message : "Error durante la descarga"
      );
    }
  }

  useEffect(() => {
    async function getProviderInfo() {
      const config = await getSystemConfigApi();
      if (!config) return;

      setProviderInfo(config.minio_provider);
    }

    getProviderInfo();
  }, []);

  useEffect(() => {
    updateBuckets();
  }, [client]);

  return (
    <MinioContext.Provider
      value={{
        providerInfo,
        setProviderInfo,
        buckets,
        setBuckets,
        createBucket,
        createFolder,
        updateBuckets,
        getBucketItems,
        deleteBucket,
        uploadFile,
        deleteFile,
        getFileUrl,
        listObjects,
        downloadAndZipFolders,
      }}
    >
      {children}
    </MinioContext.Provider>
  );
};

export const useMinio = () => useContext(MinioContext);
