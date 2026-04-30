import { useEffect, useMemo, useState } from "react";
import useServicesContext from "../../context/ServicesContext";
import Log from "../../models/log";
import { getServiceLogsApi } from "@/api/logs/getServiceLogs";
import GenericTable from "@/components/Table";
import { Badge, BadgeProps } from "@/components/ui/badge";
import { Eye, Loader,  Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import LogDetailsPopover from "./components/LogDetailsPopover";
import DeleteDialog from "@/components/DeleteDialog";
import { deleteLogApi } from "@/api/logs/deleteLog";
import { alert } from "@/lib/alert";
import deleteServiceLogsApi from "@/api/logs/deleteServiceLogs";

export type LogWithName = Log & { name: string };

export default function ServiceLogs() {
  const { formService } = useServicesContext();
  const [logs, setLogs] = useState<Record<string, Log>>({});
  const logsWithName = useMemo(
    () =>
      Object.entries(logs).map(([name, log]) => ({
        name,
        ...log,
      })) as LogWithName[],
    [logs]
  );

  const [selectedLog, setSelectedLog] = useState<LogWithName | null>(null);
  const [logsToDelete, setLogsToDelete] = useState<LogWithName[]>([]);

  function fetchServices() {
    if (!formService?.name) return;
    getServiceLogsApi(formService.name).then(setLogs);
  }

  useEffect(() => {
    fetchServices();
  }, [formService?.name]);

  function renderStatus(status: Log["status"]) {
    const variant: Record<Log["status"], BadgeProps["variant"]> = {
      Succeeded: "success",
      Failed: "destructive",
      Running: "default",
      Pending: "secondary",
    };
    return (
      <Badge variant={variant[status]}>
        {status === "Running" && (
          <Loader className="animate-spin h-3 w-3 mr-2" />
        )}
        {status}
      </Badge>
    );
  }

  function formatTimestamp(timestamp: string) {
    if (!timestamp) return null;
    const date = new Date(timestamp);
    const label = new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(date);

    return (
      <span
        style={{
          fontFamily: "Geist Mono, sans-serif",
        }}
      >
        {label}
      </span>
    );
  }

  async function handleDeleteLogs() {
    if (!formService?.name) return;

    const failedLogs: string[] = [];
    const deletedLogs: string[] = [];

    try {
      const promises = logsToDelete.map((log) =>
        deleteLogApi(formService.name, log.name)
          .then(() => deletedLogs.push(log.name))
          .catch(() => failedLogs.push(log.name))
      );

      await Promise.all(promises);

      if (deletedLogs.length === 1) {
        alert.success(`The log "${deletedLogs[0]}" was deleted successfully!`);
      } else if (failedLogs.length === logsToDelete.length) {
        console.error("All logs failed to delete");
        alert.error("Failed to delete all logs. Please try again later.");
      } else if (deletedLogs.length === logsToDelete.length) {
        alert.success("All logs were deleted successfully!");
      } else {
        alert.error(
          `Failed to delete the following logs: ${failedLogs.join(", ")}`
        );
      }

      fetchServices();
    } catch (error) {
      alert.error("An unexpected error occurred while deleting logs");
    } finally {
      setLogsToDelete([]);
    }
  }

  const [deleteAllLogs, setDeleteAllLogs] = useState(false);

  function handleDeleteAllLogs() {
    deleteServiceLogsApi(formService?.name)
      .then(() => {
        alert.success("Service logs were deleted successfully!");
        fetchServices();
      })
      .catch(() => {
        alert.error("Failed to delete service logs.");
      })
      .finally(() => {
        setDeleteAllLogs(false);
      });
  }
  return (
    <div className="flex flex-grow" style={{
        display: "flex",
        flexDirection: "column",
        flexGrow: 1,
        flexBasis: 0,
        overflow: "hidden",
      }}>
      <LogDetailsPopover
        log={selectedLog}
        serviceName={formService?.name}
        onClose={() => setSelectedLog(null)}
      />
      <DeleteDialog
        isOpen={deleteAllLogs}
        onClose={() => setDeleteAllLogs(false)}
        onDelete={handleDeleteAllLogs}
        itemNames={[`All service logs (${logsWithName.length})`]}
      />
      <DeleteDialog
        isOpen={logsToDelete.length > 0}
        onClose={() => setLogsToDelete([])}
        onDelete={handleDeleteLogs}
        itemNames={logsToDelete.map((log) => log.name)}
      />
      <GenericTable<LogWithName>
        data={logsWithName}
        columns={[
          { accessor: "name", header: "Name", sortBy: "name"},
          {
            header: "Status",
            accessor(item) {
              return item.status,renderStatus(item.status);
            },
            sortBy: "status"

          },
          {
            header: "Creation Time",
            accessor(item) {
              return item.creation_time, formatTimestamp(item.creation_time);
            },
            sortBy: "creation_time"
          },
          {
            header: "Start Time",
            accessor(item) {
              return formatTimestamp(item.start_time);
            },
            sortBy: "start_time"
          },
          {
            header: "Finish Time",
            accessor(item) {
              return formatTimestamp(item.finish_time);
            },
            sortBy: "finish_time"
          },
        ]}
        idKey="name"
        actions={[
          {
            button(item) {
              return (
                <Button
                  variant="link"
                  size="sm"
                  tooltipLabel="View"
                  onClick={() => setSelectedLog(item)}
                >
                  <Eye />
                </Button>
              );
            },
          },
          {
            button(item) {
              return (
                <Button
                  variant="link"
                  size="sm"
                  tooltipLabel="Delete"
                  className="text-red-500"
                  onClick={() => setLogsToDelete([...logsToDelete, item])}
                >
                  <Trash2 />
                </Button>
              );
            },
          },
        ]}
        bulkActions={[
          {
            button: (items) => (
              <Button
                variant="destructive"
                onClick={() => setLogsToDelete(items)}
              >
                <Trash2 className="h-5 w-5 mr-2"></Trash2> Delete selected logs
                ({items.length})
              </Button>
            ),
          },
        ]}
        globalActions={[
          {
            button: () => (
              <Button
                variant="destructive"
                onClick={() => setDeleteAllLogs(true)}
              >
                <Trash2 className="h-5 w-5 mr-2"></Trash2> Delete all logs
              </Button>
            ),
          },
        ]}
      />
    </div>
  );
}
