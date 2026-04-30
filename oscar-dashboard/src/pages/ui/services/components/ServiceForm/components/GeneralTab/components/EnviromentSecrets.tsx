import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import useServicesContext from "@/pages/ui/services/context/ServicesContext";
import { Plus, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

function EnviromentSecrets() {
  const { formService, setFormService } = useServicesContext();

  const initialArray = useMemo(() => {
    const secrets = formService.environment?.secrets;
    const array = secrets
      ? Object.entries(secrets).map(([key, value]) => {
          return {
            key,
            value,
          };
        })
      : [];

    return array;
  }, [formService]);

  const [secretsArray, setSecretsArray] = useState(initialArray);

  useEffect(() => {
    setFormService((prev) => ({
      ...prev,
      environment: {
        ...prev.environment,
        secrets: secretsArray.reduce((acc, curr) => {
          acc[curr.key] = curr.value;
          return acc;
        }, {} as Record<string, string>),
      },
    }));
  }, [secretsArray]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "9px",
      }}
    >
      {secretsArray.map((variable, index) => (
        <div
          key={index}
          style={{ display: "flex", gap: "5px", alignItems: "center" }}
        >
          <Input
            id={`secret-name-input-${index}`}
            value={variable.key}
            onChange={(e) => {
              const newVariablesArray = [...secretsArray];
              newVariablesArray[index].key = e.target.value;
              setSecretsArray(newVariablesArray);
            }}
            placeholder="Secret name"
          />
          <Input
            id={`secret-value-input-${index}`}
            type="password"
            value={variable.value}
            onFocus={(e) => (e.target.type = "text")}
            onBlur={(e) => (e.target.type = "password")}
            style={{ width: 300 }}
            onChange={(e) => {
              const newVariablesArray = [...secretsArray];
              newVariablesArray[index].value = e.target.value;
              setSecretsArray(newVariablesArray);
            }}
            placeholder="Value"
          />
          {secretsArray.length > 0 && (
            <Button
              id={`remove-secret-button-${index}`}
              size={"icon"}
              variant={"ghost"}
              onClick={() => {
                const newVariablesArray = secretsArray.filter(
                  (_, i) => i !== index
                );
                setSecretsArray(newVariablesArray);
              }}
            >
              <X size={16} />
            </Button>
          )}
        </div>
      ))}
      <Button
        id="add-secret-button"
        size={"sm"}
        style={{
          width: "max-content",
        }}
        onClick={() => {
          setSecretsArray([...secretsArray, { key: "", value: "" }]);
        }}
      >
        <Plus className="h-4 w-4 mr-2" /> Add Secret
      </Button>
    </div>
  );
}

export default EnviromentSecrets;
