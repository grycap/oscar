FROM ubuntu:20.04
RUN apt-get update && apt-get -y install git pip python
RUN pip install TTS
RUN git clone https://github.com/coqui-ai/TTS /opt
RUN DEBIAN_FRONTEND=noninteractive TZ=Europe/UTC apt-get -y install tzdata
RUN apt-get install -y libsndfile1-dev 

