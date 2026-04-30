import { useAuth } from "@/contexts/AuthContext";
import { ReactNode } from "react";
import { Navigate } from "react-router-dom";

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { authData } = useAuth();

  if (Object.values(authData).some((value) => !value)) {
    return <Navigate to="/login" />;
  }

  return children;
}

export default ProtectedRoute;
