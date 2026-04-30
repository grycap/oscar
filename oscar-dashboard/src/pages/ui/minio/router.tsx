import { Outlet, Route, Routes } from "react-router-dom";
import MinioTopbar from "./components/Topbar";
import BucketContent from "./components/BucketContent";
import BucketList from "./components/BucketList";
function MinioRouter() {
  return (
    <Routes>
      <Route
        path="/"
        element={
          <div
            style={{
              flexGrow: 1,
              flexBasis: 0,
              display: "flex",
              flexDirection: "column",
            }}
          >
            <MinioTopbar />

            <Outlet />
          </div>
        }
      >
        <Route path="" element={<BucketList />} />
        <Route path=":name/*" element={<BucketContent />} />
      </Route>
    </Routes>
  );
}

export default MinioRouter;
