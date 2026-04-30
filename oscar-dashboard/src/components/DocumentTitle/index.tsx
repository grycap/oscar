import { useEffect } from "react";

function DocumentTitle({ value }: { value: string }) {
  useEffect(() => {
    document.title = value;
  }, [value]);

  return null;
}

export default DocumentTitle;
