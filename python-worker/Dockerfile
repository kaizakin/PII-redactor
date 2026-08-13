FROM python:3.12-slim

WORKDIR /app

# grpcio, spacy, and the en_core_web_sm model all ship prebuilt manylinux
# wheels for this platform, so no compiler toolchain is needed here.
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY gen ./gen
COPY redactor ./redactor
COPY main.py .

RUN useradd --create-home --uid 1000 worker
USER worker

EXPOSE 50051
CMD ["python", "main.py"]
