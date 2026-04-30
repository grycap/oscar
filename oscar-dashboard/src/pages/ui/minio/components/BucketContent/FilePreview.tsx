import React, { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { BucketItem } from ".";
import { useMinio } from "@/contexts/Minio/MinioContext";
import Editor from "@monaco-editor/react";
import { fileTypeFromBlob } from "file-type";
import fileExtensionToLanguage from "./fileExtensionToLanguage.json";
import { Button } from "@/components/ui/button";
import { DownloadIcon } from "lucide-react";

interface FilePreviewModalProps {
  isOpen: boolean;
  onClose: () => void;
  file: BucketItem;
}

const FilePreviewModal: React.FC<FilePreviewModalProps> = ({
  isOpen,
  onClose,
  file: bucketItem,
}) => {
  const { getFileUrl } = useMinio();

  const [url, setUrl] = useState<string>();
  const [fileContent, setFileContent] = useState<string>();
  const [fileType, setFileType] = useState<"image" | "text" | "other">();
  const isText = fileType === "text";
  const isImage = fileType === "image";

  function handleDownload() {
    if (!url) return;
    const a = document.createElement("a");
    a.href = url;
    a.download = bucketItem.Name;
    a.click();
  }

  useEffect(() => {
    if (bucketItem.Type === "file") {
      getFileUrl(bucketItem.BucketName, bucketItem.Key.Key!).then((url) => {
        setUrl(url);
      });
    }
  }, [bucketItem.Name]);

  async function getFileType() {
    if (!url) return;

    const response = await fetch(url);
    const blob = await response.blob();
    const fileType = await fileTypeFromBlob(blob);
    console.log(fileType);
    setFileType(fileType?.mime?.startsWith("image/") ? "image" : "text");
  }

  useEffect(() => {
    async function fetchFileData() {
      if (!url) return;

      await getFileType();
      const response = await fetch(url);
      const text = await response.text();
      setFileContent(text);
    }
    fetchFileData();
  }, [url]);

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent
        style={{
          maxWidth: "80vw",
          width: "80vw",
        }}
      >
        <DialogHeader>
          <DialogTitle>{bucketItem.Name}</DialogTitle>
        </DialogHeader>
        <div style={{ height: "70vh", overflow: "hidden" }}>
          {isText && (
            <Editor
              width="100%"
              height="100%"
              language={
                fileExtensionToLanguage[
                  bucketItem.Name.split(
                    "."
                  ).pop()! as keyof typeof fileExtensionToLanguage
                ]
              }
              value={fileContent}
              options={{ readOnly: true }}
            />
          )}
          {isImage && url && (
            <div
              style={{
                height: "100%",
                width: "100%",
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
              }}
            >
              <img
                src={url}
                alt={bucketItem.Name}
                style={{ height: "100%", width: "auto" }}
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleDownload}>
            <DownloadIcon className="w-4 h-4 mr-2" />
            Download
          </Button>
          <Button onClick={onClose}>Close</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default FilePreviewModal;
