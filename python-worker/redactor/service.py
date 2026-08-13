"""gRPC service layer: translates between the wire format generated from
proto/redactor.proto and the framework-agnostic PresidioAnalyzer, so the
analyzer itself never depends on protobuf types.
"""

import logging

import grpc

from gen import redactor_pb2, redactor_pb2_grpc

from .analyzer import PresidioAnalyzer

logger = logging.getLogger(__name__)


class NLPService(redactor_pb2_grpc.NLPServiceServicer):
    def __init__(self, analyzer: PresidioAnalyzer):
        self._analyzer = analyzer

    def Analyze(self, request, context):
        try:
            entities = self._analyzer.analyze(request.text)
        except Exception:
            logger.exception("analysis failed for a %d-character request", len(request.text))
            context.abort(grpc.StatusCode.INTERNAL, "analysis failed")
            return redactor_pb2.AnalyzeResponse()

        return redactor_pb2.AnalyzeResponse(
            entities=[
                redactor_pb2.Entity(type=e.type, start=e.start, end=e.end, text=e.text, confidence=e.confidence)
                for e in entities
            ]
        )
