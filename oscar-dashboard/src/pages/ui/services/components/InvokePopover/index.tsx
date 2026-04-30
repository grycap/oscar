"use client";

import { useState, useRef, useEffect } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import Editor from "@monaco-editor/react";
import {
  Upload,
  FileText,
  Image as ImageIcon,
  ArrowLeft,
  Trash2,
  ArrowRight,
  Terminal,
} from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import useServicesContext from "../../context/ServicesContext";
import { Service } from "../../models/service";
import { alert } from "@/lib/alert";
import OscarColors from "@/styles";
import { useAuth } from "@/contexts/AuthContext";
import RequestButton from "@/components/RequestButton";
import invokeServiceSync from "@/api/invoke/invokeServiceSync";

type View = "upload" | "editor" | "response";

interface Props {
  service?: Service;
  triggerRenderer?: React.ReactNode;
}

export function InvokePopover({ service, triggerRenderer }: Props) {
  const { formService } = useServicesContext();
  const { authData } = useAuth();
  const currentService = service ?? formService;

  const [isOpen, setIsOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [fileContent, setFileContent] = useState<string>("");
  const [fileType, setFileType] = useState<"text" | "image" | null>(null);
  const [currentView, setCurrentView] = useState<View>("upload");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedLanguage, setSelectedLanguage] = useState<string>("yaml");
  const [response, setResponse] = useState<string>("");

  const [responseType, setResponseType] = useState<"text" | "image" | "file">(
    "text"
  );

  const handleFileUpload = (uploadedFile: File) => {
    setFile(uploadedFile);

    if (uploadedFile.type === "application/json") {
      setFileType("text");
      setSelectedLanguage("json");
    } else if (
      uploadedFile.type === "application/x-yaml" ||
      uploadedFile.type === "text/yaml" ||
      uploadedFile.name.endsWith(".yaml") ||
      uploadedFile.name.endsWith(".yml")||
      uploadedFile.name.endsWith(".npy") ||
      uploadedFile.name.endsWith(".gzip") ||
      uploadedFile.name.endsWith(".tar") ||
      uploadedFile.name.endsWith(".rar") ||
      uploadedFile.name.endsWith(".7z")
    ) {
      setFileType("text");
      setSelectedLanguage("yaml");
    } else if (uploadedFile.type.startsWith("image/")) {
      setFileType("image");
    } else {
      alert.error("Type file not supported");
      return;
    }

    const reader = new FileReader();
    reader.onload = (e) => {
      setFileContent(e.target?.result as string);
    };
    reader.readAsText(uploadedFile);
  };

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
  };

  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    const droppedFile = event.dataTransfer.files[0];
    if (droppedFile) {
      handleFileUpload(droppedFile);
    }
  };

  const handleEditorChange = (value: string | undefined) => {
    if (value !== undefined) {
      setFileContent(value);
    }
  };

  const invokeService = async () => {
    const modifiedFile = new File([fileType === "image" ? file! : fileContent], file?.name ?? "file.txt", {
      type: file?.type ?? "text/plain",
    });
    try {
      const token = authData.token ?? currentService?.token;
      const response = await invokeServiceSync({
        file: modifiedFile,
        serviceName: currentService?.name,
        token,
        endpoint: authData.endpoint,
      });

      console.log("Invoke response", response);
      setResponse(response as string);
      setCurrentView("response");
    } catch (error) {
      alert.error("Error invoking service");
    }
  };

  const removeFile = () => {
    setFile(null);
    setFileType(null);
    setFileContent("");
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + " bytes";
    else if (bytes < 1048576) return (bytes / 1024).toFixed(1) + " KB";
    else return (bytes / 1048576).toFixed(1) + " MB";
  };

  const renderUploadView = () => (
    <div className="grid grid-cols-1 w-full">
      {!file ? (
        <>
          <input
            type="file"
            ref={fileInputRef}
            onChange={(e) =>
              e.target.files && handleFileUpload(e.target.files[0])
            }
            className="hidden"
            accept="image/*,.json,.yaml,.yml"
          />
          <div className="grid grid-cols-1 grid-rows-[1fr_auto] gap-2">
            <div className="h-full my-auto border-2 border-dashed cursor-pointer border-gray-300 rounded-lg p-8 text-center flex flex-col items-center justify-center gap-4"
              onDragOver={handleDragOver}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
            >
              <Upload className="h-8 w-8" />
              Drag and drop your file here or click to open file explorer
              <Button>Upload file</Button>
            </div>
            <div className="flex justify-center items-center">
              <Button
                variant="outline"
                onClick={() => {
                  setCurrentView("editor");
                  setFile(null);
                  setFileType(null);
                }}
              >
                Or use code editor
                <ArrowRight className="h-4 w-4 ml-2" />
              </Button>
            </div>
          </div>
        </>
      ) : (
        <div className="grid grid-cols-1 grid-rows-[auto_1fr] bg-muted rounded-lg w-full h-full ">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center space-x-2">
              {fileType === "image" ? (
                <ImageIcon className="h-5 w-5" />
              ) : (
                <FileText className="h-5 w-5" />
              )}
              <span className="font-medium">{file.name}</span>
              <span className="text-sm text-muted-foreground">
                ({formatFileSize(file.size)})
              </span>
            </div>
            {fileType === "text" && (
              <Button
                variant="outline"
                onClick={() => setCurrentView("editor")}
              >
                Edit in code editor
                <ArrowRight className="h-4 w-4 ml-2" />
              </Button>
            )}
            <Button variant="destructive" size="sm" onClick={removeFile}>
              <Trash2 className="h-4 w-4 mr-2" /> Remove
            </Button>
          </div>
          {fileType === "image" && (
            <div className="grid grid-cols-1 grid-rows-[auto_1fr] mt-4">
              <h4 className="text-md font-semibold mb-2">Preview</h4>
              <img
                src={URL.createObjectURL(file)}
                alt="Uploaded file"
                className="max-w-full h-auto max-h-[200px] rounded"
              />
            </div>
          )}
        </div>
      )}
    </div>
  );

  if (response) {
    console.log("Response:", response);
  }

  const [responseFileContent, setResponseFileContent] = useState<string>("");
  useEffect(() => {
    if (responseType === "file") {
      const base64 = response.split("\n")[0];
      const decodedContent = atob(base64);
      setResponseFileContent(decodedContent);
    }
  }, [responseType]);

  const renderEditorView = () => (
    <div className="grid grid-cols-1 grid-rows-[auto_1fr] w-full gap-2">
      <div className="flex justify-between items-start gap-4">
        <Button
          variant="outline"
          onClick={() => setCurrentView("upload")}
          className="mb-4"
        >
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Upload
        </Button>
        <Select value={selectedLanguage} onValueChange={setSelectedLanguage}>
          <SelectTrigger>
            <SelectValue placeholder="Select a language" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="yaml">YAML</SelectItem>
            <SelectItem value="json">JSON</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <Editor
        height="100%"
        width="100%"
        language={selectedLanguage}
        value={fileContent}
        onChange={handleEditorChange}
        options={{ minimap: { enabled: false } }}
      />
    </div>
  );

  const renderResponseView = () => {
    return (
      <div className="grid grid-cols-1 grid-rows-[auto_1fr] w-full gap-2">
        <Select
          value={responseType}
          onValueChange={(value) =>
            setResponseType(value as "text" | "image" | "file")
          }
        >
          <SelectTrigger className="w-[100px]">
            <SelectValue placeholder="Select a response type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="text">Text</SelectItem>
            <SelectItem value="image">Image</SelectItem>
            <SelectItem value="file">File</SelectItem>
          </SelectContent>
        </Select>
        <div className="h-full">
          {responseType === "text" && (
            <div
              style={{
                overflow: "auto",
                padding: "0px 10px",
                whiteSpace: "pre-wrap",
                wordWrap: "break-word",
                overflowWrap: "break-word",
                maxHeight: "75vh",
              }}
            >
              {response}
            </div>
          )}
          {responseType === "image" && (
            <img
              src={`data:image/png;base64,${response.split("\n")[0]}`}
              alt="Response"
            />
          )}
          {responseType === "file" && (
            <div
              style={{
                padding: "0px 10px",
                whiteSpace: "pre-wrap",
                wordWrap: "break-word",
                overflowWrap: "break-word",
              }}
            >
              {responseFileContent}
            </div>
          )}
        </div>
      </div>
    );
  };

  const resetForm = () => {
    setFile(null);
    setFileContent("");
    setFileType(null);
    setCurrentView("upload");
    setSelectedLanguage("yaml");
    setResponse("");
    setResponseType("text");
  };

  useEffect(() => {
    if (!isOpen) {
      resetForm();
    }
  }, [isOpen]);

  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger asChild>
        {triggerRenderer ?? (
          <Button variant="outline">
            <Terminal className="h-4 w-4 mr-2" />
            Invoke
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="grid grid-cols-1 grid-rows-[auto_1fr_auto] w-screen sm:w-[70%] 2xl:w-[60%] h-[90%] sm:h-[80%] 2xl:h-[60%] overflow-y-auto gap-5">
        <DialogHeader>
          <DialogTitle>
            <span style={{ color: OscarColors.DarkGrayText }}>
              {`Invoke service: `}
            </span>
            {currentService?.name}
          </DialogTitle>
        </DialogHeader>
        {currentView === "upload" && renderUploadView()}
        {currentView === "editor" && renderEditorView()}
        {currentView === "response" && renderResponseView()}
        <div className="grid grid-cols-1 sm:grid-cols-[auto] sm:justify-end">
          {currentView !== "response" ? (
            <RequestButton variant="mainGreen" request={invokeService}>
              Invoke Service
            </RequestButton>
          ) : (
            <Button variant="outline" onClick={() => resetForm()}>
              <ArrowLeft className="h-4 w-4 mr-2" /> Go back
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
