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

# OCR'd image text is a separate surface from the main text path: Go's
# regex detectors (email, phone, SSN, credit card, IP — each with its own
# real validity check, see internal/detector) never see image bytes at
# all, since those never round-trip through Go's text pipeline. Presidio
# already loads built-in recognizers for these structured types as part of
# its default registry (see PresidioAnalyzer.__init__), so reusing them
# here for OCR text is free — no extra model loading, just a wider
# `entities` filter at analyze time. They're less rigorous than the Go
# detectors' own validation (no Luhn check, no SSA rules, no
# libphonenumber), which is an acceptable tradeoff given OCR output itself
# already carries misread-character noise.
OCR_ENTITIES = SUPPORTED_ENTITIES + ["EMAIL_ADDRESS", "PHONE_NUMBER", "CREDIT_CARD", "US_SSN", "IP_ADDRESS"]

# en_core_web_sm keeps startup time and Docker image size small. It does
# miss or truncate some non-Western names (see the comment on
# _is_full_name), but en_core_web_lg is not a safe swap: tested against a
# real prospectus, it caught two names the small model missed, but also
# dropped "Kushal Subbayya Hegde" (the company's own Chairman/Promoter)
# outright, in every context tested, and mis-merged "Rashi\nPatil" into
# "Patil\nEmail" on a labeled-field OCR layout the small model handles
# correctly. A false negative here is a real leak, not noise, so recall
# gaps need a fix that doesn't trade one miss for another — not a model
# swap. Revisit with en_core_web_trf or a name gazetteer instead.
DEFAULT_MODEL = "en_core_web_sm"

# spaCy's NER tags plenty of things we never asked about (numbers, dates,
# money amounts, ...). Presidio has no default entity mapping for these
# and logs a WARNING per occurrence, which is pure noise here since we
# only ever request PERSON/ORG/ADDRESS — silence it at the source instead
# of drowning real log output.
_IGNORED_NER_LABELS = [
    "CARDINAL", "ORDINAL", "MONEY", "QUANTITY", "PERCENT",
    "TIME", "DATE", "LANGUAGE", "WORK_OF_ART", "EVENT", "LAW",
    "FAC", "PRODUCT",
]


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
                "ner_model_configuration": {"labels_to_ignore": _IGNORED_NER_LABELS},
            }
        ).create_engine()

        registry = RecognizerRegistry()
        registry.load_predefined_recognizers(languages=["en"], nlp_engine=nlp_engine)
        registry.add_recognizer(SpacyOrgRecognizer())
        registry.add_recognizer(OrgSuffixRecognizer())
        registry.add_recognizer(AddressRecognizer())

        self._engine = AnalyzerEngine(registry=registry, nlp_engine=nlp_engine, supported_languages=["en"])

    def analyze(self, text: str) -> List[Entity]:
        """Analyzes free text for the main Go <-> Python text pipeline."""
        return self._analyze(text, SUPPORTED_ENTITIES)

    def analyze_ocr(self, text: str) -> List[Entity]:
        """Analyzes OCR'd image text, with the wider OCR_ENTITIES set."""
        return self._analyze(text, OCR_ENTITIES)

    def _analyze(self, text: str, entities: List[str]) -> List[Entity]:
        if not text:
            return []
        results = self._engine.analyze(text=text, language="en", entities=entities)
        found = [
            Entity(type=r.entity_type, start=r.start, end=r.end, text=text[r.start : r.end], confidence=r.score)
            for r in results
        ]
        return [e for e in found if e.type != "PERSON" or _is_full_name(e.text)]
