"""gRPC service layer: translates between the wire format generated from
proto/redactor.proto and the framework-agnostic PresidioAnalyzer, so the
analyzer itself never depends on protobuf types.
"""

import logging
import time

import grpc

from gen import redactor_pb2, redactor_pb2_grpc

from .analyzer import PresidioAnalyzer
from .image_redactor import ImageRedactor

logger = logging.getLogger(__name__)


class NLPService(redactor_pb2_grpc.NLPServiceServicer):
    def __init__(self, analyzer: PresidioAnalyzer, image_redactor: ImageRedactor):
        self._analyzer = analyzer
        self._image_redactor = image_redactor

    def Analyze(self, request, context):
        start = time.monotonic()
        logger.info("received analyze request: %d chars", len(request.text))

        try:
            entities = self._analyzer.analyze(request.text)
        except Exception:
            logger.exception(
                "analysis failed after %.1fms for a %d-character request",
                (time.monotonic() - start) * 1000,
                len(request.text),
            )
            context.abort(grpc.StatusCode.INTERNAL, "analysis failed")
            return redactor_pb2.AnalyzeResponse()

        logger.info(
            "found %d entities (%s) in %.1fms",
            len(entities),
            ", ".join(e.type for e in entities) or "none",
            (time.monotonic() - start) * 1000,
        )

        return redactor_pb2.AnalyzeResponse(
            entities=[
                redactor_pb2.Entity(type=e.type, start=e.start, end=e.end, text=e.text, confidence=e.confidence)
                for e in entities
            ]
        )

    def RedactImage(self, request, context):
        start = time.monotonic()
        logger.info("received redact_image request: %d bytes, format=%s", len(request.image_data), request.format)

        try:
            redacted, count = self._image_redactor.redact(request.image_data, request.format)
        except Exception:
            logger.exception(
                "image redaction failed after %.1fms for a %d-byte %s image",
                (time.monotonic() - start) * 1000,
                len(request.image_data),
                request.format,
            )
            context.abort(grpc.StatusCode.INTERNAL, "image redaction failed")
            return redactor_pb2.RedactImageResponse()

        logger.info(
            "redacted %d regions in %.1fms",
            count,
            (time.monotonic() - start) * 1000,
        )
        return redactor_pb2.RedactImageResponse(image_data=redacted, redactions=count)
