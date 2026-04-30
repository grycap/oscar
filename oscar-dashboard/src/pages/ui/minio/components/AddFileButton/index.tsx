import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Label } from "@/components/ui/label";
import { useMinio } from "@/contexts/Minio/MinioContext";
import { Upload } from "lucide-react";
import useSelectedBucket from "../../hooks/useSelectedBucket";
import { Input } from "@/components/ui/input";
import { useMediaQuery } from "react-responsive";

export default function AddFileButton() {

  const { uploadFile } = useMinio();
  const { name: bucketName, path } = useSelectedBucket();
  const [file, setFile] = useState<File>();
  const [isImage, setIsImage] = useState<boolean>(false);
  const [isOpen, setIsOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const isSmallScreen = useMediaQuery({ maxWidth: 799 });

  const handleUploadFile = async () => {  
    await uploadFile(bucketName!, path, file!);
    setFile(undefined);
    setIsOpen(false);
  };
  
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setIsOpen(false);
      }

      if (e.key === "Enter") {
        handleUploadFile();
      }
    }
    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [handleUploadFile]);

  const setFileAndIsImage = (file: File) => {
    setIsImage(file!.type.startsWith("image/"))
    setFile(file)
  }

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
  };

  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    const droppedFile = event.dataTransfer.files[0];
    if (droppedFile) {
      setFileAndIsImage(droppedFile);
    }
  };
  
  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + " bytes";
    else if (bytes < 1048576) return (bytes / 1024).toFixed(1) + " KB";
    else return (bytes / 1048576).toFixed(1) + " MB";
  };

  const renderUploadView = () => (
    <div className="grid gap-2 w-[100%]">
      <Label htmlFor="file">{!file || !isImage ? 'Select file' : 'Preview'}</Label>
      <Input
        id="file"
        type="file"
        ref={fileInputRef}
        onChange={(e) =>
          e.target.files && setFileAndIsImage(e.target.files[0])
        }
        className={!file || isImage ? "hidden" : ""}
        accept="image/*,.*"
      />
      {!file ? (
        <div
          onDragOver={handleDragOver}
          onDrop={handleDrop}
          onClick={() => fileInputRef.current?.click()}
          className="border-2 border-dashed cursor-pointer border-gray-300 rounded-lg p-4 text-center flex flex-col items-center justify-center gap-2"
        >
          <Upload className="h-8 w-8" />
          Drag and drop your file here or click to open file explorer
          <Button>Select file</Button>
        </div>
      ) : (
        isImage ?
        <div className="bg-muted rounded-lg w-[100%]">
          <div className="mb-4">
            <div 
              onDragOver={handleDragOver}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              className="border-2 border-dashed cursor-pointer border-gray-300 rounded-lg text-center flex flex-col items-center justify-center"
            >
              <img
                src={URL.createObjectURL(file)}
                alt="Uploaded file"
                className="max-w-full h-auto max-h-[200px] rounded"
              />
            </div>
          </div>
          <div className="flex items-center justify-between mt-4">
            <div className="flex items-center space-x-2">
              <span className="flex font-medium">{file.name}</span>
              <span className="text-sm text-muted-foreground">
                ({formatFileSize(file.size)})
              </span>
            </div>
          </div>
        </div>
        : ''
      )}
    </div>
  );

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger asChild>
        <Button variant="mainGreen" style={{gap: 8}}>
          <Upload size={20} className="h-5 w-5" />
          {!isSmallScreen && "Upload File"}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80">
        <div className="grid gap-4">
          <div className="space-y-2">
            <h4 className="font-medium leading-none">Upload File</h4>
          </div>
          {renderUploadView()}
          <div className="flex justify-end space-x-2">
            <Button variant="outline" onClick={() => {setIsOpen(false); setFile(undefined);}}>
              Cancel
            </Button>
            <Button onClick={handleUploadFile}  disabled={!file}>
              Upload
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
