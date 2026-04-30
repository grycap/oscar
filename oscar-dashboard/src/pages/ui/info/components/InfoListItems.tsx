import { alert } from "@/lib/alert";
import { Select, SelectContent, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Copy } from "lucide-react";
import InfoItem from "./InfoItem";

interface Props {
  label: string;
  placeholder: string;
  values: string[];
  enableCopy?: boolean;
}

function InfoListItems({
  label,
  placeholder,
  values,
  enableCopy = false,
}: Props) {

  async function handleCopy() {
    await navigator.clipboard.writeText(values.toString());
    alert.success(label + " copied to clipboard");
  }

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "row",
        justifyContent: "space-between",
        alignItems: "center",
        padding: "16px",
        whiteSpace: "pre-wrap",
        flexWrap: "wrap",
      }}
    >
      <h2 style={{ fontSize: "13px", fontWeight: "500" }}>{label}</h2>
      <div
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "center",
          columnGap: "16px",
        }}
      >
        <div
          style={{
            fontSize: "13px",
            fontWeight: "500",
            maxWidth: "30vw",
            whiteSpace: "pre-wrap",
            wordWrap: "break-word",
          }}
        >
          <Select>
            <SelectTrigger style={{
            background: 'transparent',
            border: 'transparent',
          }}>
              <SelectValue placeholder={placeholder} />
            </SelectTrigger>
            <SelectContent>
              {values.map((item) => {
                return (
                  <InfoItem key={item} label={item} value={item} displayLabel={false} enableCopy />
                );
              })}
            </SelectContent>
          </Select>
        </div>
        {enableCopy && (
          <Copy
            size={16}
            style={{
              cursor: "pointer",
              marginTop: "3px",
            }}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              handleCopy();
            }}
          />
        )}
      </div>
    </div>
  );
}

export default InfoListItems;
