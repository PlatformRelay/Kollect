# Spec Delta

## ADDED Requirements

### Requirement: SCB-1 Terminal export errors do not count toward the sink breaker

The shared per-sink circuit breaker SHALL exclude an export plan result for which
`kollecterrors.ClassOf` is Terminal: it SHALL count neither as a failure nor as a success, and SHALL
NOT reset the consecutive-failure count. Classification is evaluated on the result at the breaker,
not after it.

#### Scenario: One inventory's terminal rejections do not open the breaker

- **GIVEN** a sink with a closed breaker
- **WHEN** one inventory's export to that sink fails with a Terminal error 10 times in a row
- **THEN** the breaker stays closed
- **AND** another inventory's next export to the same sink runs against the backend and succeeds

#### Scenario: An inferred Terminal status is excluded

- **GIVEN** a sink with a closed breaker
- **WHEN** the export plan returns an unwrapped Kubernetes NotFound status 10 times in a row
- **THEN** the breaker stays closed

#### Scenario: An error classified Terminal only after the breaker still counts

- **GIVEN** a sink with a closed breaker
- **WHEN** the export plan returns 5 times an unclassified error that the post-breaker Git classifier would mark Terminal (an authentication failure)
- **THEN** the sixth export does not call the backend

#### Scenario: An excluded error does not break a streak of retryable failures

- **GIVEN** a sink with a closed breaker
- **WHEN** exports fail Transient, Terminal, Transient, Transient, Transient, Transient
- **THEN** the breaker opens after the fifth Transient failure

### Requirement: SCB-2 Retryable export errors still open the sink breaker

The breaker SHALL count every other non-nil export result as a failure (Transient, Forbidden, and
errors without a class, including errors that become Terminal only after the breaker) and SHALL open
after 5 consecutive ones. While open, an export to the sink SHALL NOT reach the backend and SHALL fail with a
Transient error naming the open breaker.

#### Scenario: Five transient failures open the breaker

- **GIVEN** a sink with a closed breaker
- **WHEN** 5 consecutive exports fail with a Transient error
- **THEN** the sixth export does not call the backend
- **AND** it fails with a Transient error whose message contains "circuit breaker open"

#### Scenario: Unclassified and Forbidden errors count

- **GIVEN** a sink with a closed breaker
- **WHEN** 5 consecutive exports fail with an error that carries no class, or with a Forbidden error
- **THEN** the sixth export does not call the backend

### Requirement: SCB-3 A terminal result in half-open neither closes nor wedges the breaker

When the breaker is half-open, an export whose result is a Terminal error SHALL leave the breaker
half-open and SHALL release the trial slot, so that the next export is admitted as the trial. While a
trial is in flight, a concurrent export SHALL NOT reach the backend. A successful trial SHALL close the
breaker; a counted failure on the trial SHALL re-open it. Consequence (accepted): while Terminal
results keep winning the trial, the sink stays half-open and admits one export at a time.

#### Scenario: Terminal trial, then a successful trial

- **GIVEN** a breaker that opened on Transient failures and whose open timeout has elapsed
- **WHEN** the trial export fails with a Terminal error
- **THEN** the breaker is half-open
- **WHEN** the next export succeeds
- **THEN** that export reached the backend
- **AND** the breaker is closed afterwards

#### Scenario: Terminal trial, then a failing trial

- **GIVEN** a breaker that opened on Transient failures and whose open timeout has elapsed
- **WHEN** the trial export fails with a Terminal error
- **AND** the next export fails with a Transient error
- **THEN** the breaker is open afterwards

#### Scenario: Concurrent export during a trial

- **GIVEN** a half-open breaker whose trial export is in flight
- **WHEN** a second export starts before the trial finishes
- **THEN** the second export does not reach the backend
- **AND** after the trial finishes with a Terminal error, the next export is admitted
