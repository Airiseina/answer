FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir --timeout 300 -r requirements.txt

RUN mkdir -p knowledge
COPY __init__.py __main__.py knowledge/

EXPOSE 8001

CMD ["python", "-m", "knowledge", "--mode", "sse", "--host", "0.0.0.0", "--port", "8001"]
