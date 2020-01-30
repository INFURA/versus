# Versus

Versus takes a stream of requests and runs them against multiple endpoints
simultaneously, comparing the output and timing.

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

By default, HTTP endpoints will POST their requests. Versus is designed to be
used with a stream of JSONRPC requests. For example,
[ethspam](https://github.com/shazow/ethspam) can be used to generate realistic
Ethereum JSONRPC requests.

For example:

```
$ export INFURA_API_KEY="..."
$ ethspam | versus --stop-after=100 --concurrency=5 "https://mainnet.infura.io/v3/${INFURA_API_KEY}"
Endpoints:

0. "https://mainnet.infura.io/v3/..."

   Requests:   90.16 per second
   Timing:     0.0555s avg, 0.0405s min, 0.1866s max
               0.0296s standard deviation

   Percentiles:
     25% in 0.0441s
     50% in 0.0468s
     75% in 0.0515s
     90% in 0.0629s
     95% in 0.1685s
     99% in 0.1866s

   Errors: 0.00%

** Summary for 1 endpoints:
   Completed:  100 results with 100 total requests
   Timing:     55.457821ms request avg, 1.413079345s total run time
   Errors:     0 (0.00%)
   Mismatched: 0
```

Similarly, we can run versus against multiple endpoints and each response body will be compared to match.

```
$ ethspam | versus --stop-after=500 --concurrency=5 "https://mainnet.infura.io/v3/${INFURA_API_KEY}" "https://cloudflare-eth.com"
Endpoints:

0. "https://mainnet.infura.io/v3/..."

   Requests:   77.11 per second
   Timing:     0.0648s avg, 0.0378s min, 1.2764s max
               0.0759s standard deviation

   Percentiles:
     25% in 0.0444s
     50% in 0.0489s
     75% in 0.0595s
     90% in 0.0947s
     95% in 0.1492s
     99% in 0.2218s

   Errors: 0.00%

1. "https://cloudflare-eth.com"

   Requests:   64.22 per second
   Timing:     0.0779s avg, 0.0300s min, 8.5036s max
               0.4407s standard deviation

   Percentiles:
     25% in 0.0378s
     50% in 0.0411s
     75% in 0.0481s
     90% in 0.0747s
     95% in 0.1117s
     99% in 0.2655s

   Errors: 0.00%

** Summary for 2 endpoints:
   Completed:  500 results with 1000 total requests
   Timing:     71.347768ms request avg, 10.092800734s total run time
   Errors:     0 (0.00%)
   Mismatched: 1
```

Note that there was one response mismatched out of the 500 iterations. If we
run versus with verbose flags (`-v` or `-vv`), then mismatched bodies will be
printed.

### Caveats

Things to keep in mind while using versus and reading the reports:

- Mismatched results are not always bad, often it's just a matter of JSON
  key ordering or formatting or some extra attributes. Future versions of
  versus could do a better job about parsing and comparing JSON subsets.
- Your latency (ping) to the endpoint you're benchmarking is included in the
  timing. When comparing multiple endpoints, be mindful that the latency to
  each endpoint could vary.
- Pay attention to the standard deviation in timing, that's a good hint about
  the variance between the easiest and the hardest request during the
  benchmark, regardless of fixed latency.
- While HTTP connections are reused, the extra time to spin up a fresh
  connection at the beginning is also included. With more concurrency, make
  sure to use a higher iteration count so that the effect is not as pronounced.
  For example, 50 iterations at 50 concurrency, practically every iteration
  will end up creating a fresh socket and no connection reuse will occur.

There may be ways to improve the benchmark process to account for some of these
caveats, please open an issue with ideas for pull requests!

## Goals

Features:

- [x] Run against a single endpoint or many.
- [x] Run against local or remote endpoints.
- [x] Real-time parallel test execution.
- [ ] Compare results across separately-run tests
- [ ] Use live-streaming tcpdump data as test payloads

Compare between endpoints:

- [x] Response integrity: Body, status
- [x] Duration
- [x] Throughput


## License

MIT
