import pytest

from redactor.analyzer import PresidioAnalyzer


@pytest.fixture(scope="module")
def analyzer():
    # Loading the spaCy model is the slow part (~1-2s); share one instance
    # across every test in this module instead of reloading it per test.
    return PresidioAnalyzer()


def find(entities, entity_type):
    return [e for e in entities if e.type == entity_type]


def test_detects_a_full_person_name(analyzer):
    entities = analyzer.analyze("Rashi Patil joined the call at 10am.")
    people = find(entities, "PERSON")
    assert len(people) == 1
    assert people[0].text == "Rashi Patil"
    assert people[0].start == 0 and people[0].end == 11


def test_rejects_single_token_person_matches(analyzer):
    # "Suite" alone is a known false-positive spaCy occasionally produces
    # for PERSON; the multi-token filter in PresidioAnalyzer should drop it
    # regardless of what the underlying NER model does.
    entities = analyzer.analyze("Please visit Suite 400 for check-in.")
    assert find(entities, "PERSON") == []


def test_detects_a_company_by_legal_suffix(analyzer):
    entities = analyzer.analyze("The contract was signed with Global Dynamics LLC yesterday.")
    orgs = find(entities, "ORG")
    assert any(o.text == "Global Dynamics LLC" for o in orgs)


def test_org_suffix_match_does_not_swallow_surrounding_words(analyzer):
    entities = analyzer.analyze("Rashi Patil works at Acme Corp on weekdays.")
    orgs = find(entities, "ORG")
    assert any(o.text == "Acme Corp" for o in orgs)
    assert not any("works at" in o.text for o in orgs)


def test_detects_a_street_address(analyzer):
    entities = analyzer.analyze("Please ship the package to 123 Main Street, Suite 400.")
    addresses = find(entities, "ADDRESS")
    assert any(a.text == "123 Main Street, Suite 400" for a in addresses)


def test_offsets_match_the_original_text(analyzer):
    text = "Contact Rashi Patil regarding the Acme Corp invoice."
    entities = analyzer.analyze(text)
    for e in entities:
        assert text[e.start : e.end] == e.text


def test_empty_text_returns_no_entities(analyzer):
    assert analyzer.analyze("") == []


def test_plain_text_with_no_pii_returns_no_entities(analyzer):
    entities = analyzer.analyze("The weather today is sunny with a light breeze.")
    assert entities == []
