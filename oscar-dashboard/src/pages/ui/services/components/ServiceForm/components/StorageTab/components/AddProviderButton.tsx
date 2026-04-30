import { Button } from "@/components/ui/button";
import { StorageProvider } from "@/pages/ui/services/models/service";
import { Plus } from "lucide-react";
import { Dispatch, SetStateAction } from "react";
/* import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"; */

interface Props {
  setSelectedProvider: Dispatch<SetStateAction<StorageProvider | null>>;
}

function AddProviderButton({ setSelectedProvider }: Props) {
  return (
    <Button
      onClick={() => {
        setSelectedProvider({ type: "minio" } as StorageProvider);
      }}
    >
      <Plus className="h-4 w-4 mr-2" />
      Add provider
    </Button>
  );
  /*  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button>
          <Plus className="h-4 w-4 mr-2" />
          Add provider
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuLabel>Available providers:</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem
            onClick={() => {
              setSelectedProvider({ type: "minio" } as StorageProvider);
            }}
          >
            Minio
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu> 
  );*/
}

export default AddProviderButton;
