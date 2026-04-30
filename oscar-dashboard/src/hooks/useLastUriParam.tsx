import { useLocation } from "react-router-dom";

export function useLastUriParam() {
  const location = useLocation();
  const pathnames = location.pathname.split("/").filter((x) => x && x !== "ui");

  return pathnames[pathnames.length - 1];
}
