#!/bin/bash

REST_AGENT_PORT=46101

/opt/COMPSs/Runtime/scripts/user/compss_agent_start --hostname="${AGENT_HOST}" --comm_port="${COMM_AGENT_PORT}" --rest_port="${REST_AGENT_PORT}" -d ${DEBUG} --classpath=${APP_PATH} --pythonpath=${APP_PATH} --log_dir=/tmp >/tmp/out 2>/tmp/err &
container_pid=$!

retries="10"
curl -XGET http://127.0.0.1:${REST_AGENT_PORT}/COMPSs/test 1>/dev/null 2>/dev/null
while [ ! "$?" == "0" ] && [ "${retries}" -gt "0" ]; do
    sleep 1
    retries=$((retries - 1 ))
    curl -XGET http://127.0.0.1:${REST_AGENT_PORT}/COMPSs/test 1>/dev/null 2>/dev/null
done

sleep 1

/opt/COMPSs/Runtime/scripts/user/compss_agent_call_operation --stop --master_node=127.0.0.1 --master_port=${REST_AGENT_PORT} --lang="PYTHON" --method_name="main" wordcount $* 

wait ${container_pid}
