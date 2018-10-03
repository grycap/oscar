FROM ubuntu

COPY fwatchdog /usr/bin/fwatchdog

RUN echo "Installing OSCAR dependencies." \
    && apt install -y python3 python3-pip \
    && pip3 install boto3

COPY supervisor.py supervisor.py

COPY script.sh /tmp/user_script.sh

RUN chmod +x /tmp/user_script.sh

# Set to true to see request in function logs
ENV write_debug="true"

HEALTHCHECK --interval=3s CMD [ -e /tmp/.lock ] || exit 1
CMD [ "fwatchdog" ]