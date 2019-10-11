FROM ubuntu:18.04
MAINTAINER Sebas Risco <serisgal@i3m.upv.es>

RUN apt-get update && \
    apt-get install -y build-essential && \
    apt-get install -y gcc automake autoconf libtool && \
    apt-get install -y swig python3-dev python3-numpy python3-scipy && \
    apt-get install -y sox bison git

RUN mkdir /sphinx && \
    cd /sphinx && \
    git clone https://github.com/cmusphinx/sphinxbase.git && \
    git clone https://github.com/cmusphinx/pocketsphinx.git

WORKDIR /sphinx

# Install sphinxbase
RUN cd sphinxbase && \
    ./autogen.sh && \
    ./configure && \
    make clean all && \
    make check && \
    make install

# Install PocketSphinx
RUN cd pocketsphinx && \
    ./autogen.sh && \
    ./configure && \
    make check && \
    make install

ENV LD_LIBRARY_PATH=/usr/local/lib
ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig

# Get audio2srt
RUN cd / && \
    git clone https://github.com/RunasSudo/audio2srt.git
WORKDIR /audio2srt

# Copy US model
RUN cp -r /sphinx/pocketsphinx/model/en-us/. .

# Clear
RUN rm -rf /var/lib/{apt,dpkg,cache,log} && \
    rm -rf /sphinx && \
    apt-get remove -y --auto-remove build-essential gcc automake autoconf libtool git && \
    apt-get clean



