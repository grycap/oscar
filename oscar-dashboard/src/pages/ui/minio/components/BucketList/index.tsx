import DeleteDialog from "@/components/DeleteDialog";
import GenericTable from "@/components/Table";
import { Button } from "@/components/ui/button";
import { useMinio } from "@/contexts/Minio/MinioContext";
import OscarColors from "@/styles";
import { Bucket } from "@aws-sdk/client-s3";
import { Trash } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
export default function BucketList() {
  const { buckets, deleteBucket } = useMinio();
  const [itemsToDelete, setItemsToDelete] = useState<Bucket[]>([]);
  return (
    <>
      <DeleteDialog
        isOpen={itemsToDelete.length > 0}
        onClose={() => setItemsToDelete([])}
        onDelete={() => {
          itemsToDelete.forEach((bucket) => deleteBucket(bucket.Name!));
        }}
        itemNames={itemsToDelete.map((bucket) => bucket.Name!)}
      />
      <GenericTable<Bucket>
        data={buckets}
        columns={[
          {
            header: "Name",
            accessor: (row) => (
              <Link to={`/ui/minio/${row.Name}`}>{row.Name}</Link>
            ),
            sortBy: "Name"
          },
        ]}
        actions={[
          {
            button: (bucket) => {
              return (
                <>
                  <Button
                    variant="link"
                    size="icon"
                    onClick={() => {
                      setItemsToDelete([...itemsToDelete, bucket]);
                    }}
                  >
                    <Trash color={OscarColors.Red} />
                  </Button>
                </>
              );
            },
          },
        ]}
        idKey="Name"
      />
    </>
  );
}
