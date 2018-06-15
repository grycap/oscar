FROM grycap/imagemagick

RUN apt update && apt install -y curl \
    && echo "Pulling watchdog binary from Github." \
    && curl -sSL https://github.com/openfaas/faas/releases/download/0.8.0/fwatchdog > /usr/bin/fwatchdog \
    && chmod +x /usr/bin/fwatchdog

RUN echo "Installing Oscar supervisors and dependencies." \
    && apt install -y python3 python3-pip \
    && pip3 install boto3 \
    && curl -sSL https://raw.githubusercontent.com/grycap/oscar/master/src/providers/onpremises/openfaas/function/scarsupervisor.py > supervisor.py

COPY script.sh /tmp/user_script.sh
RUN chmod +x /tmp/user_script.sh

ENV fprocess="python3 supervisor.py"
ENV sprocess="/tmp/user_script.sh"

# Set to true to see request in function logs
ENV write_debug="true"

HEALTHCHECK --interval=5s CMD [ -e /tmp/.lock ] || exit 1
CMD [ "fwatchdog" ]