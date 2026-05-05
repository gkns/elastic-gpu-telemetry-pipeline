# Constitution: Telemetry Collector

## Behavior
* The collector is responsible for consuming telemetry from the custom message queue.
* It must parse the incoming messages and reliably persist them to the QuestDB.

## Infrastructure
* The implementation must support dynamic scaling (up and down) of Collector instances.