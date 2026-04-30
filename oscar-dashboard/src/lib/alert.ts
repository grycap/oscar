import OscarColors from "@/styles";
import React from "react";
import { toast } from "sonner";

class ToastAlert {
  constructor() {}

  default(message: React.ReactNode | string) {
    toast(message);
  }

  success(message: string) {
    toast.success(message, {
      style: {
        backgroundColor: "#17A34B",
        color: "white",
        border: "none",
      },
    });
  }

  error(message: React.ReactNode | string) {
    if (typeof message === "string") {
      console.error(message);
    }

    toast.error(message, {
      style: {
        backgroundColor: OscarColors.Red,
        color: "white",
        border: "none",
      },
    });
  }

  warning(messsage: React.ReactNode | string) {
    if (typeof messsage === "string") {
      console.warn(messsage);
    }

    toast.warning(messsage, {
      style: {
        backgroundColor: "orange",
        color: "white",
        border: "none",
      },
    });
  }
}

export const alert = new ToastAlert();
