FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir --timeout 300 -r requirements.txt

COPY . .

EXPOSE 8001

CMD ["python", "-m", "knowledge_mcp_server", "--mode", "sse", "--host", "0.0.0.0", "--port", "8001"]
