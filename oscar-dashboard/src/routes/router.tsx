import { HashRouter, Navigate, Route, Routes } from "react-router-dom";
import ProtectedRoute from "@/components/ProtectedRoute/ProtectedRoute";
import AppLayout from "@/pages/ui/layout";
import ServicesRouter from "@/pages/ui/services/router";
import Login from "@/pages/Login";
import { PrivacyPolicy } from "@/pages/PrivacyPolicy";
import TermsOfUse from "@/pages/TermsOfUse";
import { MinioProvider } from "@/contexts/Minio/MinioContext";
import MinioRouter from "@/pages/ui/minio/router";
import InfoView from "@/pages/ui/info";
import { ServicesProvider } from "@/pages/ui/services/context/ServicesContext";
import JunoView from "@/pages/ui/juno";

function AppRouter() {
  return (
    <HashRouter>
      <Routes>
        <Route
          path="/ui"
          element={
            <ProtectedRoute>
              <ServicesProvider>
                <MinioProvider>
                  <AppLayout />
                </MinioProvider>
              </ServicesProvider>
            </ProtectedRoute>
          }
        >
          <Route path="services/*" element={<ServicesRouter />} />
          <Route path="minio/*" element={<MinioRouter />} />
          <Route path="info" element={<InfoView />} />
          <Route path="notebooks" element={<JunoView />} />
        </Route>
        <Route path="/login" element={<Login />} />
        <Route path="/terms-of-use" element={<TermsOfUse />} />
        <Route path="/privacy-policy" element={<PrivacyPolicy />} />
        <Route path="*" element={<Navigate to="/ui/services" replace />} />
      </Routes>
    </HashRouter>
  );
}

export default AppRouter;
