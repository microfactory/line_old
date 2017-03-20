# line
A framework that compiles to distributed AWS primitives

## Runtime failings:

### TODO, have gc process that:

- notices (states.Runtime) failures & aborted runs and deallocates their size:
{"error":"States.Runtime","cause":"Internal Error (456099e1-2efe-4864-b172-691fc458a046)"}
- empties unaligned activities
