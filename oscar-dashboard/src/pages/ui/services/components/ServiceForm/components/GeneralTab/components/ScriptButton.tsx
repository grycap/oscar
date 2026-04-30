import useServicesContext from "@/pages/ui/services/context/ServicesContext";
import { useState } from "react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";

function ScriptButton() {
  const { setFormService } = useServicesContext();

  const [uploadMethod, setUploadMethod] = useState<"file" | "url">("file");

  //@ts-ignore
  const [fileContent, setFileContent] = useState<string>("");
  const [url, setUrl] = useState<string>("");

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onload = (e) => {
        const content = e.target?.result as string;
        setFileContent(content);
        setFormService((prevService) => ({
          ...prevService,
          script: content,
        }));
      };
      reader.readAsText(file);
    }
  };

  const handleUrlChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const newUrl = event.target.value;
    setUrl(newUrl);
    setFormService((prevService) => ({
      ...prevService,
      script: newUrl,
    }));
  };

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "row",
        gap: "10px",
        alignItems: "end",
      }}
    >
      <div>
        <Label>Script upload method*</Label>
        <Select
          value={uploadMethod}
          onValueChange={(value) => setUploadMethod(value as "file" | "url")}
        >
          <SelectTrigger
            id="script-upload-method-select-trigger"
            className="w-[150px]"
          >
            <SelectValue placeholder="Select upload method" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="file">File Upload</SelectItem>
            <SelectItem value="url">URL</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {uploadMethod === "file" ? (
        <div>
          <Input
            id="script-file-input"
            type="file"
            onChange={handleFileUpload}
            className="bg-white"
          />
        </div>
      ) : (
        <Input
          id="script-url-input"
          type="url"
          placeholder="Enter script URL"
          value={url}
          onChange={handleUrlChange}
          width={"275px"}
        />
      )}
    </div>
  );
}

export default ScriptButton;
