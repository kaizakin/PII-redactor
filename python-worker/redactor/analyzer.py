"""Wraps Presidio's AnalyzerEngine with the custom recognizers this
service needs, exposing a small, framework-agnostic Entity type so the
gRPC service layer and tests don't need to depend on Presidio's own
RecognizerResult type.
"""

from dataclasses import dataclass
from typing import List

from presidio_analyzer import AnalyzerEngine, RecognizerRegistry
from presidio_analyzer.nlp_engine import NlpEngineProvider

from .recognizers import AddressRecognizer, OrgSuffixRecognizer, SpacyOrgRecognizer

# These three labels are exactly what internal/detector.NLPDetector on the
# Go side filters on (see cmd/main.go) — changing one here without
# changing the other breaks the contract silently, since gRPC has no
# compile-time link between the two.
SUPPORTED_ENTITIES = ["PERSON", "ORG", "ADDRESS"]

# en_core_web_sm keeps startup time and Docker image size small. Swapping
# in en_core_web_lg (better accuracy, ~400MB) or a transformer-based
# pipeline is a one-line change here, with no change anywhere else in the
# system — the "upgrade the model without touching the Go API" story this
# project is built around.
DEFAULT_MODEL = "en_core_web_sm"


@dataclass(frozen=True)
class Entity:
    type: str
    start: int
    end: int
    text: str
    confidence: float


def _is_full_name(text: str) -> bool:
    """Requires at least two tokens (e.g. "Rashi Patil", not "Suite").

    spaCy's PERSON NER — especially the small model this service defaults
    to — occasionally tags a single capitalized common word (a room
    label, a mid-sentence proper-sounding noun) as PERSON. Presidio
    assigns every such hit the same flat confidence score, so there's no
    score-threshold that filters bad ones without also dropping good
    ones. The assignment only asks to redact full names ("Rashi Patil",
    not "Rashi" alone), so requiring a multi-token match is both a real
    precision filter and a match to spec — not a workaround.
    """
    return len(text.split()) >= 2


class PresidioAnalyzer:
    """Detects unstructured PII (names, companies, addresses) in text."""

    def __init__(self, model_name: str = DEFAULT_MODEL):
        nlp_engine = NlpEngineProvider(
            nlp_configuration={
                "nlp_engine_name": "spacy",
                "models": [{"lang_code": "en", "model_name": model_name}],
            }
        ).create_engine()

        registry = RecognizerRegistry()
        registry.load_predefined_recognizers(languages=["en"], nlp_engine=nlp_engine)
        registry.add_recognizer(SpacyOrgRecognizer())
        registry.add_recognizer(OrgSuffixRecognizer())
        registry.add_recognizer(AddressRecognizer())

        self._engine = AnalyzerEngine(registry=registry, nlp_engine=nlp_engine, supported_languages=["en"])

    def analyze(self, text: str) -> List[Entity]:
        if not text:
            return []
        results = self._engine.analyze(text=text, language="en", entities=SUPPORTED_ENTITIES)
        entities = [
            Entity(type=r.entity_type, start=r.start, end=r.end, text=text[r.start : r.end], confidence=r.score)
            for r in results
        ]
        return [e for e in entities if e.type != "PERSON" or _is_full_name(e.text)]
