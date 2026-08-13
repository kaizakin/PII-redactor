from unittest.mock import MagicMock

import pytest

from gen import redactor_pb2
from redactor.analyzer import Entity
from redactor.service import NLPService


class FakeAnalyzer:
    def __init__(self, entities):
        self._entities = entities

    def analyze(self, text):
        return self._entities


class FailingAnalyzer:
    def analyze(self, text):
        raise RuntimeError("boom")


class FakeImageRedactor:
    def __init__(self, redacted_data=b"", count=0):
        self._redacted_data = redacted_data
        self._count = count

    def redact(self, data, fmt):
        return self._redacted_data, self._count


class FailingImageRedactor:
    def redact(self, data, fmt):
        raise RuntimeError("boom")


def test_analyze_translates_entities_to_protobuf():
    entities = [Entity(type="PERSON", start=0, end=11, text="Rashi Patil", confidence=0.85)]
    service = NLPService(FakeAnalyzer(entities), FakeImageRedactor())

    response = service.Analyze(redactor_pb2.AnalyzeRequest(text="Rashi Patil works here"), MagicMock())

    assert len(response.entities) == 1
    assert response.entities[0].type == "PERSON"
    assert response.entities[0].start == 0
    assert response.entities[0].end == 11
    assert response.entities[0].text == "Rashi Patil"
    assert response.entities[0].confidence == pytest.approx(0.85)


def test_analyze_returns_empty_response_for_no_entities():
    service = NLPService(FakeAnalyzer([]), FakeImageRedactor())
    response = service.Analyze(redactor_pb2.AnalyzeRequest(text="nothing sensitive"), MagicMock())
    assert list(response.entities) == []


def test_analyze_aborts_on_analyzer_failure():
    service = NLPService(FailingAnalyzer(), FakeImageRedactor())
    context = MagicMock()

    service.Analyze(redactor_pb2.AnalyzeRequest(text="anything"), context)

    context.abort.assert_called_once()


def test_redact_image_translates_result_to_protobuf():
    service = NLPService(FakeAnalyzer([]), FakeImageRedactor(redacted_data=b"redacted-bytes", count=2))

    response = service.RedactImage(
        redactor_pb2.RedactImageRequest(image_data=b"original-bytes", format="png"), MagicMock()
    )

    assert response.image_data == b"redacted-bytes"
    assert response.redactions == 2


def test_redact_image_returns_zero_redactions_response():
    service = NLPService(FakeAnalyzer([]), FakeImageRedactor(redacted_data=b"unchanged", count=0))

    response = service.RedactImage(
        redactor_pb2.RedactImageRequest(image_data=b"unchanged", format="jpeg"), MagicMock()
    )

    assert response.image_data == b"unchanged"
    assert response.redactions == 0


def test_redact_image_aborts_on_redactor_failure():
    service = NLPService(FakeAnalyzer([]), FailingImageRedactor())
    context = MagicMock()

    service.RedactImage(redactor_pb2.RedactImageRequest(image_data=b"x", format="png"), context)

    context.abort.assert_called_once()
