import ReactDOM from "react-dom/client";
import "./globals.css";
import { AuthProvider } from "./contexts/AuthContext.tsx";
import { Toaster } from "sonner";
import AppRouter from "./routes/router.tsx";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <>
    <AuthProvider>
      <Toaster />
      <AppRouter />
    </AuthProvider>
  </>
);
