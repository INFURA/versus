# Versus

Versus takes a stream of requests and runs them against multiple endpoints
simultaneously, comparing the output and timing.

**Status**: It works, but not ready for official release.

## Usage

```
Usage:
  versus [OPTIONS] [endpoint...]

Application Options:
      --timeout=     Abort request after duration (default: 30s)
      --stop-after=  Stop after N requests per endpoint, N can be a number or duration.
      --concurrency= Concurrent requests per endpoint (default: 1)
  -v, --verbose      Show verbose logging.
      --version      Print version and exit.

Help Options:
  -h, --help         Show this help message

Arguments:
  endpoint:          API endpoint to load test, such as "http://localhost:8080/"
```

## Goals

Features:

- Run against a single endpoint or many.
- Run against local or remote endpoints.
- Real-time parallel test execution.
- Compare results across separately-run tests
- Use live-streaming tcpdump data as test payloads

Compare between endpoints:

- Response integrity: Body, status
- Latency
- Throughput


## License

MIT
