# line

### Data
Data is a first class citizen, Line's simple scheduling borrows from Hadoop's philosophy of moving compute to the data instead of the other way around.

- Dataset: a git repository
- Version: a git commit
- Replica: a clone of a _Dataset_ on different specific node (no 2 replicas of the dataset are on the same node)
- Checkout: a local (mutable) working copy of specific _Dataset_ _Version_

replicas eventually become in the same state, tasks require a replica at specific version

- Worker: a process that is responsible for:
  - reporting replica status
  - reporting alloc status

### Tasks
The main purpose of Line is to run a container that take a specific Dataset Checkout as input and produce one or more versions of the dataset as output.

- Task: a planned container that takes N inputs and M outputs
- Input/Output: a local checkout of a certain dataset version, inputs are read-only. Outputs are writeable and committed when the task is finished.

A task can only be run if a dataset replica is present on the worker and checkouts for input and outputs can be placed on the node.


### Hadoop like scheduling (compute to data)

- HDFS Arch guide: https://hadoop.apache.org/docs/r1.2.1/hdfs_design.html
-

### Problems with state machine approach:

Current Challenges:
- how to deallocate on user/admin aborts of the state machines
- how to retry and deallocate on system failures: "States.Runtime" and "Lambda.Unkown"
- unaligned activity tokens
- very inefficient in a stable situation (infinite retries)
