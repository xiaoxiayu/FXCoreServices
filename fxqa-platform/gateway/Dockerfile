FROM 10.103.129.81:5000/alpine:latest
MAINTAINER xiaoxia_yu <xiaoxia_yu@foxitsoftware.com>

RUN mkdir /foxitqa
ADD ./fxqa-gateway.conf /foxitqa/fxqa-gateway.conf
ADD ./fxqa-gateway /foxitqa/fxqa-gateway
ADD ./swagger-index-template.html /foxitqa/swagger-index-template.html
COPY ./swagger/ /foxitqa/swagger/

WORKDIR /foxitqa
CMD [ "/bin/sh", "-c", "./fxqa-gateway"] 
