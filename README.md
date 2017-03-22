# line
A framework that compiles to distributed AWS primitives

## Runtime failings:

### TODO, have gc process that:

Current Challenges:
- how to deallocate on user/admin aborts of the state machines
- how to retry and deallocate on system failures: "States.Runtime" and "Lambda.Unkown"
- unaligned activity tokens
- very inefficient in a stable situation (infinite retries)
