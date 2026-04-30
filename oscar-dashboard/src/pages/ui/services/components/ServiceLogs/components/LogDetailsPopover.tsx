import {
  Dialog,
  DialogTitle,
  DialogHeader,
  DialogContent,
} from "@/components/ui/dialog";
import { LogWithName } from "..";
import { useState } from "react";
import { useEffect } from "react";
import { getLogApi } from "@/api/logs/getLog";
import OscarColors from "@/styles";
import { Editor } from "@monaco-editor/react";
import { Loader } from "lucide-react";
import { alert } from "@/lib/alert";

interface Props {
  log: LogWithName | null;
  serviceName: string;
  onClose: () => void;
}

export default function LogDetailsPopover({
  log,
  serviceName,
  onClose,
}: Props) {
  const [logContent, setLogContent] = useState<string>("");
  const [isLoading, setIsLoading] = useState(false);
  const [isError, setIsError] = useState(false);

  useEffect(() => {
    if (log) {
      setIsLoading(true);
      getLogApi(serviceName, log.name)
        .then((content) => {
          setLogContent(content);
          setIsError(false);
        })
        .catch(() => {
          alert.error("Failed to fetch log");
          setIsError(true);
        })
        .finally(() => setIsLoading(false));
    }
  }, [log]);

  return (
    <Dialog open={!!log} onOpenChange={onClose}>
      <DialogContent style={{ width: "70vw" }}>
        <DialogHeader>
          <DialogTitle>
            <span style={{ color: OscarColors.DarkGrayText }}>Log name: </span>
            {log?.name}
          </DialogTitle>
        </DialogHeader>
        {isLoading ? (
          <div
            style={{
              display: "flex",
              justifyContent: "center",
              height: "500px",
              alignItems: "center",
            }}
          >
            <Loader className="animate-spin" />
          </div>
        ) : isError ? (
          <div
            style={{
              display: "flex",
              justifyContent: "center",
              height: "500px",
              alignItems: "center",
            }}
          >
            <span>Failed to fetch log</span>
          </div>
        ) : (
          <Editor
            height="500px"
            language="json"
            value={logContent || ""}
            options={{ readOnly: true, minimap: { enabled: false } }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
