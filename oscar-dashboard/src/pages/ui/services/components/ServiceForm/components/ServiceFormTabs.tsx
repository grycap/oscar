import { OscarStyles } from "@/styles";
import { ServiceFormTab } from "../../../models/service";
import { Dispatch, SetStateAction } from "react";

interface Props {
  tab: ServiceFormTab;
  setTab: Dispatch<SetStateAction<ServiceFormTab>>;
}

function ServiceFormTabs({ tab: activeTab, setTab }: Props) {
  return (
    <div
      style={{
        background: "white",
        borderBottom: OscarStyles.border,
        display: "flex",
        flexDirection: "row",
        padding: "0 16px",
      }}
    >
      {Object.keys(ServiceFormTab)
        .filter((tab) => isNaN(Number(tab)))
        .map((tab) => {
          const isSelected = tab === ServiceFormTab[activeTab];
          return (
            <div
              style={{
                height: 34,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                padding: "0 16px",
                cursor: "pointer",
                position: "relative",
              }}
              onClick={() =>
                setTab(ServiceFormTab[tab as keyof typeof ServiceFormTab])
              }
              key={tab}
            >
              {isSelected && (
                <div
                  style={{
                    position: "absolute",
                    bottom: -1,
                    left: 0,
                    height: 1,
                    width: "100%",
                    background: "black",
                  }}
                ></div>
              )}
              {tab}
            </div>
          );
        })}
    </div>
  );
}

export default ServiceFormTabs;
