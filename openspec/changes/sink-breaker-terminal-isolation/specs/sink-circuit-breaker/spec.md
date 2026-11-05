# Spec Delta

## ADDED Requirements

### Requirement: SCB-1 Terminal export errors do not count toward the sink breaker

The shared per-sink circuit breaker SHALL exclude an export error classified Terminal: it SHALL count
neither as a failure nor as a success, and SHALL NOT reset the consecutive-failure count.

#### Scenario: One inventory's terminal rejections do not open the breaker

- **GIVEN** a sink with a closed breaker
- **WHEN** one inventory's export to that sink fails with a Terminal error 10 times in a row
- **THEN** the breaker stays closed
- **AND** another inventory's next export to the same sink runs against the backend and succeeds

#### Scenario: An excluded error does not break a streak of retryable failures

- **GIVEN** a sink with a closed breaker
- **WHEN** exports fail Transient, Terminal, Transient, Transient, Transient, Transient
- **THEN** the breaker opens after the fifth Transient failure

### Requirement: SCB-2 Retryable export errors still open the sink breaker

The breaker SHALL count Transient and unclassified export errors as failures and SHALL open after 5
consecutive ones. While open, an export to the sink SHALL NOT reach the backend and SHALL fail with a
Transient error naming the open breaker.

#### Scenario: Five transient failures open the breaker

- **GIVEN** a sink with a closed breaker
- **WHEN** 5 consecutive exports fail with a Transient error
- **THEN** the sixth export does not call the backend
- **AND** it fails with a Transient error whose message contains "circuit breaker open"

#### Scenario: Unclassified errors count

- **GIVEN** a sink with a closed breaker
- **WHEN** 5 consecutive exports fail with an error that carries no class
- **THEN** the sixth export does not call the backend

### Requirement: SCB-3 A terminal result in half-open neither closes nor wedges the breaker

When the breaker is half-open, an export whose result is a Terminal error SHALL leave the breaker
half-open and SHALL release the trial slot, so that the next export is admitted as the trial.

#### Scenario: Terminal trial, then a successful trial

- **GIVEN** a breaker that opened on Transient failures and whose open timeout has elapsed
- **WHEN** the trial export fails with a Terminal error
- **AND** the next export succeeds
- **THEN** the next export reaches the backend
- **AND** the breaker is closed afterwards

#### Scenario: Terminal trial, then a failing trial

- **GIVEN** a breaker that opened on Transient failures and whose open timeout has elapsed
- **WHEN** the trial export fails with a Terminal error
- **AND** the next export fails with a Transient error
- **THEN** the breaker is open afterwards
