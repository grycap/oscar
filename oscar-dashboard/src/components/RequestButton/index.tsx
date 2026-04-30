import { useState } from "react";
import { Button, ButtonProps } from "../ui/button";
import { Loader2 } from "lucide-react";
import { motion } from "framer-motion";

type Props = {
  request: () => Promise<void>;
  icon?: React.ReactNode;
} & ButtonProps;

/**
 * RequestButton is a button that makes a request when clicked showing a loader when the request is in progress.
 *
 * @param {React.ReactNode} children - The children of the button.
 * @param {() => Promise<void>} request - The request function to be called when the button is clicked.
 * @param {ButtonProps} props - The props of the button.
 */
function RequestButton({
  children,
  request,
  icon = <Loader2 className="animate-spin" />,
  ...props
}: Props) {
  const [isLoading, setIsLoading] = useState(false);

  async function onClick() {
    if (props.disabled) return;
    if (isLoading) return; // Evita múltiples clics
    setIsLoading(true);
    await request();
    setIsLoading(false);
  }

  return (
    <Button onClick={onClick} {...props} >
      <div className="grid grid-cols-[auto_1fr] gap-1 items-center">
        <motion.div
          initial={{ width: 0, opacity: 0 }}
          animate={isLoading ? { width: 24, opacity: 1 } : { width: 0, opacity: 0 }}
          transition={{ type: "spring", stiffness: 300, damping: 30 }}
          style={{ overflow: "hidden", display: "flex", alignItems: "center" }}
        >
          {isLoading && <Loader2 className="animate-spin" />}
        </motion.div>
        <div>
          {children}
        </div>
      </div>
    </Button>
  );
}

export default RequestButton;
