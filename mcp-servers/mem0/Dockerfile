FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000

CMD ["python", "-m", "mem0_mcp_server", "--mode", "sse", "--host", "0.0.0.0", "--port", "8000"]
