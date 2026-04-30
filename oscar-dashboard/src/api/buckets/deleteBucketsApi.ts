import axios from "axios";

async function deleteBucketsApi(bucket: string) {
  const response  = await axios.delete("/system/bucket/"+bucket)
  
  return response;
}

export default deleteBucketsApi;
