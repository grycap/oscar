FROM bitnami/minideb

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        curl \
        git \
        python-setuptools \
        python-pip \
        build-essential \
        python-dev \
        python-wheel \
        python-numpy \
        python-scipy \
        python-tk

RUN pip install --upgrade https://github.com/Theano/Theano/archive/master.zip
RUN pip install --upgrade https://github.com/Lasagne/Lasagne/archive/master.zip

RUN cd /opt && \
    git clone https://github.com/indigo-dc/plant-classification-theano -b package && \
    cd plant-classification-theano && \
    pip install -e .

# Copy the classify_image script
COPY classify_image.py /opt/plant-classification-theano/classify_image.py

# Get pretrained model
ENV SWIFT_CONTAINER https://cephrgw01.ifca.es:8080/swift/v1/Plants/
ENV THEANO_TR_WEIGHTS resnet50_6182classes_100epochs.npz
ENV THEANO_TR_JSON resnet50_6182classes_100epochs.json
ENV SYNSETS synsets_binomial.txt
RUN curl -o /opt/plant-classification-theano/plant_classification/training_weights/${THEANO_TR_WEIGHTS} \
        ${SWIFT_CONTAINER}${THEANO_TR_WEIGHTS} && \
    curl -o /opt/plant-classification-theano/plant_classification/training_info/${THEANO_TR_JSON} \
        ${SWIFT_CONTAINER}${THEANO_TR_JSON} && \
    curl -o /opt/plant-classification-theano/data/data_splits/synsets_binomial.txt \
        ${SWIFT_CONTAINER}${SYNSETS}

RUN echo "Pulling watchdog binary from Github." \
    && curl -sSL https://github.com/openfaas/faas/releases/download/0.8.0/fwatchdog > /usr/bin/fwatchdog \
    && chmod +x /usr/bin/fwatchdog

RUN echo "Installing Oscar supervisors and dependencies." \
    && apt install -y python3 python3-pip \
    && pip3 install boto3 \
    && curl -sSL https://raw.githubusercontent.com/grycap/oscar/master/src/providers/onpremises/openfaas/function/supervisor.py > supervisor.py

COPY script.sh /tmp/user_script.sh
RUN chmod +x /tmp/user_script.sh

ENV fprocess="python3 supervisor.py"
ENV sprocess="/tmp/user_script.sh"

# Set to true to see request in function logs
ENV write_debug="true"

HEALTHCHECK --interval=5s CMD [ -e /tmp/.lock ] || exit 1
CMD [ "fwatchdog" ]