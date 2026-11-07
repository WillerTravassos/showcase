# Threat model

A lightweight STRIDE pass, run while drafting a ticket that touches a trust boundary. Its
output fills the ticket's "Security notes" section, and each mitigation becomes either a
scenario (an abuse case) or a rule the change must follow.

## When it is required

The ticket does any of the following:

- adds or changes an endpoint, consumer, CLI flag, or file the system reads;
- accepts input from outside the process (network, user, file, environment);
- touches authentication, authorization, sessions, or secrets;
- adds a dependency, a container image, or a new outbound call;
- changes Kubernetes manifests (permissions, network exposure, volumes).

## Steps

1. **Draw the data flow** in one or two lines: who sends what, through which component, to
   which store or service. Mark each point where data crosses a trust boundary (from a
   less trusted zone into a more trusted one).
2. **Walk STRIDE** for each crossing and fill the table. Skip a row only by writing why
   it does not apply.
3. **Turn mitigations into work**: an abuse-case scenario for each behaviour the system
   must reject, and a rule reference from `docs/standards/security.md` for each coding
   control.

| Threat (STRIDE)                                    | Question to answer                                              | Mitigation |
|----------------------------------------------------|-----------------------------------------------------------------|------------|
| **S**poofing (pretending to be someone else)       | How does the system know who is calling?                        |            |
| **T**ampering (changing data in transit or at rest)| Can the input or stored data be altered by the caller?          |            |
| **R**epudiation (denying an action happened)       | Is the action recorded with who, what, and when?                |            |
| **I**nformation disclosure (leaking data)          | What could leak through responses, logs, errors, or traces?     |            |
| **D**enial of service (exhausting resources)       | What bounds size, rate, time, and concurrency?                  |            |
| **E**levation of privilege (gaining extra rights)  | Can the caller reach data or actions beyond their role?         |            |

## Abuse-case scenarios

Write them like any other scenario, from the attacker's side:

```
Given a patient signed in as patient A
When they request the appointment history of patient B
Then the request is refused as not found
And no data about patient B is returned
```

Prefer "not found" over "forbidden" for resources the caller should not know exist.
