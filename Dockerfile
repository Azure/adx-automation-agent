FROM python:3.6.4-jessie

COPY material /tmp/material 

RUN find /tmp/material/build -name '*.whl' | xargs pip install && \
    find /tmp/material/build -name '*nspkg*.whl' | xargs pip install && \
    az

RUN mkdir -p /app/static && \
    python /tmp/material/scripts/collect_tests.py > /app/static/manifest.json

