import { OscarStyles } from "@/styles";
import { motion, AnimatePresence } from "framer-motion";
import { Copy } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { alert } from "@/lib/alert";
import "./style.css";

export default function UserInfo() {
  const authContext = useAuth();
  const transition = { duration: 0.2, ease: "easeOut" };

  return (
    <div
      style={{
        borderLeft: OscarStyles.border,
        position: "relative",
        padding: "0 16px",
        overflow: "hidden",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <AnimatePresence mode="popLayout" initial={false}>
        {
          <motion.div
            key="text"
            className="text-decoration-underline-hover"
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 20 }}
            whileHover={{
              scale: 1.02,
            }}
            transition={transition}
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              gap: 10,
              overflow: "hidden",
            }}
            onClick={() => {
              navigator.clipboard.writeText(authContext.authData.endpoint);
              alert.success("Endpoint copied to clipboard");
            }}
          >
            <span
              style={{
                textWrap: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              <div className="grid grid-cols-1 xl:grid-cols-[auto_auto]">
                <div className="truncate">
                  {`${authContext.authData.user} -\u00A0`}
                </div>
                <div className="truncate">
                  {`${authContext.authData.endpoint}`}
                </div>
              </div>
            </span>
            <Copy className="h-4 w-4" />
          </motion.div>
        }
      </AnimatePresence>
    </div>
  );
}
