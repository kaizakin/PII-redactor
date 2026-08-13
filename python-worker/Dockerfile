FROM python:3.12-slim

WORKDIR /app

# tesseract-ocr is the OCR engine ImageRedactor shells out to via
# pytesseract; on Debian it pulls in English language data
# (tesseract-ocr-eng) as a dependency, which is all this service needs.
RUN apt-get update \
    && apt-get install -y --no-install-recommends tesseract-ocr \
    && rm -rf /var/lib/apt/lists/*

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
