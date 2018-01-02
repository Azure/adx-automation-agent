FROM alpine:3.7
LABEL author=troy.dai@outlook.com

# Add Python3 and Azure CLI requirements
RUN apk update && apk upgrade && \
    apk add --virtual=build && \
    apk add bash gcc make libffi-dev musl-dev openssl-dev python3 python3-dev && \
    ln /usr/bin/python3 /usr/bin/python && ln /usr/bin/pip3 /usr/bin/pip

COPY app /app

RUN pip install -r /app/requirements.txt

