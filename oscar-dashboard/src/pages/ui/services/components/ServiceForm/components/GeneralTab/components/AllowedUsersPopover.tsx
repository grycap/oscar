import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Trash2,
  Pencil,
  Search,
} from "lucide-react";
import OscarColors from "@/styles";
import useServicesContext from "@/pages/ui/services/context/ServicesContext";
import GenericTable from "@/components/Table";
import { Input } from "@/components/ui/input";
import DeleteDialog from "@/components/DeleteDialog";

interface User {
  uid: string;
}

export function AllowedUsersPopover() {
  const { formService, setFormService } = useServicesContext();
  const allowedUsersDefault = formService.allowed_users.map((user) => {return {uid: user}});
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const [allowedUsers, setAllowedUsers] = useState<User[]>(allowedUsersDefault);
  const [usersToDelete, setUsersToDelete] = useState<User[]>([]);

  const buttonRefAdd = useRef<HTMLButtonElement>(null);
  const buttonRefApply = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Enter" && buttonRefAdd.current && buttonRefApply.current) {
        e.preventDefault();
        buttonRefAdd.current.disabled ? buttonRefApply.current.click() : buttonRefAdd.current.click();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const filteredAllowedUsers = allowedUsers.filter(item =>
    item.uid.toLowerCase().includes(inputValue.toLowerCase())
  );

  const applyChanges = (users: User[]) => {
    setFormService((prev) => ({
      ...prev,
      allowed_users: users.map((user) => {return user.uid})
    }));
  };

  const addAllowedUser = (newUID: string) => {
    setAllowedUsers(prev => [...prev, { uid: newUID }]);
    setInputValue("");
  };

  const deleteSelectedAllowedUsers = () => {
    setAllowedUsers(allowedUsers.filter((user, _) =>  !usersToDelete.some((userToDelete) => userToDelete.uid === user.uid)));
  };

  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger asChild>
        <Button
          style={{ width: "unset", height: "unset" }}
          variant="link"
          size="icon"
          tooltipLabel="Edit"
          onClick={() => {setAllowedUsers(allowedUsersDefault); setIsOpen(false);}}
        >
          <Pencil size={18} />
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-[800px] max-h-[90%] overflow-y-auto gap-4">
        <DialogHeader>
          <DialogTitle>
            <span style={{ color: OscarColors.DarkGrayText }}>
              {`Allowed users: `}
            </span>
          </DialogTitle>
        </DialogHeader>

        <div className="grid grid-cols-1 sm:grid-cols-6 gap-y-2 sm:gap-x-2 ">
          <div className="col-span-5">
            <Input
              placeholder="Filter by UID"
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value.trim())}
              endIcon={<Search size={16} />}
            />
          </div>
          <div className="col-span-1 ">
            <Button 
              ref={buttonRefAdd}
              className="w-[100%]"
              disabled={
                inputValue === "" ||
                allowedUsers.some((user) => user.uid === inputValue)
              }
              onClick={() => addAllowedUser(inputValue)}
              variant={"lightGreen"}
            >
              Add
            </Button>
          </div>
        </div>
        <div className="flex border rounded p-2 max-w-[90vw] sm:max-w-[750px] max-h-[350px] min-h-[200px] sm:min-h-[350px]">
          <GenericTable<User>
            data={filteredAllowedUsers}
            idKey="uid"
            columns={[{ header: "UID", accessor: "uid", sortBy: "uid"  }]}
            actions={[
              {
                button: (user) => (
                  <Button
                    variant="link"
                    size="icon"
                    onClick={() => setUsersToDelete([user])}
                    tooltipLabel="Delete"
                  >
                    <Trash2 color={OscarColors.Red} />
                  </Button>
                ),
              },
            ]}
            bulkActions={[
              {
                button: (items) => (
                  <Button
                    variant="destructive"
                    className="flex items-center gap-2"
                    onClick={() => setUsersToDelete(items)}
                  >
                    <Trash2 className="h-5 w-5" />
                    Delete users
                  </Button>
                ),
              },
            ]}
          />
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-[140px_140px] grid-row-1 place-content-between gap-y-2 gap-x-2 ">
          <div className="col-span-1">
          <Button 
              className="w-[100%]"
              onClick={() => {setAllowedUsers(allowedUsersDefault); setIsOpen(false);}}
              variant={"destructive"}
            >
              Cancel
            </Button>
          </div>
          <div className="col-span-1">
            <Button 
              ref={buttonRefApply}
              className="w-[100%]"
              onClick={() => {applyChanges(allowedUsers); setIsOpen(false);}}
              variant={"mainGreen"}
            >
              Apply Changes
            </Button>
          </div>
        </div>
        <DeleteDialog
          isOpen={usersToDelete.length > 0}
          onClose={() => setUsersToDelete([])}
          onDelete={deleteSelectedAllowedUsers}
          itemNames={usersToDelete.map((user) => user.uid)}
        />
      </DialogContent>
    </Dialog>
  );
}
