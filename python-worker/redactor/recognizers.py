"""Custom Presidio recognizers for the entity types Presidio's built-in
recognizer set doesn't cover out of the box.

Presidio's default recognizers already give us PERSON (via its
SpacyRecognizer, backed by the shared spaCy NER model), so that one needs
no custom code. Company names and physical addresses are the two gaps this
module fills:

- Presidio's SpacyRecognizer deliberately excludes ORG from its supported
  entities (spaCy's ORG tag alone is considered too noisy), so
  SpacyOrgRecognizer re-surfaces it, and OrgSuffixRecognizer backs it up
  with a legal-suffix pattern for the cases NER misses.
- spaCy has no ADDRESS entity at all, so AddressRecognizer is pure
  rule-based heuristics: a house number, a street name, and a
  recognized street-type word.
"""

from typing import List, Optional

import regex
from presidio_analyzer import EntityRecognizer, Pattern, PatternRecognizer, RecognizerResult
from presidio_analyzer.nlp_engine import NlpArtifacts

# PatternRecognizer matches case-insensitively by default (its
# global_regex_flags default includes regex.I), which would let
# "[A-Z]" match lowercase letters too. Both recognizers below depend on
# real capitalization to tell a proper noun from an ordinary word, so
# they explicitly drop the I flag.
_CASE_SENSITIVE_FLAGS = regex.M | regex.S


class SpacyOrgRecognizer(EntityRecognizer):
    """Surfaces spaCy's own ORG named-entity spans as ORG results.

    The shared NlpEngine already runs full NER as part of every Presidio
    request, so this recognizer costs nothing extra — it just reads spans
    the model already computed instead of matching text itself.
    """

    def __init__(self):
        super().__init__(supported_entities=["ORG"], name="SpacyOrgRecognizer", supported_language="en")

    def load(self) -> None:
        # No model of our own to load; we ride on the NlpEngine's spaCy
        # pipeline, which AnalyzerEngine loads once and shares.
        pass

    def analyze(
        self, text: str, entities: List[str], nlp_artifacts: Optional[NlpArtifacts]
    ) -> List[RecognizerResult]:
        if not nlp_artifacts or "ORG" not in entities:
            return []
        return [
            RecognizerResult(entity_type="ORG", start=ent.start_char, end=ent.end_char, score=0.6)
            for ent in nlp_artifacts.entities
            if ent.label_ == "ORG"
        ]


# Legal-entity suffixes that make a preceding capitalized phrase very
# likely to be a company name, independent of whether spaCy's NER tagged
# it as ORG at all.
_ORG_SUFFIXES = r"Inc|Incorporated|LLC|L\.L\.C\.|Corp|Corporation|Ltd|Limited|Co|Company|GmbH|PLC|LLP|LP|Group|Holdings"

_ORG_SUFFIX_PATTERN = Pattern(
    name="org_suffix",
    regex=rf"\b(?:[A-Z][\w&'-]*\s+){{0,4}}[A-Z][\w&'-]*,?\s+(?:{_ORG_SUFFIXES})\.?\b",
    score=0.85,
)


class OrgSuffixRecognizer(PatternRecognizer):
    """Catches company names by their legal suffix (Inc, LLC, Corp, ...).

    High precision: a random capitalized phrase in ordinary prose almost
    never happens to end in "LLC" or "Corp", so a match here is a strong
    signal even without NER agreeing.
    """

    def __init__(self):
        super().__init__(
            supported_entity="ORG",
            patterns=[_ORG_SUFFIX_PATTERN],
            name="OrgSuffixRecognizer",
            global_regex_flags=_CASE_SENSITIVE_FLAGS,
        )


# US street-address heuristic: a house number, one to four capitalized
# words, and a recognized street-type token, optionally followed by a
# suite/apartment/unit designator.
_STREET_TYPES = (
    r"Street|St|Avenue|Ave|Boulevard|Blvd|Road|Rd|Lane|Ln|Drive|Dr|Court|Ct|"
    r"Way|Circle|Cir|Place|Pl|Highway|Hwy|Parkway|Pkwy|Terrace|Ter|Square|Sq"
)

_ADDRESS_PATTERN = Pattern(
    name="street_address",
    regex=(
        rf"\b\d{{1,6}}\s+(?:[A-Z][a-zA-Z.]*\s+){{1,4}}(?:{_STREET_TYPES})\.?"
        rf"(?:,?\s+(?:Suite|Ste|Apt|Unit)\.?\s+\w+)?\b"
    ),
    score=0.75,
)


class AddressRecognizer(PatternRecognizer):
    """Finds physical street addresses via number + street-name +
    street-type heuristics — spaCy has no native address entity to lean
    on, so this one is regex-only.
    """

    def __init__(self):
        super().__init__(
            supported_entity="ADDRESS",
            patterns=[_ADDRESS_PATTERN],
            name="AddressRecognizer",
            global_regex_flags=_CASE_SENSITIVE_FLAGS,
        )
