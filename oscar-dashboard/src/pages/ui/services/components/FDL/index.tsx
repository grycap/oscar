import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import useServicesContext from "../../context/ServicesContext";
import {
  Dialog,
  DialogDescription,
  DialogTitle,
  DialogContent,
  DialogHeader,
  DialogFooter,
} from "@/components/ui/dialog";
import { useState, useEffect } from "react";
import Editor from "@monaco-editor/react";
import { Input } from "@/components/ui/input";
import createServiceApi from "@/api/services/createServiceApi";
import getServiceApi from "@/api/services/getServiceApi";
import updateServiceApi from "@/api/services/updateServiceApi";
import { alert } from "@/lib/alert";
import RequestButton from "@/components/RequestButton";
import yamlToServices from "./utils/yamlToService";

function FDLForm() {
  const { showFDLModal, setShowFDLModal, refreshServices } =
    useServicesContext();
  const [selectedTab, setSelectedTab] = useState<"fdl" | "script">("fdl");
  const [editorKey, setEditorKey] = useState(0);

  const [fdl, setFdl] = useState("");
  const [script, setScript] = useState("");

  function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (e) => {
      const result = e.target?.result as string;
      if (selectedTab === "fdl") {
        setFdl(result);
      } else {
        setScript(result);
      }
    };
    reader.readAsText(file);
  }

  async function handleSave() {
    if (!fdl) {
      alert.error("Please fill the FDL file");
      return;
    }

    if (!script) {
      alert.error("Please fill the script");
      return;
    }

    const services = yamlToServices(fdl, script);

    const promises = services.map(async (service) => {
      try{
        await getServiceApi(service.name);
      }catch (error) {
        const response = await createServiceApi(service);
        return response;
      }
        const response = await updateServiceApi(service);
        return response;
    });

    const results = await Promise.allSettled(promises);

    results.forEach((result, index) => {
      if (result.status === "rejected") {
        alert.error(
          `Error creating service ${services[index].name}: ${result.reason.response.data}`
        );
      } else {
        alert.success(`Service ${services[index].name} created successfully`);
      }
    });

    if (results.every((result) => result.status === "fulfilled")) {
      setShowFDLModal(false);
      setFdl("");
      setScript("");
      setSelectedTab("fdl");
      refreshServices();
    }
  }

  useEffect(() => {
    const handleResize = () => {
      setEditorKey((prevKey) => prevKey + 1);
    };

    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
    };
  }, []);

  useEffect(() => {
    if (!showFDLModal) {
      setFdl("");
      setScript("");
      setSelectedTab("fdl");
    }
  }, [showFDLModal]);

  return (
    <Dialog open={showFDLModal} onOpenChange={setShowFDLModal}>
      {/* <DialogTrigger>Open</DialogTrigger> */}
      <DialogContent className="grid grid-cols-1 grid-rows-[auto_1fr_auto] w-screen sm:w-[70%] 2xl:w-[60%] h-[90%] sm:h-[80%] 2xl:h-[60%] overflow-y-auto gap-4">
        <DialogHeader>
          <DialogTitle>Create the service using FDL</DialogTitle>
          <DialogDescription>
            Use the code editor to edit the FDL file and the script.
          </DialogDescription>
        </DialogHeader>
        <Tabs className="grid grid-cols-1 grid-rows-[auto_1fr]"
          defaultValue="account"
          value={selectedTab}
          onValueChange={(value) => {
            setSelectedTab(value as "fdl" | "script");
          }}
        >
          <div className="grid grid-cols-1 sm:grid-cols-[auto_1fr] gap-2 justify-items-start">
            <TabsList>
              <TabsTrigger style={{ padding: "7px 30px" }} value="fdl">
                FDL
              </TabsTrigger>
              <TabsTrigger style={{ padding: "7px 30px" }} value="script">
                Script
              </TabsTrigger>
            </TabsList>

            <Input key={selectedTab} type="file" onChange={handleFileUpload} />
          </div>
          <TabsContent value="fdl" style={{ outline: "none", width: "100%" }}>
            <Editor
              key={`fdl-${editorKey}`}
              language="yaml"
              value={fdl}
              onChange={(e) => {
                setFdl(e || "");
              }}
              width="100%"
              height="100%"
              options={{
                minimap: {
                  enabled: false,
                },
              }}
            />
          </TabsContent>
          <TabsContent
            value="script"
            style={{ outline: "none", width: "100%" }}
          >
            <Editor 
              key={`script-${editorKey}`}
              language="javascript"
              value={script}
              onChange={(e) => {
                setScript(e || "");
              }}
              width="100%"
              height="100%"
              options={{
                minimap: {
                  enabled: false,
                },
              }}
            />
          </TabsContent>
        </Tabs>
        <DialogFooter>
          <RequestButton request={handleSave}>Create Service</RequestButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default FDLForm;
