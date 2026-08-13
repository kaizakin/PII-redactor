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


def test_analyze_translates_entities_to_protobuf():
    entities = [Entity(type="PERSON", start=0, end=11, text="Rashi Patil", confidence=0.85)]
    service = NLPService(FakeAnalyzer(entities))

    response = service.Analyze(redactor_pb2.AnalyzeRequest(text="Rashi Patil works here"), MagicMock())

    assert len(response.entities) == 1
    assert response.entities[0].type == "PERSON"
    assert response.entities[0].start == 0
    assert response.entities[0].end == 11
    assert response.entities[0].text == "Rashi Patil"
    assert response.entities[0].confidence == pytest.approx(0.85)


def test_analyze_returns_empty_response_for_no_entities():
    service = NLPService(FakeAnalyzer([]))
    response = service.Analyze(redactor_pb2.AnalyzeRequest(text="nothing sensitive"), MagicMock())
    assert list(response.entities) == []


def test_analyze_aborts_on_analyzer_failure():
    service = NLPService(FailingAnalyzer())
    context = MagicMock()

    service.Analyze(redactor_pb2.AnalyzeRequest(text="anything"), context)

    context.abort.assert_called_once()
