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

export default function AddBucketButton() {
  const [bucketName, setBucketName] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const { createBucket } = useMinio();

  const handleCreateBucket = async () => {
    console.log("Creating bucket", bucketName);
    await createBucket(bucketName);
    setBucketName("");
    setIsOpen(false);
  };

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setIsOpen(false);
      }

      if (e.key === "Enter") {
        handleCreateBucket();
      }
    }
    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [handleCreateBucket]);

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger asChild>
        <Button variant="mainGreen">
          <Plus size={20} className="mr-2" />
          Create bucket
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80">
        <div className="grid gap-4">
          <div className="space-y-2">
            <h4 className="font-medium leading-none">Create bucket</h4>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="bucketName">Bucket Name</Label>
            <Input
              id="bucketName"
              value={bucketName}
              onChange={(e) => setBucketName(e.target.value)}
              placeholder="Enter bucket name"
            />
          </div>
          <div className="flex justify-end space-x-2">
            <Button variant="outline" onClick={() => setIsOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreateBucket} disabled={!bucketName.trim()}>
              Create
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
