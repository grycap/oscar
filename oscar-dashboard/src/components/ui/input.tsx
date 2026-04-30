import * as React from "react";
import { useState } from "react";

import { cn } from "@/lib/utils";
import { Label } from "./label";
import { Eye, EyeOff } from "lucide-react";

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  endIcon?: React.ReactNode;
  label?: string;
  flex?: string | number;
  error?: string;
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, required, endIcon, label, ...props }, ref) => {
    const [showPassword, setShowPassword] = useState(false);

    const togglePasswordVisibility = () => {
      setShowPassword((prev) => !prev);
    };

    return (
      <div
        className="relative grid"
        style={{
          width: props.width,
          flex: props.flex,
        }}
      >
        {label && 
          <div className="flex flex-row mb-1">
            {label && <Label>{label + (required ? "*" : "")}</Label>}
            {label && props.error && <Label>-</Label>}
            {props.error && <Label className="text-red-500">{props.error}</Label>}
          </div>
        }
        <input
          type={type === "password" && showPassword ? "text" : type}
          className={cn(
            "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:bg-white",
            className,
            (endIcon || type === "password") && "pr-10"
          )}
          ref={ref}
          {...props}
        />
        {type === "password" && (
          <div
            className="absolute inset-y-0 right-0 flex justify-center items-end pr-3 pb-3 cursor-pointer"
            onClick={togglePasswordVisibility}
          >
            {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
          </div>
        )}
        {endIcon && (
          <div className="absolute inset-y-0 right-3 flex items-center pointer-events-none">
            {endIcon}
          </div>
        )}
      </div>
    );
  }
);
Input.displayName = "Input";

export { Input };
