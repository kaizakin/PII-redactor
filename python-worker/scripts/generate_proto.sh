#!/usr/bin/env bash
# Regenerates python-worker/gen/ from proto/redactor.proto. Run this from
# the repo root (or let it cd there itself) after editing the .proto file.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

python -m grpc_tools.protoc \
  -I proto \
  --python_out=python-worker/gen \
  --grpc_python_out=python-worker/gen \
  --pyi_out=python-worker/gen \
  proto/redactor.proto

# grpc_tools.protoc always emits a flat "import redactor_pb2", which only
# resolves if gen/ is on sys.path directly. Rewriting it as a relative
# import lets callers do `from gen import redactor_pb2_grpc` normally.
sed -i 's/^import redactor_pb2 as redactor__pb2$/from . import redactor_pb2 as redactor__pb2/' \
  python-worker/gen/redactor_pb2_grpc.py

touch python-worker/gen/__init__.py

echo "Regenerated python-worker/gen/ from proto/redactor.proto"
