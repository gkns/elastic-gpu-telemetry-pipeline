# Constitution: Messaging Queue

## Constraints
* **Custom Implementation**: This must be a custom-built message queue and should not use any existing open-source message queue implementation. Also, the storage for this MQ should be in memory. This should be a separate service. It should expose GoLang APIs for the streamers and collectors to use to send and receive messages.
* **Prohibited Technologies**: Do not use existing message queues such as Kafka, RabbitMQ, or ZeroMQ.
* **Performance**: Design the system to handle scale, performance, and availability. It must support routing between up to 10 instances of streamers and collectors.
