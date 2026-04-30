import { useAuth } from "@/contexts/AuthContext";
import { useMinio } from "@/contexts/Minio/MinioContext";
import OscarColors, { OscarStyles } from "@/styles";
import InfoItem from "./components/InfoItem";
import InfoBooleanItem from "./components/InfoBooleanItem";
import InfoListItems from "./components/InfoListItems";
import { useEffect } from "react";
import { useMediaQuery } from "react-responsive";
import { useSidebar } from "@/components/ui/sidebar";

function InfoView() {
  
  useEffect(() => {
    document.title ="OSCAR - Info"
  });
  const { authData, systemConfig, clusterInfo } = useAuth();
  const { endpoint, user, password, egiSession, token } = authData;
  const { providerInfo } = useMinio();
  const { open } = useSidebar();
  // 1976 is the width when flex wrap is applied with the sidebar open
  // 1824 is the width when flex wrap is applied with the sidebar closed
  const isBigScreen = useMediaQuery({maxWidth: open ? 1976 : 1824});

  if (!systemConfig) return null;
  if (!authData.authenticated) return null;

  return (
    <div className="grid grid-cols-1 gap-6 w-[95%] sm:w-[90%] lg:w-[80%] mx-auto mt-[40px] min-w-[300px] content-start">
      <div className={(isBigScreen ? "flex justify-center": "")}>
        <div className="max-w-[700px] w-full text-center sm:text-left">
          <h1 style={{ fontSize: "24px", fontWeight: "500" }}>
            Server information
          </h1>
        </div>
      </div>
      <div className={"flex flex-wrap gap-5 w-full items-start" + (isBigScreen ? " justify-center": "")}>
        <div className="max-w-[700px] w-full"
          style={{
            border: OscarStyles.border,
            borderRadius: "8px",
          }}
        >
          <div
            style={{
              background: OscarColors.Gray2,
              padding: "16px",
            }}
          >
            <h1 style={{ fontSize: "16px", fontWeight: "500" }}>
              User
            </h1>
          </div>
          <InfoItem label="User" value={user} enableCopy />
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          {token ? (
              <>
                <InfoItem label="EGI UID" value={egiSession?.sub! ?? egiSession?.sub!} enableCopy />
                <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
                <InfoItem
                  label="Access Token"
                  value={token}
                  isPassword
                  enableCopy
                />
              </>
            )
            :
            <>
              <InfoItem label="Password" value={password} isPassword enableCopy />
              <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
            </>
          }
        </div>
        <div className="max-w-[700px] w-full"
          style={{
            border: OscarStyles.border,
            borderRadius: "8px",
          }}
        >
          <div
            style={{
              background: OscarColors.Gray2,
              padding: "16px",
            }}
          >
            <h1 style={{ fontSize: "16px", fontWeight: "500" }}>
              OSCAR Cluster
            </h1>
          </div>
          <InfoItem label="Endpoint" value={endpoint} enableCopy isLink />
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          {systemConfig.config.oidc_groups.length > 1 ? 
            <InfoListItems  label="Supported VOs" placeholder={systemConfig.config.oidc_groups[0] + '... '} values={systemConfig.config.oidc_groups} enableCopy />
            :
            <InfoItem label="Supported VOs" value={systemConfig.config.oidc_groups.toString()} enableCopy />
          }
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          <InfoItem label="Version" value={clusterInfo?.version!} enableCopy />
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          <div
            style={{
              padding: "16px",
              display: "flex",
              justifyContent: "space-evenly",
            }}
          >
            <InfoBooleanItem
              label="GPU"
              enabled={Boolean(systemConfig?.config.gpu_available)}
            />

            <InfoBooleanItem
              label="InterLink"
              enabled={Boolean(systemConfig?.config.interLink_available)}
            />
            <InfoBooleanItem
              label="Yunikorn"
              enabled={Boolean(systemConfig?.config.yunikorn_enable)}
            />
          </div>
        </div>
        <div className="max-w-[700px] w-full"
          style={{
            border: OscarStyles.border,
            borderRadius: "8px",
          }}
        >
          <div
            style={{
              background: OscarColors.Gray2,
              padding: "16px",
            }}
          >
            <h1 style={{ fontSize: "16px", fontWeight: "500" }}>
              MinIO
            </h1>
          </div>
          <InfoItem label="Endpoint" value={providerInfo.endpoint} enableCopy isLink />
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          <InfoItem
            label="Access key"
            value={providerInfo.access_key}
            enableCopy
          />
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          <InfoItem
            label="Secret key"
            value={providerInfo.secret_key}
            isPassword
            enableCopy
          />
          <div style={{ borderTop: OscarStyles.border, margin: "0px 16px" }} />
          <div
            style={{
              padding: "16px",
              display: "flex",
              justifyContent: "space-evenly",
            }}
          >
            <InfoBooleanItem
              label="SSL"
              enabled={Boolean(providerInfo.endpoint.includes("http://"))}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export default InfoView;
