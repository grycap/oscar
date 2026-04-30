import React, { useEffect, useState } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { OscarStyles } from "@/styles";
import { ArrowDownAZ, ArrowUpAZ } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

export type ColumnDef<T> = {
  header: string;
  accessor: keyof T | ((item: T) => React.ReactNode);
  sortBy: keyof T ;
};

type ActionButton<T> = {
  button: (item: T) => React.ReactNode;
};

type GenericTableProps<T> = {
  data: T[];
  columns: ColumnDef<T>[];
  onRowClick?: (item: T) => void;
  actions?: ActionButton<T>[];
  bulkActions?: ActionButton<T[]>[];
  globalActions?: ActionButton<T[]>[];
  idKey: keyof T;
};

function GenericTable<T extends object>({
  data,
  columns,
  actions,
  bulkActions,
  globalActions,
  onRowClick,
  idKey,
}: GenericTableProps<T>) {
  const [selectedRows, setSelectedRows] = useState<Set<T[typeof idKey]>>(
    new Set()
  );

  useEffect(() => {
    setSelectedRows(new Set());
  }, [data.length]);

  const [sortConfig, setSortConfig] = useState<{
    key: keyof T;
    direction: "asc" | "desc";
  } | null>(null);

  const sortedData = React.useMemo(() => {
    if (sortConfig !== null) {
      return [...data].sort((a, b) => {
        const aValue = a[sortConfig.key];
        const bValue = b[sortConfig.key];
        if (aValue < bValue) {
          return sortConfig.direction === "asc" ? -1 : 1;
        }
        if (aValue > bValue) {
          return sortConfig.direction === "asc" ? 1 : -1;
        }
        return 0;
      });
    }
    return data;
  }, [data, sortConfig]);

  const handleHeaderClick = (column: ColumnDef<T>) => {
    if (sortConfig?.key === column.accessor) {
      setSortConfig({
        key: column.sortBy as keyof T,
        direction: sortConfig.direction === "asc" ? "desc" : "asc",
      });
    } else {
      setSortConfig({ key: column.accessor as keyof T, direction: "asc" });
    }
  };

  const toggleAll = () => {
    if (selectedRows.size === sortedData.length) {
      setSelectedRows(new Set());
    } else {
      setSelectedRows(new Set(sortedData.map((item) => item[idKey])));
    }
  };

  const toggleRow = (id: T[typeof idKey]) => {
    const newSelected = new Set(selectedRows);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedRows(newSelected);
  };

  return (
    <div className="relative flex flex-col flex-grow flex-basis-0 overflow-hidden">
      <Table className="overflow-y-auto">
        <TableHeader className="sticky top-0 z-10 h-[34px]">
          <TableRow
            style={{
              background: "white",
              padding: 0,
              height: "34px",
              borderBottom: OscarStyles.border,
            }}
          >
            <TableHead className="w-[50px] h-[34px]" style={{ height: "34px" }}>
              <Checkbox
                checked={
                  sortedData.length > 0 &&
                  selectedRows.size === sortedData.length
                }
                onCheckedChange={toggleAll}
              />
            </TableHead>
            {columns.map((column, index) => (
              <TableHead
                key={index}
                style={{ height: "34px" }}
                onClick={() => handleHeaderClick(column)}
              >
                <div className="flex items-center gap-1 cursor-pointer">
                  {column.header}
                  {sortConfig?.key === column.accessor &&
                    (sortConfig.direction === "asc" ? (
                      <ArrowDownAZ size={20} />
                    ) : (
                      <ArrowUpAZ size={20} />
                    ))}
                </div>
              </TableHead>
            ))}
            {actions && (
              <TableHead className="text-right pr-6" style={{ height: "34px" }}>
                Actions
              </TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {sortedData?.map((item, rowIndex) => (
            <TableRow
              key={rowIndex}
              onClick={() => onRowClick?.(item)}
              className="cursor-pointer"
            >
              <TableCell>
                <Checkbox
                  checked={selectedRows.has(item[idKey])}
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleRow(item[idKey]);
                  }}
                />
              </TableCell>
              {columns.map((column, colIndex) => (
                <TableCell key={colIndex}>
                  {typeof column.accessor === "function"
                    ? column.accessor(item)
                    : (item[column.accessor] as React.ReactNode)}
                </TableCell>
              ))}
              {actions && (
                <TableCell>
                  <div className="flex justify-end items-center">
                    {actions.map((action, index) => (
                      <div key={index} onClick={(e) => e.stopPropagation()}>
                        {action.button(item)}
                      </div>
                    ))}
                  </div>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <AnimatePresence mode="popLayout">
        {(globalActions || (bulkActions && selectedRows.size > 0)) && (
          <motion.div
            initial={{
              opacity: 0,
              y: 20,
            }}
            animate={{
              opacity: 1,
              y: 0,
            }}
            exit={{
              opacity: 0,
              y: 20,
            }}
            style={{
              borderTop: OscarStyles.border,
              position: "sticky",
              bottom: 0,
              left: 0,
              background: "white",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              padding: "4px",
            }}
          >
            <AnimatePresence mode="popLayout">
              {globalActions && (
                <motion.div
                  key="global-actions"
                  layout
                  layoutId="global-actions"
                  initial={{
                    opacity: 0,
                  }}
                  animate={{
                    opacity: 1,
                  }}
                  exit={{
                    opacity: 0,
                  }}
                  transition={{
                    duration: 0.2,
                    ease: "easeOut",
                  }}
                  className="flex items-center gap-1"
                >
                  {globalActions.map((action, index) => {
                    return (
                      <motion.div layout key={index}>
                        {action.button(data)}
                      </motion.div>
                    );
                  })}
                </motion.div>
              )}
              {bulkActions && selectedRows.size > 0 && (
                <motion.div
                  key="bulk-actions"
                  layoutId="bulk-actions"
                  initial={{
                    opacity: 0,
                  }}
                  animate={{
                    opacity: 1,
                  }}
                  exit={{
                    opacity: 0,
                  }}
                  transition={{
                    duration: 0.2,
                    ease: "easeOut",
                  }}
                  className="flex items-center gap-1"
                >
                  {bulkActions.map((action, index) => {
                    const idKeys = Array.from(selectedRows.values());
                    const items = data?.filter((item) =>
                      idKeys.includes(item[idKey])
                    );

                    return (
                      <motion.div layout key={index}>
                        {action.button(items)}
                      </motion.div>
                    );
                  })}
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export default GenericTable;
