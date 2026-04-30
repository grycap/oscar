import axios from "axios";

async function createBucketsApi(bucket: string,users: Array<string> | undefined) {
  const response  = users == null ?await axios.post("/system/bucket/"+bucket, users) : await axios.post("/system/bucket/"+bucket)
  return response;
}

export default createBucketsApi;
