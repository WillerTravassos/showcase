# NOTES to update spec documentation

Review my comments below, and ensure that they fit industry standards and verify that I am not breaking any security rules or Go/DDD conventions

I replaced {project} with github.com/WillerTravassos/showcase

## Hard Rules — Never Violate These

### Identity
- Every principal uses UUIDv5. Namespace constants live in `internal/auth/identity.go`. Never generate a UUID any other way for a user principal.
**Comment**: I need to think of this scenario, what happens if user changes email? Is there a way for me to link new UUIDv5 to the old ID. What other parameters
could provide me with a good UUIDv5 generation?

### Testing
- Every exported function must have at least one test.
**Comment**: actually, every test should have at least 2 tests, one for success scenarios and another for failure. In reality this should multiple scenarios for
both success and failure. Reason being, certain functions can fail, or succeed in multiple ways.
**Comment 2**: Avoid writing test for the sake of coverage, tests should test logic correctness of self-contained logic

- Integration tests are tagged `//go:build integration` and never run in CI unit test pass.
**Comment**: Integration tests will actually work 2 ways, they are either isolated using testcontainers-go or they are integration tests running on top of the tilt
managed cluster. Code should be instrumented with tracing and metrics. Integration tests uses the otel tracetest library to select spans and assert on its attributes.
Avoid tags for now, there should be a TILT_CI env var that can be used by tilt and a test util wrapper to determine which integration test requires TILT running.

### Database
- Never write raw SQL outside of `*_queries.sql` files (used by sqlc) or repository implementations.
**Comment**: This is good for relatively simple queries. For more complex querying (complex JOINs and WHERE clauses), then we should sqlc in conjunction with masterminds/squirrel.
Squirrel queries should never leave repository files.

### General Notes
- Vendor packages, I want to avoid being throttled in CI and want to guarantee packages to work, if they ever vanish
- Avoid string properties, where a customer type could easily scope values. For example, gender should not be a string but a type Gender string.


## specs/conventions/tilt

This needs full rework. Here's a description of how tilt will be used

- Makefile contains targets to instal and run tilt-up, tilt-down, and kind cluster management
- There should be a tilt directory which is subdivided in:
    - services: contains local only services that are managed by til
    - deploy: mimics customization directory for the repository. Overlays are only ci and local, or other variations
    - tilt.d: contains tiltfiles specific for each service we are wiring up. Ex: patient.tiltfile, api.tiltfile, etc
- The Tiltfile will dynamically load tiltfiles in ./tilt/tilt.d/
- tilt.d tiltfiles may contain a deploy_service(cfg) function which will receive tilt's cfg object from the tiltfile. This
is for dynamically creating services according to flag added in tilt up/ci make target
- Flags and their default values will be declared at the start of the Tiltfile
