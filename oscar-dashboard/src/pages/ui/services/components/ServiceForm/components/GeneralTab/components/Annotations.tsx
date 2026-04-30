import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import useServicesContext from "@/pages/ui/services/context/ServicesContext";
import { Plus, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

function Annotations() {
  const { formService, setFormService } = useServicesContext();

  const initialArray = useMemo(() => {
    const annotations = formService.annotations;
    const array = annotations
      ? Object.entries(annotations).map(([key, value]) => {
          return {
            key,
            value,
          };
        })
      : [];

    return array;
  }, [formService]);

  const [annotationsArray, setAnnotationsArray] = useState(initialArray);

  useEffect(() => {
    setFormService((prev) => ({
      ...prev,
        annotations: annotationsArray.reduce((acc, curr) => {
          acc[curr.key] = curr.value;
          return acc;
        }, {} as Record<string, string>),
    }));
  }, [annotationsArray]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "9px",
      }}
    >
      {annotationsArray.map((variable, index) => (
        <div
          key={index}
          style={{ display: "flex", gap: "5px", alignItems: "center" }}
        >
          <Input
            id={`annotations-name-input-${index}`}
            value={variable.key}
            onChange={(e) => {
              const newVariablesArray = [...annotationsArray];
              newVariablesArray[index].key = e.target.value;
              setAnnotationsArray(newVariablesArray);
            }}
            placeholder="Annotation name"
          />
          <Input
            id={`annotation-value-input-${index}`}
            type="password"
            value={variable.value}
            onFocus={(e) => (e.target.type = "text")}
            onBlur={(e) => (e.target.type = "password")}
            style={{ width: 300 }}
            onChange={(e) => {
              const newVariablesArray = [...annotationsArray];
              newVariablesArray[index].value = e.target.value;
              setAnnotationsArray(newVariablesArray);
            }}
            placeholder="Value"
          />
          {annotationsArray.length > 0 && (
            <Button
              id={`remove-annotation-button-${index}`}
              size={"icon"}
              variant={"ghost"}
              onClick={() => {
                const newVariablesArray = annotationsArray.filter(
                  (_, i) => i !== index
                );
                setAnnotationsArray(newVariablesArray);
              }}
            >
              <X size={16} />
            </Button>
          )}
        </div>
      ))}
      <Button
        id="add-annotations-button"
        size={"sm"}
        style={{
          width: "max-content",
        }}
        onClick={() => {
          setAnnotationsArray([...annotationsArray, { key: "", value: "" }]);
        }}
      >
        <Plus className="h-4 w-4 mr-2" /> Add Annotation
      </Button>
    </div>
  );
}

export default Annotations;
