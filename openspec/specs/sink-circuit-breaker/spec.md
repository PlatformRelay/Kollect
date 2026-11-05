# sink-circuit-breaker Specification

## Purpose
Which export failures count toward a sink's shared circuit breaker, and how the breaker behaves
closed, open and half-open. The breaker sheds load from a sink that is failing for every inventory;
a failure that belongs to one inventory (an ownership conflict) must not open it for the others.

## Requirements

### Requirement: SCB-1 Inventory-scoped export failures do not count toward the sink breaker

The shared per-sink circuit breaker SHALL exclude an export plan result that carries the
inventory-scoped marker (`errors.Is(err, kollecterrors.ErrInventoryScoped)`): it SHALL count neither
as a failure nor as a success, and SHALL NOT reset the consecutive-failure count. The marker SHALL
make the error Terminal and keep its text.

#### Scenario: One inventory's ownership rejections do not open the breaker

- **GIVEN** a Git sink with a closed breaker
- **WHEN** one inventory's export to that sink is rejected for ownership 10 times in a row
- **THEN** the breaker stays closed
- **AND** another inventory's next export to the same sink runs against the backend and succeeds

#### Scenario: The marker survives to the breaker

- **GIVEN** a Git sink whose export plan rejects an inventory for ownership
- **WHEN** the export returns
- **THEN** the error carries the inventory-scoped marker and is Terminal

#### Scenario: An excluded result does not break a streak of counted failures

- **GIVEN** a sink with a closed breaker
- **WHEN** exports fail Transient, inventory-scoped, Transient, Transient, Transient, Transient
- **THEN** the breaker opens after the fifth Transient failure

### Requirement: SCB-2 Every other export failure still counts toward the sink breaker

The breaker SHALL count every non-nil export plan result without the marker as a failure (Transient,
Forbidden, Terminal, and errors without a class) and SHALL open after 5 consecutive ones within its
closed-state counting interval. While open, an export to the sink SHALL NOT reach the backend and SHALL
fail with a Transient error naming the open breaker.

#### Scenario: Five transient failures open the breaker

- **GIVEN** a sink with a closed breaker
- **WHEN** 5 consecutive exports fail with a Transient error
- **THEN** the sixth export does not call the backend
- **AND** it fails with a Transient error whose message contains "circuit breaker open"

#### Scenario: Sink-wide Terminal failures open the breaker

- **GIVEN** a Git sink with a closed breaker
- **WHEN** 5 consecutive exports fail with an authentication error the Git backend classifies Terminal
- **THEN** the sixth export does not call the backend

#### Scenario: Unclassified and Forbidden errors count

- **GIVEN** a sink with a closed breaker
- **WHEN** 5 consecutive exports fail with an error that carries no class, or with a Forbidden error
- **THEN** the sixth export does not call the backend

### Requirement: SCB-3 An inventory-scoped result in half-open neither closes nor wedges the breaker

When the breaker is half-open, an export whose result is inventory-scoped SHALL leave it half-open
and SHALL release the trial slot, so the next export is admitted as the trial. While a trial is in
flight, a concurrent export SHALL NOT reach the backend. A successful trial SHALL close the breaker; a
counted failure on the trial SHALL re-open it.

#### Scenario: Inventory-scoped trial, then a successful trial

- **GIVEN** a breaker that opened on Transient failures and whose open timeout has elapsed
- **WHEN** the trial export fails inventory-scoped
- **THEN** the breaker is half-open
- **WHEN** the next export succeeds
- **THEN** that export reached the backend
- **AND** the breaker is closed afterwards

#### Scenario: Inventory-scoped trial, then a failing trial

- **GIVEN** a breaker that opened on Transient failures and whose open timeout has elapsed
- **WHEN** the trial export fails inventory-scoped
- **AND** the next export fails with a Transient error
- **THEN** the breaker is open afterwards

#### Scenario: Concurrent export during a trial

- **GIVEN** a half-open breaker whose trial export is in flight
- **WHEN** a second export starts before the trial finishes
- **THEN** the second export does not reach the backend
- **AND** after the trial finishes inventory-scoped, the next export is admitted
