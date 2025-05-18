# Load Balancer And Rate Limiter App

## Description
Distributes loads between backends evenly using round robin algorithm, checking for backends availability. Uses api key in header Authorization for clients, limits their access to a certain limit using token bucket algorithm.

Forwards requests to backends, checks the answer, if it has failed, retries on another backend for ```max_retries``` tries.
Performs periodic health checks for all backends, marking them as healthy or not, forwards requests only to healthy backends.

Uses structured logging with key-values pairs with various levels. Log level and log file can set in config

Rate limiter stores clients state to PostgreSQL.

Efficiently handles high load by using in-memory cache with ```sync.Pool``` for clients.

Uses a lot of communication between goroutines using channels. Concurrency is used safely, making sure no data races, deadlocks or leaking goroutines occur. Lock times are reduced by using RW mutexes and atomics.

Has graceful shutdown, which stops all running goroutines by using context.

## Installation
```bash
docker compose up
```

## Usage

### Get client
GET /clients/{cliend_id}
#### Example
GET /clients/client_1
#### Returned value
```json
{
    "client_id": "client_1",
    "api_key": "api_key",
    "rate_per_sec": 1.4,
    "capacity": 20,
    "tokens": 0,
    "last_refill": "timestamp"
}
```
### Add client
POST /clients
#### Example
```json
{
    "client_id": "client_1",
    "rate_per_sec": 1.4,
    "capacity": 20
}
```
or just
```json
{
    "client_id": "client_1"
}
```
in this case ```default_rate_per_sec``` and ```default_capacity``` will be used
#### Returned value
status 201
```json
{
    "client_id": "client_1",
    "api_key": "api_key",
    "rate_per_sec": 1.4,
    "capacity": 20,
    "tokens": 0,
    "last_refill": "timestamp"
}
```
### Update client
PATCH /clients
#### Example
```json
{
    "client_id": "client_1",
    "rate_per_sec": 5,
    "capacity": 100
}
```
#### Returned value
```json
{
    "client_id": "client_1",
    "api_key": "api_key",
    "rate_per_sec": 5,
    "capacity": 100,
    "tokens": 0,
    "last_refill": "timestamp"
}
```
### Delete client
DELETE /clients/{client_id}
#### Returned value
status 204 no content

### Error Response
```json
{
    "code":    404,
    "message": "not found"
}
```

## Config
```json
{
    "port": 8080,
    "log_level": "debug",
    "log_file": "path_to_file",
    "load_balancer": {
        "backend_urls": ["http://host.docker.internal:8081", "http://host.docker.internal:8082"],
        "health_check_interval": 10,
        "health_check_worker_count": 2,
        "max_retries": 2,
        "request_timeout": 2
    },
    "rate_limiter": {
        "add_tokens_interval": 5,
        "save_state_interval": 10,
        "remove_stale_interval": 300,
        "default_rate_per_sec": 5,
        "default_capacity": 20,
        "cache": {
            "time_to_live": 300
        }
    }
}
```
All time is in seconds (can be decimal).

if ```log_file``` is empty string stdout is used

```max_retries``` number of retries for failed requests to backends

```time_to_live``` is the duration of a client remaining in cache. Every time client makes a request, expiration time will be set to time.Now + ```time_to_live```.

```add_tokens_interval``` is the interval between running a func for adding tokens to all clients in cache.

```save_state_interval``` is the interval between running a func for saving clients state to database.


```remove_stale_interval``` is the interval between running a func for removing clients from cache whose expiration date set by ```time_to_live``` has passed.

more info about these config options are in comments in code.
