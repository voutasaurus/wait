# Wait

A system to reliably process asynchronous tasks.

# Design

* Synchronous API server.
- Validates Request
- Stores Request Persistently (with timelock)
- Responds to client
- Asynchronously calls other API to process job

* Asynchronous API server
- Validates Request
- Responds to client
- Asynchronously processes the background job
- Updates the timelock in the persistent store (on completion)

* Pickup worker
- Queries the jobs for those which are not timelocked (expired timelock)
- Locks a job
- Sends that job to the Asynchronous API server
