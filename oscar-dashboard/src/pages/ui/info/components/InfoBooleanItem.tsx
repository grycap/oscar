import OscarColors from "@/styles";
import { Check, X } from "lucide-react";

type Props = {
  label: string;
  enabled: boolean;
};

export default function InfoBooleanItem({ label, enabled }: Props) {
  return (
    <div
      style={{
        display: "flex",
        columnGap: 9,
        alignItems: "center",
      }}
    >
      <span style={{ fontSize: "13px", fontWeight: "500" }}>{label}</span>
      {enabled ? (
        <Check size={16} color={OscarColors.Green4} style={{ marginTop: 2 }} />
      ) : (
        <X size={16} color={OscarColors.Red} style={{ marginTop: 2 }} />
      )}
    </div>
  );
}
