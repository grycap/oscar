import { Button } from "@/components/ui/button";
import { ServiceViewMode } from "..";
import { useLastUriParam } from "@/hooks/useLastUriParam";
import { Link, useParams } from "react-router-dom";
import { CreateUpdateServiceButton } from "./CreateUpdateButton";
import { InvokePopover } from "../../InvokePopover";
import { AnimatePresence, motion, MotionConfig } from "framer-motion";

interface Props {
  mode: ServiceViewMode;
}

function CreateUpdateServiceTabs({ mode }: Props) {
  const tab = useLastUriParam();
  const { serviceId } = useParams();
  const isInCreateMode = mode === ServiceViewMode.Create;
  const isInUpdateMode = mode === ServiceViewMode.Update;

  function getVariant(label: string) {
    return tab === label ? "lightGreen" : "ghost";
  }

  const isSettingsTab = tab === "settings";

  return (
    <MotionConfig transition={{ duration: 0.2, type: "spring", bounce: 0 }}>
      <div
        style={{
          display: "flex",
          flexDirection: "row",
          flexGrow: 1,
          justifyContent: "center",
          gap: "9px",
        }}
      >
        {isInUpdateMode && (
          <motion.div
            layout
            style={{
              display: "flex",
              flexDirection: "row",
              gap: "9px",
              marginLeft: "auto",
            }}
          >
            <Link to={`/ui/services/${serviceId}/settings`}>
              <Button variant={getVariant("settings")}>Settings</Button>
            </Link>

            <Link to={`/ui/services/${serviceId}/logs`}>
              <Button variant={getVariant("logs")}>Logs</Button>
            </Link>
          </motion.div>
        )}

        <div
          style={{
            minWidth: "80px",
            marginLeft: "auto",
            display: "flex",
            flexDirection: "row",
            justifyContent: "flex-end",
            gap: "10px",
          }}
        >
          {isInUpdateMode && (
            <motion.div layout>
              <InvokePopover />
            </motion.div>
          )}
          <AnimatePresence mode="popLayout">
            {(isSettingsTab || isInCreateMode) && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 20 }}
              >
                <CreateUpdateServiceButton isInCreateMode={isInCreateMode} />
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </MotionConfig>
  );
}

export default CreateUpdateServiceTabs;
