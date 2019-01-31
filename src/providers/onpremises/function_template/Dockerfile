FROM ubuntu

COPY fwatchdog /usr/bin/fwatchdog
COPY supervisor /usr/bin/supervisor
COPY user_script.sh /tmp/user_script.sh

ENV SUPERVISOR_TYPE='OPENFAAS'

# Set to true to see request in function logs
ENV write_debug="true"

HEALTHCHECK --interval=3s CMD [ -e /tmp/.lock ] || exit 1
CMD [ "fwatchdog" ]