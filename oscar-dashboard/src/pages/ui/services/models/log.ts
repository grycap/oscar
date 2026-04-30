type Log = {
  status: "Succeeded" | "Failed" | "Running" | "Pending";
  creation_time: string;
  start_time: string;
  finish_time: string;
};

export default Log;
