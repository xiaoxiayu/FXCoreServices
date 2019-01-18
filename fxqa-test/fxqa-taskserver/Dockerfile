FROM 10.103.129.81:5000/alpine:latest
MAINTAINER xiaoxia_yu <xiaoxia_yu@foxitsoftware.com>

RUN mkdir /foxitqa
ADD ./fxqa-testmanager.conf /foxitqa/fxqa-testmanager.conf
ADD ./fxqa-testmanager /foxitqa/fxqa-testmanager

WORKDIR /foxitqa
CMD [ "/bin/sh", "-c", "./fxqa-testmanager"] 
