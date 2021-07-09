FROM compss/agent:2.8.rc2103

COPY wordcount.py /app/wordcount.py
COPY wordcount.sh /app/wordcount.sh

RUN chmod +x /app/wordcount.sh

ENV PYTHONPATH=/app

ENTRYPOINT  ["/app/wordcount.sh"]
