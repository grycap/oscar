import RequestButton from "@/components/RequestButton";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMinio } from "@/contexts/Minio/MinioContext";
import { alert } from "@/lib/alert";
import { Plus } from "lucide-react";

import { Check, ExternalLink } from "lucide-react";
import { useEffect, useState } from "react";
import yamlToServices from "../services/components/FDL/utils/yamlToService";
import { useAuth } from "@/contexts/AuthContext";
import { Service } from "../services/models/service";
import createServiceApi from "@/api/services/createServiceApi";
import useServicesContext from "../services/context/ServicesContext";
import { Link } from "react-router-dom";
import deleteServiceApi from "@/api/services/deleteServiceApi";

function JunoView() {
  
  const { buckets } = useMinio();
  const { systemConfig } = useAuth();
  const { authData } = useAuth();
  const [ kindInputBucket, setkindInputBucket ] = useState(false);

  
  const namePrefix =
    authData?.egiSession?.sub ?? authData?.token ?? authData?.user;
  const namePrefixSlice = namePrefix?.slice(0, 6);

  useEffect(() => {
      document.title ="OSCAR - Notebooks"
    });

  const oidcGroups = systemConfig?.config.oidc_groups ?? [];

  const { services, refreshServices } = useServicesContext();
  const initialVo = oidcGroups[0];

  const junoService = services.find(
    (service) => service.name === `juno${namePrefixSlice}`
  );

  const isDeployed = !!junoService;

  const [formData, setFormData] = useState({
    cpuCores: "1.0",
    memoryRam: "2",
    memoryUnit: "Gi",
    bucket: "",
    vo: initialVo ?? "",
  });

  const handleDeploy = async () => {
    if (
      !formData.cpuCores ||
      !formData.memoryRam ||
      !formData.bucket ||
      !formData.vo
    ) {
      alert.error("Please fill in all fields");
      return;
    }

    try {
      if (!namePrefix) throw Error("No name prefix found");

      const fdlUrl =
        "https://raw.githubusercontent.com/grycap/oscar-juno/refs/heads/main/juno.yaml";
      const fdlResponse = await fetch(fdlUrl);
      const fdlText = await fdlResponse.text();

      const scriptUrl =
        "https://raw.githubusercontent.com/grycap/oscar-juno/refs/heads/main/script.sh";
      const scriptResponse = await fetch(scriptUrl);
      const scriptText = await scriptResponse.text();

      const services = yamlToServices(fdlText, scriptText);
      if (!services.length) throw Error("No services found");

      const service = services[0];
      const modifiedService: Service = {
        ...service,
        name: `juno${namePrefixSlice}`,
        vo: formData.vo,
        memory: `${formData.memoryRam}${formData.memoryUnit}`,
        cpu: formData.cpuCores,
        mount: {
          ...service.mount,
          path: formData.bucket ?? "/notebook",
          storage_provider: service.mount?.storage_provider ?? "minio.default",
        },
        environment: {
          variables: {
            ...service.environment.variables,
            JHUB_BASE_URL: `/system/services/juno${namePrefixSlice}/exposed`,
            JUPYTER_DIRECTORY: "/mnt/"+ formData.bucket,
            GRANT_SUDO: "yes",
            OSCAR_ENDPOINT: authData.endpoint,
          },
          secrets:{
            ...service.environment.secrets,
            JUPYTER_TOKEN:	 authData?.token ?? "junooscar",
          }
        },
      };
      console.log(modifiedService)

      await createServiceApi(modifiedService);
      refreshServices();

      alert.success("Jupyter Notebook instance deployed");
    } catch (error) {
      console.log(error)
      alert.error("Error deploying Jupyter Notebook instance");
    }
  };

  useEffect(() => {
    if (oidcGroups.length) {
      setFormData({ ...formData, vo: oidcGroups[0] });
    }
  }, [oidcGroups.length]);

  async function handleDelete() {
    if (!junoService) return;
    try {
      await deleteServiceApi(junoService);
      refreshServices();
      alert.success("Jupyter Notebook instance deleted");
    } catch (error) {
      console.error(error);
      alert.error("Error deleting Jupyter Notebook instance");
    }
  }

  return (
    <div className="grid grid-cols-1 gap-6 w-[95%] sm:w-[90%] lg:w-[80%] mx-auto mt-[40px] min-w-[300px] max-w-[700px] content-start">
      <h1 className="text-center sm:text-left" style={{ fontSize: "24px", fontWeight: "500" }}>
        Jupyter Notebook
      </h1>
      <Card>
        <CardHeader>
          <CardTitle key={isDeployed.toString()}>
            {isDeployed ? "Deployment" : "Configuration"}
          </CardTitle>
        </CardHeader>

        <CardContent>
          {!isDeployed ? (
            <form className="grid grid-cols-1 gap-4">
              <div className="space-y-2">
                <Label htmlFor="cpu-cores">CPU Cores</Label>
                <Input
                  id="cpu-cores"
                  type="number"
                  step="0.1"
                  placeholder="Enter CPU cores"
                  value={formData.cpuCores}
                  onChange={(e) =>
                    setFormData({ ...formData, cpuCores: e.target.value })
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="memory-ram">Memory RAM</Label>
                <div className="flex space-x-2">
                  <Input
                    id="memory-ram"
                    type="number"
                    step={formData.memoryUnit === "Gi" ? 1 : 256}
                    placeholder="Enter memory RAM"
                    value={formData.memoryRam}
                    onChange={(e) =>
                      setFormData({ ...formData, memoryRam: e.target.value })
                    }
                    className="flex-grow"
                  />
                  <Select
                    value={formData.memoryUnit}
                    onValueChange={(value) =>
                      setFormData({ ...formData, memoryUnit: value })
                    }
                  >
                    <SelectTrigger className="w-[80px]">
                      <SelectValue id="memory-unit" placeholder="Unit" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="Gi">Gi</SelectItem>
                      <SelectItem value="Mi">Mi</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="bucket">Bucket</Label>
                {kindInputBucket? 
                <Input
                  id="bucket-value"
                  type="input"
                  onFocus={(e) => (e.target.type = "text")}
                  style={{ width: "100%",
                    fontWeight: "normal",
                   }}
                  onChange={(e) => {
                     setFormData({ ...formData, bucket: e.target?.value })
                  }}
                 
                  placeholder="Select a bucket"
                />
                :
                  <Select
                    value={formData.bucket}
                    onValueChange={(value) =>
                      setFormData({ ...formData, bucket: value })
                    }
                  >
                    <SelectTrigger id="bucket">
                      <SelectValue
                        id="bucket-value"
                        placeholder="Select a bucket"
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {buckets.map((bucket) => (
                        <SelectItem key={bucket.Name} value={bucket.Name!}>
                          {bucket.Name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                }
                <div className="flex justify-end space-x-2">
                <Button 
                        id="add-annotations-button"
                        size={"sm"}
                        style={{
                          width: "max-content",
                        }}
                        onClick={() => {
                            setkindInputBucket(!kindInputBucket);
                        }}
                      >
                  <Plus className="h-4 w-4 mr-2" /> {kindInputBucket?"Buckets created":"New Bucket"}
                </Button>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="vo">VO</Label>
                <Select
                  value={formData.vo}
                  onValueChange={(value) =>
                    setFormData({ ...formData, vo: value })
                  }
                >
                  <SelectTrigger id="vo">
                    <SelectValue placeholder="Select a VO" />
                  </SelectTrigger>
                  <SelectContent>
                    {oidcGroups.map((group) => (
                      <SelectItem key={group} value={group}>
                        {group}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </form>
          ) : (
            <Alert>
              <Check className="h-4 w-4" />
              <AlertTitle>Success</AlertTitle>
              <AlertDescription>
                Your Jupyter Notebook instance has been deployed.
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
        <CardFooter className="grid grid-cols-1">
          {!isDeployed ? (
            <RequestButton id="juno-deploy-button " request={handleDeploy} className="sm:justify-self-end">
              Deploy
            </RequestButton>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-[auto_auto] gap-2 justify-end">
              <Link
                key={`juno-${namePrefixSlice}`}
                to={`${
                  authData.endpoint
                }/system/services/juno${namePrefixSlice}/exposed/lab?token=${
                  authData.token ?? ""
                }`}
                target="_blank"
              >
                <Button id="juno-visit-button" className="w-full">
                  <ExternalLink className="w-5 h-5 mr-2" />
                  Visit
                </Button>
              </Link>
              <RequestButton
                id="juno-delete-button"
                variant={"destructive"}
                request={handleDelete}
              >
                Delete
              </RequestButton>
            </div>
          )}
        </CardFooter>
      </Card>
    </div>
  );
}

export default JunoView;
