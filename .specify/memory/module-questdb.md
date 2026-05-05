# Constitution: QuestDB instance

## Behavior
* This single-node QuestDB instance enables the persistence of processed messages from collector.
* It persists the data for optimised time-series querying by the API gateway.

## Infrastructure
* The implementation remains single node. But multiple components like the collector and the API Gateway can connect to this simultaneously.