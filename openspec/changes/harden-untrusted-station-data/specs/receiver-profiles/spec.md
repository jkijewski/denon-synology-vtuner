# Spec Delta

## Purpose

Makes the receiver's address harder to fake when the request comes through the reverse proxy.

## ADDED Requirements

### Requirement: Receiver address behind the reverse proxy
For a request from the local reverse proxy, the system SHALL take the client address from the last entry of `X-Forwarded-For`, which the proxy adds, and SHALL use `X-Real-IP` only when there is no `X-Forwarded-For`. Requests from any other address SHALL use the connection's own address.

#### Scenario: Made-up forwarded address
- **GIVEN** a client at `192.168.1.50` sends `X-Forwarded-For: 192.168.1.99`, and the proxy appends its address
- **WHEN** the request reaches vTuner Radio as `X-Forwarded-For: 192.168.1.99, 192.168.1.50`
- **THEN** the system treats the client as `192.168.1.50`
