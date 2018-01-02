FROM python:3.6.4-jessie
LABEL author=troy.dai@outlook.com

COPY app /app

RUN rm /usr/bin/python && ln /usr/local/bin/python3.6 /usr/bin/python && \
    pip install -r /app/requirements.txt


