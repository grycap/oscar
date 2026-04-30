import OscarLogo from "@/assets/oscar-big.png";
import { Codesandbox, Database, Info, LogOut, Notebook } from "lucide-react";
import OscarColors from "@/styles";
import { useAuth } from "@/contexts/AuthContext";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";
import { AnimatePresence, motion } from "framer-motion";
import { Link, useLocation } from "react-router-dom";

function AppSidebar() {
  const authContext = useAuth();
  const { open } = useSidebar();
  const location = useLocation();

  const items = [
    {
      title: "Services",
      icon: <Codesandbox size={20} />,
      path: "/services",
    },
    {
      title: "Buckets",
      icon: <Database size={20} />,
      path: "/minio",
    },
    {
      title: "Notebooks",
      icon: <Notebook size={20} />,
      path: "/notebooks",
    },
    {
      title: "Info",
      icon: <Info size={20} />,
      path: "/info",
    },
  ];

  function handleLogout() {
    localStorage.removeItem("authData");
    authContext.setAuthData({
      user: "",
      password: "",
      endpoint: "",
      token: undefined,
      authenticated: false,
    });
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "16px",
            paddingTop: 6,
            height: "59px",
          }}
        >
          <AnimatePresence mode="popLayout">
            {open && (
              <motion.img src={OscarLogo} alt="Oscar logo" width={140} />
            )}
          </AnimatePresence>
          <SidebarTrigger />
        </div>
      </SidebarHeader>
      <SidebarContent className="mt-8">
        <SidebarGroup>
          <SidebarGroupContent>
            {items.map((item) => {
              const isActive = location.pathname.includes(item.path);
              return (
                <SidebarMenuItem key={item.title}>
                  <SidebarMenuButton asChild tooltip={item.title}>
                    <Link
                      to={"/ui" + item.path}
                      style={{
                        textDecoration: "none",
                        position: "relative",
                        fontWeight: isActive ? "bold" : undefined,
                      }}
                    >
                      {item.icon}
                      <span className="text-[16px]">{item.title}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              );
            })}
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarSeparator />
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild tooltip="Log out">
              <div
                onClick={handleLogout}
                style={{
                  height: "33px",
                  display: "flex",
                  justifyContent: "space-between",
                  cursor: "pointer",
                }}
              >
                <LogOut color={OscarColors.Red} />
                <span>Log out</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  );
}

export default AppSidebar;
