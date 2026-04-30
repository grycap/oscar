import { useParams } from "react-router-dom";

export default function useSelectedBucket() {
  const { name, ...params } = useParams();
  const path = params["*"] ?? "";
  return { name, path };
}
