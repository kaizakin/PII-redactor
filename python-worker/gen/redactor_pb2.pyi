from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AnalyzeRequest(_message.Message):
    __slots__ = ("text",)
    TEXT_FIELD_NUMBER: _ClassVar[int]
    text: str
    def __init__(self, text: _Optional[str] = ...) -> None: ...

class AnalyzeResponse(_message.Message):
    __slots__ = ("entities",)
    ENTITIES_FIELD_NUMBER: _ClassVar[int]
    entities: _containers.RepeatedCompositeFieldContainer[Entity]
    def __init__(self, entities: _Optional[_Iterable[_Union[Entity, _Mapping]]] = ...) -> None: ...

class Entity(_message.Message):
    __slots__ = ("type", "start", "end", "text", "confidence")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    type: str
    start: int
    end: int
    text: str
    confidence: float
    def __init__(self, type: _Optional[str] = ..., start: _Optional[int] = ..., end: _Optional[int] = ..., text: _Optional[str] = ..., confidence: _Optional[float] = ...) -> None: ...

class RedactImageRequest(_message.Message):
    __slots__ = ("image_data", "format")
    IMAGE_DATA_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    image_data: bytes
    format: str
    def __init__(self, image_data: _Optional[bytes] = ..., format: _Optional[str] = ...) -> None: ...

class RedactImageResponse(_message.Message):
    __slots__ = ("image_data", "redactions")
    IMAGE_DATA_FIELD_NUMBER: _ClassVar[int]
    REDACTIONS_FIELD_NUMBER: _ClassVar[int]
    image_data: bytes
    redactions: int
    def __init__(self, image_data: _Optional[bytes] = ..., redactions: _Optional[int] = ...) -> None: ...
