import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useMinio } from "@/contexts/Minio/MinioContext";
import { Plus } from "lucide-react";
import useSelectedBucket from "../../hooks/useSelectedBucket";
import { useMediaQuery } from "react-responsive";

export default function AddFolderButton() {
  const { name: bucketName, path } = useSelectedBucket();
  const [folderName, setFolderName] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const { createFolder } = useMinio();
  const isSmallScreen = useMediaQuery({ maxWidth: 799 });

  const handleCreateFolder = async () => {
    await createFolder(bucketName as string, path + folderName);
    setFolderName("");
    setIsOpen(false);
  };

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setIsOpen(false);
      }

      if (e.key === "Enter") {
        handleCreateFolder();
      }
    }
    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [handleCreateFolder]);

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger asChild>
        <Button variant="mainGreen" style={{gap: 8}}>
          <Plus size={20} className="h-5 w-5" />
          {!isSmallScreen && "Create Folder"}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80">
        <div className="grid gap-4">
          <div className="space-y-2">
            <h4 className="font-medium leading-none">Create Folder</h4>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="folderName">Folder Name</Label>
            <Input
              id="folderName"
              value={folderName}
              onChange={(e) => setFolderName(e.target.value)}
              placeholder="Enter the folder name"
            />
          </div>
          <div className="flex justify-end space-x-2">
            <Button variant="outline" onClick={() => setIsOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreateFolder} disabled={!folderName.trim()}>
              Create
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
