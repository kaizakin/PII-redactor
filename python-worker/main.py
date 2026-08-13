"""Entry point for the PII redactor's Python NLP worker.

Serves NLPService (see proto/redactor.proto) over gRPC so the Go engine's
internal/grpcclient.GRPCClient can detect unstructured PII — person names,
company names, physical addresses — in text that regex-based structured
detectors can't reliably catch, and PII embedded in images (scanned IDs,
screenshots) via OCR.
"""

import logging
import os
from concurrent import futures

import grpc

from gen import redactor_pb2_grpc
from redactor.analyzer import DEFAULT_MODEL, PresidioAnalyzer
from redactor.image_redactor import ImageRedactor
from redactor.service import NLPService

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
logger = logging.getLogger("pii-nlp-worker")


def serve() -> None:
    port = os.environ.get("GRPC_PORT", "50051")
    model_name = os.environ.get("SPACY_MODEL", DEFAULT_MODEL)
    max_workers = int(os.environ.get("GRPC_MAX_WORKERS", "10"))

    logger.info("loading spaCy model %s", model_name)
    analyzer = PresidioAnalyzer(model_name=model_name)
    image_redactor = ImageRedactor(analyzer)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    redactor_pb2_grpc.add_NLPServiceServicer_to_server(NLPService(analyzer, image_redactor), server)
    server.add_insecure_port(f"[::]:{port}")

    server.start()
    logger.info("NLP worker listening on :%s", port)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
